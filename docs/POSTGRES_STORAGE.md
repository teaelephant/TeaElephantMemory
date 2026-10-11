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

## Data model

| Tables | Responsibility |
|---|---|
| `users` | UUID identities with unique Apple IDs |
| `teas` | Shared beverage catalogue: name, type, description |
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
