# FoundationDB → PostgreSQL Migration Plan

## 1. Executive Summary

Update 2025-10-19: Data migration to PostgreSQL is finished. The application now runs on PostgreSQL exclusively, and FoundationDB is no longer required at runtime. The main branch no longer ships any FDB-specific code; this document is retained for historical context and audit trails.
This document describes the completed migration path from FoundationDB (FDB) to PostgreSQL (PG) and highlights the cleanup that removed FDB code paths, configs, and deployment mounts from the project.

Approach highlights:
- Introduce PostgreSQL alongside FDB, implement dual-read/-write shims, backfill, verify, and cut over behind feature flags.
- Map existing FDB keyspaces and JSON-encoded payloads to normalized relational tables with explicit constraints and indexes.
- Keep GraphQL schema stable; limit application code churn by adding a Postgres-backed adapter that fits existing interfaces.
- Roll out incrementally per domain (users/devices → teas/tags → QR/collections → notifications → consumption).


## 2. Current State (as of 2025-10-19)
The backend persists via PostgreSQL exclusively.
- Runtime storage: PostgreSQL using the schema in db/schema.sql with sqlc-generated accessors in pkg/pgstore and the thin adapter in pkg/pg.
- Consumption (history) uses a Postgres-backed Store (internal/consumption/pg_store.go) that also relies on sqlc-generated helpers.
- The legacy FDB backfill tool and helpers have been removed from main; recovery would require checking out the archival migration branch.


## 3. Target State (PostgreSQL)
- A normalized relational schema with explicit constraints and indexes mirroring existing access patterns.
- A new adapter package (e.g., pkg/pg) implementing the same or a minimally adjusted interface as pkg/fdb.DB and internal/consumption.Store.
- Runtime selection of backend via configuration: DATABASE_BACKEND=foundationdb|postgres and PG_DSN.
- Database migrations managed via a tool like golang-migrate or goose.


## 4. Data Model Mapping (FDB → PG)
This section enumerates current keyspaces and proposes corresponding tables and indexes.

Legend: PK = primary key, FK = foreign key, UQ = unique index/constraint.

### 4.1 Users and Devices
- FDB keys: Users(), User(id), UserByAppleID(apple_id)
- Devices: Device(id), DevicesByUserID(user_id) → stores []uuid UUIDs; device records hold token.

Proposed tables:
- users
  - id uuid PK
  - apple_id text NOT NULL UNIQUE
  - created_at timestamptz NOT NULL DEFAULT now()
- devices
  - id uuid PK
  - user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE
  - token text NOT NULL
  - created_at timestamptz NOT NULL DEFAULT now()
  - UNIQUE(token) -- optional but recommended
  - INDEX (user_id)

Behavioral notes:
- GetOrCreateUser(unique AppleID) → SELECT id FROM users WHERE apple_id=$1; INSERT IF NOT FOUND.
- AddDeviceForUser(user_id, device_id) currently maintains a per-user list of device IDs in a single KV; in PG store one row per device.
- MapUserIdToDeviceID → SELECT token FROM devices WHERE user_id=$1.

### 4.2 Teas (aka Records)
- FDB keys: Records(), Record(id), RecordsByName(name)
- Values: encoder.TeaData JSON with fields: name, type (string: tea|herb|coffee|other), description.

Proposed tables:
- teas
  - id uuid PK
  - name text NOT NULL
  - type text NOT NULL CHECK (type IN ('tea','herb','coffee','other'))
  - description text NULL
  - created_at timestamptz NOT NULL DEFAULT now()
  - UQ(name) — optional; FDB maps name→id implying 1:1; if duplicates are desired, drop UQ and keep an index for search only.

Indexes:
- CREATE INDEX teas_name_prefix_idx ON teas (lower(name) text_pattern_ops);
  - Enables case-insensitive prefix queries (ILIKE 'abc%'). Alternatively use citext.

### 4.3 Tag Categories, Tags, and Tea↔Tag relations
- FDB keys include TagCategory(id|byName), Tag(id|byName), composite indices for tag name+category, and per-entity relation indices (TagsByTea, TeasByTag).

Proposed tables:
- tag_categories
  - id uuid PK
  - name text NOT NULL UNIQUE
