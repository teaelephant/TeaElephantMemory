# Ready to Push - FDB Backfill

Everything is ready. Just commit and push to trigger the workflow.

## Quick Start (TL;DR)

```bash
# 1. Commit everything
git add .
git commit -m "Add FDB backfill tool with GitHub Actions and Kubernetes Job"
git push origin postgres-implementation

# 2. Wait for GitHub Actions to build image (~5-10 min)
# Go to: GitHub → Actions → "FDB Backfill Image"

# 3. Apply ConfigMap and Job
kubectl apply -f deployment/fdb-backfill/configmap.yml
kubectl apply -f deployment/fdb-backfill/job.yml

# 4. Watch it run
kubectl logs -n teaelephant -f job/fdb-backfill

# 5. Cleanup when done
kubectl delete job fdb-backfill -n teaelephant
```

## What You're Committing

### GitHub Actions
- `.github/workflows/fdb-backfill-image.yml` - Auto-builds Docker image

### Backfill Tool
- `cmd/backfill/main.go` - Migrates QR records + devices
- `cmd/backfill/README.md` - Usage docs
- `cmd/backfill/restore_fdb_deps.sh` - Restores FDB code

### Kubernetes Job
- `deployment/fdb-backfill/job.yml` - **The file you'll apply**
- `deployment/fdb-backfill/DEPLOYMENT.md` - Full deployment guide
- `deployment/fdb-backfill/README.md` - Quick reference

### Supporting Files
- `Dockerfile.fdb` - Docker build with FDB support
- `BACKFILL_GUIDE.md` - Overall guide
- Updated `.gitignore` and migration plan

## Prerequisites (Already Done ✅)

- ✅ FDB cluster ConfigMap exists in Kubernetes
- ✅ Secret `postgres-dsn` exists
- ✅ Secret `regcred` exists

## Expected Output

When the Job runs successfully:
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

## Verify Data

```sql
SELECT 'qr_records' as table_name, COUNT(*) FROM qr_records
UNION ALL
SELECT 'devices', COUNT(*) FROM devices;
```

## Troubleshooting

**Job fails?**
```bash
kubectl describe job fdb-backfill -n teaelephant
kubectl logs -n teaelephant -l app=fdb-backfill
```

**Need full guide?**
See `deployment/fdb-backfill/DEPLOYMENT.md`

---

**That's it!** Push and deploy. 🚀
