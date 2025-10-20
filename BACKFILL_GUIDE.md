# FDB → PostgreSQL Backfill Guide

This guide provides complete instructions for backfilling missing data from FoundationDB to PostgreSQL.

## Overview

After the initial migration, some data may not have been backfilled:
- ✅ Users (already migrated)
- ✅ Teas (already migrated)
- ⚠️ **QR Records** (needs backfill)
- ⚠️ **Devices** (needs backfill)
- ⚠️ Consumptions (optional)

## Quick Start

### Recommended: GitHub Actions + Kubernetes Job

**This is the preferred production approach.**

```bash
# 1. Update FDB cluster config in deployment/fdb-backfill/job.yml

# 2. Commit and push to trigger build
git add .
git commit -m "Add FDB backfill tool"
git push origin postgres-implementation

# 3. Wait for GitHub Actions to build image (5-10 min)
# GitHub → Actions → "FDB Backfill Image"

# 4. Deploy Kubernetes Job
kubectl apply -f deployment/fdb-backfill/job.yml

# 5. Monitor progress
kubectl logs -n teaelephant -f job/fdb-backfill

# 6. Cleanup after completion
kubectl delete job fdb-backfill -n teaelephant
```

**📖 Detailed instructions:** See [`deployment/fdb-backfill/DEPLOYMENT.md`](deployment/fdb-backfill/DEPLOYMENT.md)

### Alternative: Local Development/Testing

For local testing only (not for production):

```bash
# 1. Restore FDB dependencies
cd cmd/backfill
./restore_fdb_deps.sh

# 2. Set environment variables
export PG_DSN="postgres://user:pass@localhost:5432/teaelephant"
export DATABASEPATH="config/fdb.cluster"

# 3. Run backfill
cd ../..
go run cmd/backfill/main.go

# 4. Cleanup
mv go.mod.pg-only.bak go.mod
mv go.sum.pg-only.bak go.sum
rm -rf pkg/fdbclient common/key_value
go mod tidy
```

## Files Created

### Backfill Tool
- **`cmd/backfill/main.go`** - Backfill implementation
- **`cmd/backfill/README.md`** - Detailed usage instructions
- **`cmd/backfill/restore_fdb_deps.sh`** - Automated dependency restoration

### Docker & Deployment
- **`Dockerfile.fdb`** - Docker image with FDB client libraries
- **`deployment/fdb-backfill/server.yml`** - Kubernetes deployment config
- **`deployment/fdb-backfill/README.md`** - Kubernetes deployment guide
- **`deployment/fdb-backfill/build-and-deploy.sh`** - Automated build & deploy script

### Configuration
- **`config/fdb.cluster`** - FDB cluster connection string
- **`go.mod.fdb`** - go.mod with FDB dependencies (reference)
- **`go.sum.fdb`** - go.sum with FDB dependencies (reference)

## What Gets Backfilled

The backfill tool migrates data in this order:

### 1. Users
- User IDs and Apple IDs
- Already migrated, but re-run is safe (idempotent)

### 2. Teas
- Tea records with name, type, and description
- Already migrated, but re-run is safe (idempotent)

### 3. QR Records ⚠️ NEW
- QR code IDs
- Linked tea IDs
- Boiling temperatures
- Expiration dates
- Discovered by scanning user collections

### 4. Devices ⚠️ NEW
- Device IDs
- User associations
- Push notification tokens
- Discovered via user device indices

### 5. Consumptions (Optional)
- Currently skipped
- Implement if consumption history is needed

## Verification

After backfill, verify the data:

```sql
-- Check counts
SELECT 'users' as table_name, COUNT(*) as count FROM users
UNION ALL
SELECT 'teas', COUNT(*) FROM teas
UNION ALL
SELECT 'qr_records', COUNT(*) FROM qr_records
UNION ALL
SELECT 'devices', COUNT(*) FROM devices;

-- Sample data
SELECT * FROM qr_records
JOIN teas ON qr_records.tea_id = teas.id
LIMIT 10;

SELECT d.*, u.apple_id
FROM devices d
JOIN users u ON d.user_id = u.id
LIMIT 10;
```

## Troubleshooting

### "PG_DSN is required"
Set the environment variable:
```bash
export PG_DSN="postgres://user:pass@host:5432/dbname"
```

### "open FDB: ..."
- Ensure FoundationDB is running
- Verify `config/fdb.cluster` contains correct connection string
- Install FDB client libraries:
  ```bash
  # macOS
  brew install foundationdb

  # Linux
  # See https://apple.github.io/foundationdb/
  ```

### Cannot connect to PostgreSQL
Check DSN format and credentials:
```bash
psql "$PG_DSN" -c "SELECT 1"
```

### Decode errors in logs
Warnings like "skip QR X decode error" are normal for corrupted data. Valid records continue to process.

### Missing QR records
QR records are discovered by scanning collections. If collections weren't indexed properly in FDB, some QRs may be missed. Consider adding a direct QR keyspace scan if needed.

## Architecture

### How It Works

1. **FDB Scanning**
   - Connects to FoundationDB using legacy client
   - Scans keyspaces by prefix (Users, Records, etc.)
   - Decodes JSON-encoded values
   - Follows indices to discover relationships

2. **PostgreSQL Writing**
   - Uses idempotent `INSERT ... ON CONFLICT DO UPDATE`
   - Batch inserts for performance (1000 records per batch)
   - Transaction safety ensures consistency

3. **Dependency Restoration**
   - Temporarily restores FDB packages from git (commit c00e5bb)
   - Backs up current go.mod/go.sum
   - Restores FDB-compatible go.mod/go.sum
   - Cleanup restores PG-only dependencies

### Why This Approach?

- **Safety**: Idempotent operations allow re-runs without duplicates
- **Isolation**: FDB dependencies only present during backfill
- **Simplicity**: Direct read from FDB, write to PG (no dual-write complexity)
- **Verifiable**: Can compare counts and sample data before cutover

## Production Checklist

Before running in production:

- [ ] FDB cluster is accessible and healthy
- [ ] PostgreSQL database is provisioned with schema (db/schema.sql)
- [ ] Network connectivity between backfill pod and both databases
- [ ] Kubernetes secrets configured (postgres-dsn, etc.)
- [ ] FDB cluster file is correct (config/fdb.cluster)
- [ ] Sufficient disk space in PostgreSQL
- [ ] Backup of PostgreSQL database before backfill
- [ ] Maintenance window scheduled (if needed)
- [ ] Monitoring/alerts configured for backfill pod

After successful backfill:

- [ ] Verify data counts match expectations
- [ ] Sample data looks correct
- [ ] Application works correctly with new data
- [ ] Delete backfill deployment
- [ ] Clean up FDB dependencies from codebase
- [ ] Document backfill completion in migration plan

## Support

For issues:
1. Check logs: `kubectl logs -n teaelephant -l app=server-fdb-backfill`
2. Review troubleshooting section above
3. Verify FDB and PG connectivity independently
4. Check cmd/backfill/README.md and deployment/fdb-backfill/README.md

## Timeline

Typical backfill times (approximate):
- Small dataset (<1000 records): 1-5 minutes
- Medium dataset (1000-10000 records): 5-30 minutes
- Large dataset (>10000 records): 30+ minutes

Factors affecting time:
- FDB cluster performance
- Network latency
- PostgreSQL write performance
- Number of collections/relationships to traverse