- tags
  - id uuid PK
  - name text NOT NULL
  - color text NOT NULL
  - category_id uuid NOT NULL REFERENCES tag_categories(id) ON DELETE RESTRICT
  - UNIQUE(category_id, lower(name)) -- enforce uniqueness within category, case-insensitive
  - INDEX (category_id)
- tea_tags (junction)
  - tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE
  - tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE CASCADE
  - PRIMARY KEY (tea_id, tag_id)
  - INDEX (tag_id)

### 4.4 QR Records
- FDB key: QR(id) → encoder.QR { Tea uuid, BowlingTemp int, ExpirationDate time.Time }

Proposed table:
- qr_records
  - id uuid PK
  - tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE
  - boiling_temp int NOT NULL
  - expiration_date timestamptz NOT NULL
  - created_at timestamptz NOT NULL DEFAULT now()
  - INDEX (tea_id)
  - INDEX (expiration_date)

### 4.5 Collections and Membership to QR Records
- FDB keys: Collection(id,userID), UserCollections(userID), CollectionsTeas(collectionID, teaID) where teaID is the QR id; RecordsByCollection(index prefix)
- Collection value holds { Name, UserID }.

Proposed tables:
- collections
  - id uuid PK
  - user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE
  - name text NOT NULL
  - created_at timestamptz NOT NULL DEFAULT now()
  - INDEX (user_id)
- collection_qr_items
  - collection_id uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE
  - qr_id uuid NOT NULL REFERENCES qr_records(id) ON DELETE CASCADE
  - PRIMARY KEY (collection_id, qr_id)
  - INDEX (qr_id)

List CollectionRecords(id) can be expressed via a JOIN on collection_qr_items → qr_records → teas.

### 4.6 Notifications
- FDB maintains NotificationByUserID(userID) as a list of UUIDs, and per-notification value with encoder.Notification { UserID, Type }.

Proposed table:
- notifications
  - id uuid PK
  - user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE
  - type smallint NOT NULL -- maps to common.NotificationType
  - created_at timestamptz NOT NULL DEFAULT now()
  - INDEX (user_id, created_at DESC)

### 4.7 Consumption (Recent History)
- FDB keys: ConsumptionByUserID(user), ConsumptionKey(userID, ts, teaID); value is empty — all meaning is in the key; internal/consumption has both MemoryStore and FDBStore.

Proposed table:
- consumptions
  - user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE
  - ts timestamptz NOT NULL
  - tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE
  - PRIMARY KEY (user_id, ts, tea_id)
  - INDEX (user_id, ts DESC)

Retention (e.g., 30 days) enforced by periodic deletion or date-based partitioning.


## 5. SQL Bootstrap (DDL Sketch)
Below is an initial schema sketch. Use goose/migrate to maintain versioned migrations.

```sql
-- Optional: enable citext for case-insensitive text handling on name-like columns
-- CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
  id uuid PRIMARY KEY,
  apple_id text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE teas (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  type text NOT NULL CHECK (type IN ('tea','herb','coffee','other')),
  description text,
  created_at timestamptz NOT NULL DEFAULT now()
);
-- Optional uniqueness mirroring FDB name→id index
-- ALTER TABLE teas ADD CONSTRAINT teas_name_uq UNIQUE (lower(name));
CREATE INDEX teas_name_prefix_idx ON teas (lower(name) text_pattern_ops);

CREATE TABLE tag_categories (
  id uuid PRIMARY KEY,
  name text NOT NULL UNIQUE
);

CREATE TABLE tags (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  color text NOT NULL,
  category_id uuid NOT NULL REFERENCES tag_categories(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX tags_category_name_uq ON tags (category_id, lower(name));
CREATE INDEX tags_category_idx ON tags (category_id);

CREATE TABLE tea_tags (
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (tea_id, tag_id)
);
CREATE INDEX tea_tags_tag_idx ON tea_tags (tag_id);

CREATE TABLE qr_records (
  id uuid PRIMARY KEY,
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  boiling_temp int NOT NULL,
  expiration_date timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX qr_records_tea_idx ON qr_records (tea_id);
CREATE INDEX qr_records_exp_idx ON qr_records (expiration_date);

CREATE TABLE collections (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX collections_user_idx ON collections (user_id);

CREATE TABLE collection_qr_items (
  collection_id uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  qr_id uuid NOT NULL REFERENCES qr_records(id) ON DELETE CASCADE,
  PRIMARY KEY (collection_id, qr_id)
);
CREATE INDEX collection_qr_items_qr_idx ON collection_qr_items (qr_id);

CREATE TABLE devices (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (token)
);
CREATE INDEX devices_user_idx ON devices (user_id);

CREATE TABLE notifications (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type smallint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_created_idx ON notifications (user_id, created_at DESC);

CREATE TABLE consumptions (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  ts timestamptz NOT NULL,
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, ts, tea_id)
);
CREATE INDEX consumptions_user_ts_desc_idx ON consumptions (user_id, ts DESC);
```


