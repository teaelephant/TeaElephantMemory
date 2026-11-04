# PostgreSQL Storage

PostgreSQL is the backend's only storage engine. The server requires `PG_DSN`,
opens a `pgx` connection through `database/sql`, and checks connectivity before
starting the API.

## Runtime code

- `cmd/server/main.go` creates the connection and wires it into the services.
- `pkg/pg/db.go` implements storage operations for managers and authentication.
- `pkg/pgstore` contains the query helpers imported by the runtime adapter.
- `internal/consumption/pg_store.go` uses the same connection for consumption history.
- `db/schema.sql` defines tables, constraints, and indexes.
- `db/queries` contains the SQL definitions.

The current `db/sqlc.yaml` generates into `db/pkg/pgstore`, while the running
application imports `pkg/pgstore`. Reconcile these paths and adapter interfaces
before replacing runtime query helpers with newly generated code.

Device-token registration follows the scoped direct-SQL runtime pattern: `pkg/pg/db.go` updates only a row matching both device ID and the authenticated user ID, then requires an affected row. `db/queries/devices.sql` records the same owner predicate and `:execrows` result; configured generated helpers in `db/pkg/pgstore` are kept synchronized. The legacy unscoped helper in `pkg/pgstore` is not called for token registration. Device binding remains unchanged and never transfers ownership on an ID conflict. This role-alignment change needs no schema migration.

`pkg/pg/tea_creation.go` follows the same direct-SQL pattern for `WriteRecordWithTags`. A transaction validates and locks existing initial tags, inserts a Tea with a fresh UUID, inserts deduplicated tag links, and commits them together. Unknown tags or any insertion/commit failure roll back every write. Existing entries, tag definitions and galleries are never updated. The manager queues the creation event only after successful commit. The optional GraphQL `newTea` argument `tagIDs: [ID!]! = []` keeps older calls compatible; selecting initial tags grants no right to change a saved Tea. Existing `tea_tags` constraints suffice, so no new migration is needed.

## Data model

| Tables | Responsibility |
|---|---|
| `users` | UUID identities with unique Apple IDs |
| `teas` | Shared beverage catalogue: name, type, description |
| `tea_images` | Validated JPEG/PNG bytes and metadata linked to a Tea ID, with one selected cover |
| `tag_categories`, `tags`, `tea_tags` | Categorized tags and catalogue assignments |
| `qr_records` | Individual packages linked to a tea, with boiling temperature and expiration |
| `collections`, `collection_qr_items` | User-owned collections of QR records |
| `devices`, `notifications` | Push tokens and notification records |
| `consumptions` | Per-user tea history keyed by user, timestamp, and tea |

Collections contain QR-record IDs. The tea ID describes a catalogue entry;
multiple QR records can refer to that same tea.

Application code generates entity UUIDs. Timestamp columns use `timestamptz`,
and GraphQL dates map to Go `time.Time`. Foreign keys and uniqueness constraints
are specified in `db/schema.sql`.

## Schema setup

The local Compose PostgreSQL service initializes a new database using
`db/schema.sql`. Initialization scripts run only for an empty data directory.
The named `postgres-data-v18` volume persists across ordinary
`docker compose down` calls. It is mounted at `/var/lib/postgresql`, with
PostgreSQL 18 data under `/var/lib/postgresql/18/docker`.

The older PostgreSQL 17 `postgres-data` volume is preserved separately.
Compose does not migrate it or attach it to PostgreSQL 18. Back up any existing
database and explicitly migrate it with dump/restore or `pg_upgrade` before
using the new database for existing data.

The server does not apply schema migrations at startup. Apply reviewed schema
changes explicitly to existing databases, after taking a PostgreSQL backup.
Keep the SQL definitions and runtime query helpers aligned.

The current setup and environment variables are documented in [README](../README.md).
Retired migration tooling remains recoverable from Git history.

## Tea image storage

`db/migrations/20261011_tea_images.sql` creates the image table and indexes
idempotently for existing databases. `db/schema.sql` includes the same
objects for fresh databases. Apply the migration explicitly before rollout;
server startup does not apply it.

`pkg/pg/tea_images.go` owns the new runtime SQL through `database/sql` directly,
keeping the existing sqlc adapter mismatch unchanged. Metadata reads omit
`bytea` data; the HTTP image route loads bytes separately. Image IDs identify
immutable content. The Tea foreign key cascades image deletion.

All image writes lock the parent Tea row in one transaction. This serializes
the ten-image limit, first-image cover assignment, cover switches and cover
promotion. The partial unique index also prevents more than one cover per
Tea. Image removal and cover selection constrain both Tea ID and image ID.

## Versioned catalogue operations

The additive `db/migrations/20261012_catalog_agent_operations.sql` must be applied
explicitly after the tea-image migration. It adds opaque entity revisions and a
`catalog_operation_receipts` table. Matching definitions are in `db/schema.sql`.
The migration is repeatable and preserves shared Teas, images, QR records and
collection membership. Triggers increment catalogue revisions even for older
clients omitting preconditions; Tea tag/photo triggers advance the parent Tea.

`pkg/pg/catalogue_records.go`, `catalogue_operations.go`, `tea_creation.go` and
`tea_images.go` implement runtime catalogue reads and atomic guarded writes with
`database/sql`. They return row metadata and its revision together and do not
modify the legacy generated `pkg/pgstore` file. Configured sqlc output in
`db/pkg/pgstore` is regenerated from source queries, including receipt definitions.

Catalogue transactions take one transaction advisory lock before entity locks,
which orders small household catalogue writes and taxonomy cascades. A revision
conflict and failed creation/upload roll back every write. Successful keyed
creation/upload commits a resource receipt and payload hash with the data.
Receipts have no resource foreign key so they survive deliberate deletion without
allowing a retry to resurrect the resource. See [Editor agent API](EDITOR_AGENT_API.md)
for the optional GraphQL arguments, strict capability checks and current limits.

## Duplicate Tea merge

`db/migrations/20261012_tea_merge.sql` expands the receipt operation constraint and
adds `tea_merge_receipt_details`, with immutable input revisions/token, original
metadata and historical transfer counts. It follows the image and catalogue
operation migrations and requires an explicit backup/application. It is repeatable
and does not migrate or delete existing Tea/QR/image/collection/history rows.

`pkg/pg/tea_merge.go` and `tea_merge_preview.go` implement the single transaction
and snapshot hash. They take the catalogue advisory lock, dependent table locks,
then Tea row locks; this orders legacy QR/membership/history writers without FK
lock cycles. Source QR/photo IDs are preserved; tags are unioned and consumption
collisions/image overflow refuse the entire merge. Historical receipts have no
Tea FK and stay untouched. `internal/consumption/qr_selection.go` uses the same
lock only during fresh Tea-of-the-Day QR selection/history recording, after remote
scoring; cache hits record nothing. See [Tea merge](TEA_MERGE.md) for API policies,
receipt recovery and the explicit scalability/rollout boundaries.
