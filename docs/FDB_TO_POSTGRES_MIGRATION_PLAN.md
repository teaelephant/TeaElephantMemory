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

### 5.6. Backfill/Migration Utility (Historical)
The original FDB→PG backfill lived under `cmd/backfill` and relied on the legacy FDB helpers. Those components were removed from the main branch after the cut-over; recovering the tool requires checking out the archival migration branch or rebuilding from history.

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


## 7. Migration Strategy (Online, Zero/Low Downtime)
Phased rollout per domain with dual-write and verifications.

1) Prepare
- Ship code that can talk to both stores. Add feature flags:
  - FF_PG_DUAL_WRITE=true enables writing to PG in parallel with FDB
  - FF_PG_READ_PERCENT=0..100 gradual read shifting to PG for specific methods (start with 0)
- Deploy PostgreSQL and run initial migrations.

2) Backfill
- Build a one-shot backfill tool (cmd/backfill) or admin endpoint:
  - Stream keys by prefix from FDB, decode via common/key_value/encoder, transform, and bulk-insert into PG.
  - Do in domain order with FK awareness: users → categories/tags → teas → tea_tags → qr_records → collections → collection_qr_items → devices → notifications → consumptions.
  - Use COPY for bulk throughput, batch size ~5–10k rows, idempotent on conflict DO NOTHING.

3) Verify
- Data reconciliation jobs compare row counts and a sample checksum per table (e.g., hash of concatenated fields) vs. FDB sources.
- Enable dual-write in production and compare error rates/latency.

4) Shift Reads
- Per method, increase FF_PG_READ_PERCENT to route a fraction of reads to PG; monitor metrics and logs.
- If stable, move to 100% reads from PG for the method/domain.

5) Cut Over Writes
- After read cutover, turn off FDB writes per domain. Keep backfill job in catch-up mode until lag is 0.
- Finally, disable dual-write and remove FDB write paths.

6) Decommission
- Stop backfill; snapshot FDB; announce deprecation window; remove FDB code paths behind build tags/flags when safe.

Rollback plan:
- At any point, reduce FF_PG_READ_PERCENT to 0, disable dual-write, re-point reads and writes fully to FDB.


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

### 9.5 Cross-store Atomicity and Error Handling (Dual-Write)
This project will temporarily dual-write to FDB and PostgreSQL during migration. Because we cannot have a single atomic transaction across both data stores, we embrace an at-least-one-success policy with repair. The strategy is:
- Define the primary store per phase. Initially FDB is primary; PG is shadow. Later phases flip this.
- For each write:
  1) Perform the write in the primary store synchronously and return the primary error to the caller if it fails (no partial success is acknowledged).
  2) Fire a best-effort write to the shadow store. If it fails, record a durable outbox event for reconciliation.

Implementation details:
- Outbox table (in PG) or FDB key: write events with payload {domain, entity_id, op, version, body, error}.
- Background reconciler retries failed shadow writes with exponential backoff and dead-letter after N attempts. Observability metrics (success/failure/lag) are exported.
- Idempotency: all write operations must be idempotent. Use UPSERT/ON CONFLICT DO UPDATE in PG keyed by deterministic IDs; in FDB, Set() overwrites or conditional writes use transaction checks as needed.
- Read Consistency: while dual-writing, reads are served from a single selected backend (via FF_PG_READ_PERCENT per method). We do not merge results at read time to avoid complexity.
- Backfill Interaction: during backfill, dual-write remains on; reconciler ensures shadow catches up. After verification, we cut over primary.

Failure matrix:
- Primary fail, shadow not attempted or fail → return error to caller; no state change acknowledged.
- Primary success, shadow fail → acknowledge success; create outbox entry; reconciler repairs shadow. Verification jobs will also detect divergence.

Multi-entity operations:
- Wrap all related mutations in a single transaction in the primary store, then emit one composite outbox item (or a sequence with the same correlation ID) to replicate into the shadow using a transactional unit. Example: creating a collection and adding items must be atomic in the primary DB; the shadow replication applies the same unit serially with idempotency.


## 10. Operations and Configuration
- New env:
  - DATABASE_BACKEND (default foundationdb)
  - PG_DSN
- Docker Compose: add a postgres service with a persistent volume; expose port 5432.
- Kubernetes: add a StatefulSet/Deployment for PG if self-managed, or use a managed PostgreSQL service; inject PG_DSN via Secret; mount TLS if needed.
- Migrations: run on startup or as a separate Job using goose/migrate. Gate by MIGRATE_ON_START=true for dev.
- Backups: use pg_dump or snapshot tooling of managed service; define RPO/RTO.
- Observability: add pgbouncer/pg_stat_statements in non-prod; scrape PG exporter metrics; add application metrics per backend to compare latencies and error rates.


