# FDB Backfill - Quick Reference

Quick reference for running the FDB → PostgreSQL data backfill.

## TL;DR

```bash
# 1. Update config/fdb.cluster with your FDB cluster connection
# 2. Update deployment/fdb-backfill/job.yml ConfigMap section

# 3. Push to GitHub to trigger build
git add .
git commit -m "Add FDB backfill configuration"
git push origin postgres-implementation

# 4. Wait for GitHub Actions build to complete (~5-10 min)

# 5. Deploy the Job
kubectl apply -f deployment/fdb-backfill/job.yml

# 6. Watch logs
kubectl logs -n teaelephant -f job/fdb-backfill

# 7. Cleanup when done
kubectl delete job fdb-backfill -n teaelephant
```

## Files in this Directory

- **`job.yml`** - Kubernetes Job manifest (apply this to run backfill)
- **`DEPLOYMENT.md`** - Complete deployment guide with troubleshooting
- **`server.yml`** - Alternative Deployment approach (not recommended)
- **`build-and-deploy.sh`** - Local build script (not needed for GitHub Actions)

## Prerequisites

1. **FDB cluster** accessible from Kubernetes
2. **PostgreSQL** database with schema applied
3. **Kubernetes secrets** configured:
   ```bash
   kubectl create secret generic postgres-dsn \
     --from-literal=dsn='postgres://...' \
     -n teaelephant
   ```

## What Gets Migrated

- ✅ Users
- ✅ Teas
- ⚠️ QR Records (main focus)
- ⚠️ Devices (main focus)
- ⏭️ Consumptions (skipped, implement if needed)

## Architecture

```
GitHub Push → GitHub Actions → Build Image → Push to ghcr.io
                                                     ↓
                                    Kubernetes Job pulls image
                                                     ↓
                              Job reads from FDB → writes to PostgreSQL
```

## Troubleshooting

**Job failed?**
```bash
kubectl describe job fdb-backfill -n teaelephant
kubectl logs -n teaelephant -l app=fdb-backfill
```

**Need to re-run?**
```bash
kubectl delete job fdb-backfill -n teaelephant
kubectl apply -f deployment/fdb-backfill/job.yml
```

**Check image was built:**
```bash
# Go to GitHub → Actions → "FDB Backfill Image"
```

## Documentation

- **Full deployment guide:** [`DEPLOYMENT.md`](DEPLOYMENT.md) ← START HERE
- **Overall backfill guide:** [`../../BACKFILL_GUIDE.md`](../../BACKFILL_GUIDE.md)
- **Backfill tool README:** [`../../cmd/backfill/README.md`](../../cmd/backfill/README.md)

## Support

Common issues and solutions in [`DEPLOYMENT.md`](DEPLOYMENT.md#troubleshooting)
