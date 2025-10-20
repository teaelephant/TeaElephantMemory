# FDB to PostgreSQL Backfill Tool

This tool migrates data from FoundationDB to PostgreSQL for the TeaElephantMemory project.

## Prerequisites

Before running the backfill tool, you need to restore the FoundationDB-related code from git history since it was removed in the main branch after migration.

### Restoring FDB Dependencies

The following packages need to be temporarily restored from commit `c00e5bb`:

1. `pkg/fdbclient/` - FDB client wrapper
2. `common/key_value/key_builder/` - FDB keyspace builders
3. `common/key_value/encoder/` - FDB value encoders

**Restore commands:**

```bash
# From repository root
git show c00e5bb:pkg/fdbclient/client.go > pkg/fdbclient/client.go
git show c00e5bb:common/key_value/key_builder/builder.go > common/key_value/key_builder/builder.go
git show c00e5bb:common/key_value/key_builder/keys.go > common/key_value/key_builder/keys.go
git show c00e5bb:common/key_value/encoder/encoder.go > common/key_value/encoder/encoder.go

# Add FDB dependency to go.mod
go get github.com/apple/foundationdb/bindings/go@v0.0.0-20231107151356-57ccdb8fee6d
go mod tidy
```

### FoundationDB Runtime

You also need FoundationDB client libraries installed on your system:

```bash
# macOS (example)
brew install foundationdb

# Linux
# Follow instructions at https://apple.github.io/foundationdb/
```

## Environment Variables

- `DATABASEPATH` - (Optional) Path to FoundationDB cluster file. If not set, uses system default.
- `PG_DSN` - (Required) PostgreSQL connection string, e.g., `postgres://user:pass@localhost:5432/teaelephant?sslmode=disable`

## Running the Backfill

```bash
export PG_DSN="postgres://user:pass@localhost:5432/teaelephant?sslmode=disable"
export DATABASEPATH="/path/to/fdb.cluster"  # Optional

go run cmd/backfill/main.go
```

## What Gets Backfilled

The tool migrates the following data in order:

1. **Users** - User accounts with Apple IDs
2. **Teas** - Tea records with name, type, and description
3. **QR Records** - QR codes linked to teas with expiration dates and brewing temperatures
4. **Devices** - User devices for push notifications
5. **Consumptions** - Tea consumption history (currently skipped, implement if needed)

## Output

The tool provides progress logging:

```
Starting backfill...
  Inserted 42 users
✓ Users backfilled
  Inserted 156 tea records
✓ Teas backfilled
  Inserted 89 QR records
✓ QR records backfilled
  Inserted 23 devices
✓ Devices backfilled
  Skipping consumptions (implement if needed)
✓ Consumptions backfilled
Backfill completed successfully!
```

## Idempotency

All insert operations use `ON CONFLICT DO UPDATE`, making the backfill idempotent. You can run it multiple times safely.

## After Backfill

Once the backfill is complete and verified:

1. Remove the temporarily restored FDB packages
2. Run `go mod tidy` to clean up FDB dependencies
3. Verify data in PostgreSQL:

```sql
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM teas;
SELECT COUNT(*) FROM qr_records;
SELECT COUNT(*) FROM devices;
```

## Troubleshooting

### "PG_DSN is required"
Set the `PG_DSN` environment variable with a valid PostgreSQL connection string.

### "open FDB: ..."
Ensure FoundationDB is running and the cluster file path is correct. Check `DATABASEPATH` or use the system default.

### Decode Errors
Some warnings like "skip tea X decode error" are normal if there's corrupted data in FDB. The tool will continue processing other records.

### Missing QR Records
QR records are discovered by scanning user collections. If collections weren't properly indexed in FDB, some QRs may be missed. Consider adding a direct QR keyspace scan if needed.

## Notes

- The tool uses batch inserts with a batch size of 1000 records for performance
- Transaction isolation ensures consistency during migration
- Collections, tags, and notifications are not yet implemented in this version
- For production use, consider running in a maintenance window or during low-traffic periods