## 11. Testing Strategy
- Unit tests: introduce pg repos behind interfaces; add tests using dockertest or a sqlmock for behavior.
- Integration tests: run go test ./... with a test Postgres via docker-compose; reuse openweather gating pattern for external deps.
- Data verification: checksum comparisons during backfill; golden samples.


## 12. Milestones & Timeline (indicative)
1) Week 1–2: Schema migrations, PG adapter skeletons, config plumbing, CI jobs for PG.  
2) Week 3: Backfill tool, domain 1 (users/devices) dual-write + read cutover.  
3) Week 4: Teas/tags; search verification; tea_tags relations.  
4) Week 5: QR and collections; collection listing parity checks.  
5) Week 6: Notifications; consumption store; retention jobs.  
6) Week 7: Turn off dual-write, decommission FDB, archival.


## 13. Risks and Mitigations
- Hidden invariants in KV layout (e.g., unique tea names): model with explicit constraints or preserve behavior in code.
- Search behavior differences (prefix vs. case-insensitive): add appropriate indexes and use ILIKE with prefix.
- Backfill races during live writes: dual-write first, then incremental backfill to converge; use last-updated timestamps where available or compare row existence only if immutable; otherwise design a conflict policy.
- Performance regressions: benchmark critical queries (search by name, list tags by tea, collection listing, recent consumption). Add indexes or caching as needed.


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


## 16. Appendix B — Code Touch Points
- New package: pkg/pg implementing the same methods as pkg/fdb.DB (CreateTagCategory, ListTags, WriteRecord, etc.).
- internal/consumption: add PG-backed Store implementation; wire via constructor in internal/server/server.go.
- Configuration: extend internal/server/server.go to parse DATABASE_BACKEND and PG_DSN.
- CI: add service container for postgres in GitHub Actions or use docker-compose for local runs; add a make target to run migrations.


## 17. Appendix C — Example Dual-Write Wrapper
```text
type DualDB struct { primary, shadow fdb.DB }
func (d *DualDB) WriteRecord(ctx context.Context, rec *common.TeaData) (*common.Tea, error) {
  tea, err := d.primary.WriteRecord(ctx, rec)
  if err != nil { return nil, err }
  go func(t *common.Tea){ _ = d.shadow.Update(context.Background(), t.ID, t.TeaData) }(tea)
  return tea, nil
}
// Apply same pattern for other write methods; reads are routed via feature flag.
```


---
This plan keeps changes minimal in application layers, focuses on safety via dual-write and verification, and provides concrete table schemas and roll-out steps tailored to TeaElephantMemory’s current FoundationDB layouts and usages.

## 19. Post-migration Decommission Plan — Remove FoundationDB and Key-Value Relations
- Remove FDB application code:
  - Delete pkg/fdb/* (DB adapters per domain).
  - Delete pkg/fdbclient/* (FDB client wrapper).
  - Delete common/key_value/key_builder/* (keyspace builders) and common/key_value/encoder/* (KV encoders).
  - Delete config/fdb.cluster and fdb-go-install.sh from the repo.
- Replace remaining FDB usages in managers/resolvers with PG implementations:
  - Keep pkg/pg as the canonical Postgres adapter; extend it alongside the sqlc-generated pkg/pgstore queries as features evolve.
  - Wire all managers (users, teas, tags, QR, collections, devices, notifications) to PG repositories.
- Clean up build/runtime dependencies:
  - Remove FoundationDB client libs from Dockerfile and CI (no foundationdb-clients packages).
  - Remove DATABASEPATH and DATABASE_BACKEND from deployment manifests; keep only PG_DSN.
  - Remove FDB-related volumes/mounts from deployment/server.yml (ConfigMap fdb.cluster, paths under /etc/fdb/).
- Configuration and flags:
  - Remove FF_PG_DUAL_WRITE and FF_PG_READ_PERCENT flags from code and manifests (obsolete after full cutover).
- Codebase hygiene:
  - Run go mod tidy to drop FDB bindings and transitive deps.
  - Update db/schema.sql alongside any query changes and keep pg-specific unit tests covering new behaviour.
  - Update README/docs to reflect PG-only architecture and decommission timeline.
- Observability and ops:
  - Remove FDB dashboards/alerts; ensure PG metrics are primary.
  - Backups and DR now rely on PostgreSQL tooling only.