## 5.5. Tooling: sqlc (Query Code Generation)
To standardize Postgres access, we adopt sqlc to generate type-safe data access code from SQL.

Repository scaffolding already added:
- Config: db/sqlc.yaml (engine, outputs, type overrides)
- Schema: db/schema.sql (DDL used by sqlc for types)
- Queries: db/queries/*.sql (named queries grouped by domain)
- Output package: pkg/pgstore (sqlc target; generated code lives here)

How to generate code:
```
# 1) Install sqlc (see https://docs.sqlc.dev/en/latest/overview/install.html)
# 2) From repository root:
sqlc generate
```

Notes:
- The generated code uses pgx/v5 (github.com/jackc/pgx/v5). The dependency is present in go.mod.
- Keep SQL changes in db/queries/*.sql and db/schema.sql; re-run sqlc generate when they change.
- Thin adapters in pkg/pg wrap sqlc Queries to satisfy the existing pkg/fdb.DB method set where needed.

### 5.6. Backfill/Migration Utility
The FDB→PG backfill tool is available at `cmd/backfill/` and includes:
- **main.go** - Backfill implementation supporting users, teas, QR records, devices, and consumptions
- **README.md** - Detailed instructions for running the backfill
- **restore_fdb_deps.sh** - Helper script to restore FDB dependencies from git history (commit 06bca0c)

The backfill tool requires temporarily restoring FDB-related packages (pkg/fdbclient, common/key_value/*) that were removed from main after migration completion. See cmd/backfill/README.md for full instructions.

**Quick start:**
```bash
cd cmd/backfill
./restore_fdb_deps.sh
export PG_DSN="postgres://user:pass@localhost:5432/teaelephant"
go run main.go
```

## 6. Application Changes (Incremental)
Goal: keep GraphQL API unchanged; swap storage behind interfaces.

- Introduce a storage abstraction in a new package (pkg/store or pkg/pg with an interface):
  - For core features, pkg/fdb.DB is already an interface. Add a Postgres implementation with the same method set.
  - For consumption, implement internal/consumption.Store backed by PG.
- Add runtime selector (config/env):
  - DATABASE_BACKEND=foundationdb|postgres (default: foundationdb)
  - PG_DSN=postgres://user:pass@host:5432/dbname?sslmode=disable
- Wire server startup (internal/server/server.go) to construct either FDB or PG adapters based on configuration.
- Keep resolvers and managers unchanged; they depend on the DB interfaces.
- For read-after-write semantics, continue to use transactions in PG (SERIALIZABLE or REPEATABLE READ where needed); most operations are single-row UPSERTs.


## 7. Migration Strategy (Completed - Direct Cutover)
The migration was completed using a direct cutover approach rather than gradual dual-write.

1) Prepare
- Deploy PostgreSQL and run initial migrations.
- Implement PostgreSQL adapter (pkg/pg) with same interface as legacy FDB code.

2) Backfill
- Build a one-shot backfill tool (cmd/backfill):
  - Stream keys by prefix from FDB, decode via common/key_value/encoder, transform, and bulk-insert into PG.
  - Do in domain order with FK awareness: users → categories/tags → teas → tea_tags → qr_records → collections → collection_qr_items → devices → notifications → consumptions.
  - Use COPY for bulk throughput, batch size ~5–10k rows, idempotent on conflict DO NOTHING.

3) Verify
- Data reconciliation jobs compare row counts and sample data per table vs. FDB sources.
- Test all GraphQL queries and mutations against PostgreSQL backend.

4) Cut Over
- Update application to require PG_DSN and remove FDB initialization.
- Deploy new version with PostgreSQL-only backend.
- Monitor error rates and performance.

5) Decommission
- Snapshot FDB for archival.
- Remove FDB code paths from codebase.


## 8. ETL/Backfill Details
Pseudo-code snippet for a domain (teas):

```text
// Stream all Tea records from FDB and insert into PG
tr, _ := fdb.NewTransaction(ctx)
pr, _ := fdb.PrefixRange(kb.Records())
it := tr.GetIterator(pr)
var batch []TeaRow
for it.Advance() {
  kv, _ := it.Get()
  id, _ := uuid.FromBytes(kv.Key[1:])
  var td encoder.TeaData; _ = td.Decode(kv.Value)
  batch = append(batch, TeaRow{ID: id, Name: td.Name, Type: td.Type, Description: td.Description})
  if len(batch) == 5000 { pgCopy(batch); batch = batch[:0] }
}
if len(batch) > 0 { pgCopy(batch) }
```

Notes:
- Use the existing encoders (encoder.User, encoder.TagData, encoder.QR, etc.).
- For indices that stored arrays in FDB (e.g., DevicesByUserID, NotificationByUserID), materialize rows directly in normalized tables.
- For consumptions, parse key via key_builder.ParseConsumptionKey().


## 9. Concurrency, Consistency, and Performance
- FoundationDB provided strict serializability for transactions; PostgreSQL can match consistent semantics using transaction isolation:
  - Default to READ COMMITTED; elevate to REPEATABLE READ where multi-row lookups and subsequent writes must not see phantoms.
  - Use SELECT … FOR UPDATE on rows you intend to mutate in subsequent statements.
- Indexes defined above mirror FDB access paths (name/search, user-scoped lists).
- For heavy time-series reads (consumptions recent window), consider:
  - BRIN index on ts if volume is large
  - Partition by month if growth warrants



## 10. Operations and Configuration
- Required environment variable:
  - PG_DSN (PostgreSQL connection string)
- Docker Compose: add a postgres service with a persistent volume; expose port 5432.
- Kubernetes: add a StatefulSet/Deployment for PG if self-managed, or use a managed PostgreSQL service; inject PG_DSN via Secret; mount TLS if needed.
- Migrations: run on startup or as a separate Job using goose/migrate. Gate by MIGRATE_ON_START=true for dev.
- Backups: use pg_dump or snapshot tooling of managed service; define RPO/RTO.
- Observability: add pgbouncer/pg_stat_statements in non-prod; scrape PG exporter metrics; monitor query performance and error rates.


## 11. Testing Strategy
- Unit tests: introduce pg repos behind interfaces; add tests using dockertest or a sqlmock for behavior.
- Integration tests: run go test ./... with a test Postgres via docker-compose; reuse openweather gating pattern for external deps.
- Data verification: checksum comparisons during backfill; golden samples.


## 12. Milestones & Timeline (Completed)
The migration was completed in October 2025:
- Schema migrations and PG adapter implementation
- Backfill tool for data migration from FDB
- Full verification of all domains (users, teas, tags, QR, collections, notifications, consumption)
- Cutover to PostgreSQL-only backend
- FDB code decommissioned and removed from main branch


## 13. Risks and Mitigations (Historical)
Risks addressed during migration:
- Hidden invariants in KV layout (e.g., unique tea names): modeled with explicit constraints in PostgreSQL schema.
- Search behavior differences (prefix vs. case-insensitive): added appropriate indexes and use ILIKE with prefix.
- Data consistency during migration: used backfill tool with verification before cutover.
- Performance regressions: benchmarked critical queries (search by name, list tags by tea, collection listing, recent consumption) and added necessary indexes.


## 14. Acceptance Criteria
- All GraphQL queries and mutations are backed by PostgreSQL with identical observable behavior to the legacy FDB-backed API.
- Data parity validated: row counts match and spot-check hashes on each table after backfill.
- Operational readiness: migrations, backups, observability in place; README/docs updated.
- FoundationDB is no longer required at runtime; all deployments run with PG only (PG_DSN configured).
- FDB code paths and keyspace helpers are removed from the repository (or kept under an archival branch/tag), including pkg/fdb, pkg/fdbclient, common/key_value/key_builder, and common/key_value/encoder.
- Postgres access is mediated by sqlc-generated code in pkg/pgstore with pkg/pg providing the thin adapter layer documented here.
- A backfill utility exists at cmd/backfill with documented environment variables and example run command in this document.

## 18. Technical Concerns Addressed (Decisions)
1) UUID generation
- Policy: The application generates UUIDv4 for all new entities (users, teas, tags, qr_records, collections, devices, notifications). This keeps IDs stable between FDB and PG and avoids DB-side sequences.
- Backfill: Preserve existing FDB UUIDs when copying to PG. Dual-write uses the same app-generated ID for both backends.
- PG DDL: No DEFAULT gen_random_uuid() to avoid accidental divergence; rely on app-provided values.

2) Timestamp precision and timezone
- Store timestamps as timestamptz in PG; write and read in UTC. Go’s time.Time carries location; normalize to UTC at boundaries.
- Precision: PG timestamptz stores microsecond precision by default. Ensure comparisons tolerant to <=1µs differences if they occur (not expected with pgx). For ordering and retention, use >= and <= appropriately.
- Backfill: Convert FDB-encoded time.Time to UTC and insert; verify round-trip by sampling hashes.

3) Case sensitivity for names and search
- Short term (implemented): keep lower(name) indexes and use ILIKE 'prefix%'.
- Option (recommended): enable citext for columns such as users.apple_id, teas.name, tag_categories.name, tags.name to get consistent case-insensitive semantics without lower().
- Migration path: add CREATE EXTENSION citext and gradually migrate columns in a later migration if chosen; sql and code can continue using plain text in the interim.

4) Missing indexes
- Added an index on qr_records(boiling_temp) in db/schema.sql (qr_records_boiling_temp_idx) to support filtering by preparation temperature.
- Confirmed existing indexes for teas(lower(name)), qr_records(expiration_date), notifications(user_id, created_at desc), consumptions(user_id, ts desc).

5) Transaction boundaries for multi-entity ops across stores
- Primary store ensures atomicity using its native transactions (e.g., PG transaction that inserts a collection and its items together). Shadow writes are applied via idempotent replay from an outbox with a correlation ID to represent the same unit of work.
- Reads do not attempt cross-store merges; feature flags determine the single source of truth during migration.


## 15. Appendix A — FDB Keyspace → PG Table Reference
- Users(): users; UserByAppleID(): users.apple_id UQ
- Device(id): devices.id; DevicesByUserID(): devices.user_id index
- Records()/Record(): teas
- RecordsByName(): teas(lower(name)) index (optionally unique)
- TagCategory()/TagCategoryByName(): tag_categories(name UQ)
- Tag()/TagsByName()/TagsByCategory(): tags + indexes
- TagsByTea()/TeasByTag(): tea_tags
- QR(id): qr_records
- Collection(id,userID), UserCollections(): collections
- CollectionsTeas(), RecordsByCollection(): collection_qr_items
- Notification(id), NotificationByUserID(): notifications
- ConsumptionByUserID(), ConsumptionKey(): consumptions


## 16. Appendix B — Code Touch Points (Completed)
- pkg/pg: Implements the same methods as legacy pkg/fdb.DB (CreateTagCategory, ListTags, WriteRecord, etc.).
- internal/consumption: PG-backed Store implementation (pg_store.go).
- Configuration: cmd/server/main.go requires PG_DSN environment variable.
- CI: Postgres service container for integration tests.


---
This document provides the migration plan from FoundationDB to PostgreSQL. The migration has been completed using a direct cutover approach with comprehensive backfill and verification.

## 19. Post-migration Decommission (Completed)
All FoundationDB code and dependencies have been removed:
- ✅ Removed FDB application code:
  - pkg/fdb/* (DB adapters per domain)
  - pkg/fdbclient/* (FDB client wrapper)
  - common/key_value/key_builder/* (keyspace builders)
  - common/key_value/encoder/* (KV encoders)
  - cmd/backfill (archived for historical reference)
- ✅ All managers now use PG implementations:
  - pkg/pg is the canonical Postgres adapter
  - All managers (users, teas, tags, QR, collections, devices, notifications) wired to PG
- ✅ Clean build/runtime:
  - FoundationDB client libs removed from Dockerfile
  - deployment/server.yml uses only PG_DSN
  - No FDB-related volumes/mounts
- ✅ Codebase hygiene:
  - FDB bindings removed via go mod tidy
  - db/schema.sql maintained as source of truth
  - README updated to reflect PG-only architecture
- ✅ Operations:
  - Backups rely on PostgreSQL tooling only
  - Monitoring focused on PG metrics
