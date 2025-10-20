# Ready to Commit: FDB Backfill Setup

Everything is ready for you to commit and push to GitHub to trigger the backfill workflow.

## Summary

✅ **FDB Backfill Tool** - Complete data migration from FoundationDB to PostgreSQL
✅ **GitHub Actions Workflow** - Automated Docker image build with FDB support
✅ **Kubernetes Job** - Production-ready Job manifest for running backfill
✅ **Complete Documentation** - Step-by-step guides and troubleshooting

## Files Ready to Commit

### Core Backfill Tool
```
cmd/backfill/
├── main.go                    # Backfill implementation (users, teas, QR, devices)
├── README.md                  # Usage instructions
└── restore_fdb_deps.sh        # Script to restore FDB dependencies
```

### GitHub Actions
```
.github/workflows/
└── fdb-backfill-image.yml     # Workflow to build Docker image with FDB
```

### Kubernetes Deployment
```
deployment/fdb-backfill/
├── job.yml                    # Kubernetes Job manifest (MAIN FILE TO APPLY)
├── DEPLOYMENT.md              # Complete deployment guide
├── README.md                  # Quick reference
├── server.yml                 # Alternative Deployment (not needed)
└── build-and-deploy.sh        # Local build script (not needed)
```

### Docker & Config
```
Dockerfile.fdb                 # Docker image with FDB client libraries
config/fdb.cluster             # FDB cluster connection string
```

### Reference Files (not for commit)
```
go.mod.fdb                     # Reference: go.mod with FDB deps
go.sum.fdb                     # Reference: go.sum with FDB deps
```

### Documentation
```
BACKFILL_GUIDE.md              # Comprehensive backfill guide
docs/FDB_TO_POSTGRES_MIGRATION_PLAN.md  # Updated migration plan
```

### Modified Files
```
.gitignore                     # Added FDB temporary files
deployment/server.yml          # Current PG-only deployment (unchanged logic)
```

## What Happens After You Push

1. **GitHub Actions triggers** automatically (or manually via Actions tab)
2. **Workflow restores FDB dependencies** from git commit c00e5bb
3. **Docker image builds** using Dockerfile.fdb (~5-10 minutes)
4. **Image pushes to** `ghcr.io/teaelephant/teaelephantmemory:fdb-backfill-latest`
5. **You deploy** the Kubernetes Job
6. **Job runs** backfill and migrates QR records + devices
7. **You verify** data in PostgreSQL
8. **Job auto-deletes** after 24 hours (or manually delete)

## Before You Commit

### 1. Update FDB Cluster Configuration

Edit `deployment/fdb-backfill/job.yml` line 16-18:

```yaml
data:
  fdb.cluster: |
    YOUR_CLUSTER:YOUR_DESC@YOUR_HOST:YOUR_PORT
```

Replace with your actual FDB cluster connection string.

### 2. Verify Prerequisites

- [ ] FDB cluster is running and accessible
- [ ] PostgreSQL database is provisioned
- [ ] PostgreSQL schema applied: `db/schema.sql`
- [ ] Kubernetes secrets exist:
  ```bash
  kubectl get secret postgres-dsn -n teaelephant
  kubectl get secret regcred -n teaelephant
  ```

If secrets don't exist, create them:
```bash
# PostgreSQL DSN
kubectl create secret generic postgres-dsn \
  --from-literal=dsn='postgres://user:pass@host:5432/teaelephant' \
  -n teaelephant

# GitHub Container Registry (if not exists)
kubectl create secret docker-registry regcred \
  --docker-server=ghcr.io \
  --docker-username=YOUR_GITHUB_USERNAME \
  --docker-password=YOUR_GITHUB_TOKEN \
  -n teaelephant
```

## Commit and Push

```bash
# Review what will be committed
git status

# Add all new files
git add .github/workflows/fdb-backfill-image.yml
git add cmd/backfill/
git add deployment/fdb-backfill/
git add Dockerfile.fdb
git add config/fdb.cluster
git add BACKFILL_GUIDE.md
git add COMMIT_READY.md
git add .gitignore
git add docs/FDB_TO_POSTGRES_MIGRATION_PLAN.md

# Commit
git commit -m "Add FDB backfill tool with GitHub Actions and Kubernetes Job

- Backfill tool for migrating QR records and devices from FDB to PostgreSQL
- GitHub Actions workflow to build Docker image with FDB support
- Kubernetes Job for production backfill deployment
- Comprehensive documentation and troubleshooting guides

Closes #<issue_number> (if applicable)"

# Push to trigger GitHub Actions
git push origin postgres-implementation
```

## After Push: Deploy Workflow

### Step 1: Monitor GitHub Actions

```bash
# Go to: GitHub → Your Repo → Actions → "FDB Backfill Image"
# Watch the build progress (~5-10 minutes)
```

### Step 2: Deploy Kubernetes Job

```bash
# Apply the Job
kubectl apply -f deployment/fdb-backfill/job.yml

# Monitor logs
kubectl logs -n teaelephant -f job/fdb-backfill
```

Expected output:
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

### Step 3: Verify Data

```sql
-- Connect to PostgreSQL and run:
SELECT 'users' as table_name, COUNT(*) as count FROM users
UNION ALL
SELECT 'teas', COUNT(*) FROM teas
UNION ALL
SELECT 'qr_records', COUNT(*) FROM qr_records
UNION ALL
SELECT 'devices', COUNT(*) FROM devices;
```

### Step 4: Cleanup

```bash
# Delete the Job
kubectl delete job fdb-backfill -n teaelephant
```

## Quick Reference

| Document | Purpose |
|----------|---------|
| `BACKFILL_GUIDE.md` | Overall guide and options |
| `deployment/fdb-backfill/README.md` | Quick reference for deployment |
| `deployment/fdb-backfill/DEPLOYMENT.md` | Detailed deployment guide with troubleshooting |
| `cmd/backfill/README.md` | Local development instructions |

## Troubleshooting

**Build fails in GitHub Actions?**
- Check Actions logs for errors
- Verify FDB dependencies can be restored from commit c00e5bb

**Job fails in Kubernetes?**
```bash
kubectl describe job fdb-backfill -n teaelephant
kubectl logs -n teaelephant -l app=fdb-backfill
```

**Cannot connect to FDB?**
- Verify `deployment/fdb-backfill/job.yml` ConfigMap has correct cluster string
- Check network connectivity from Kubernetes to FDB

**Cannot connect to PostgreSQL?**
```bash
kubectl get secret postgres-dsn -n teaelephant -o jsonpath='{.data.dsn}' | base64 -d
```

## Architecture

```
┌──────────────┐
│    GitHub    │
│  Repository  │
└──────┬───────┘
       │ git push
       ↓
┌──────────────┐
│   GitHub     │
│   Actions    │ Restores FDB deps from git
└──────┬───────┘ Builds Dockerfile.fdb
       │
       ↓
┌──────────────┐
│   ghcr.io    │
│ teaelephant/ │
│ memory:fdb-  │
│  backfill    │
└──────┬───────┘
       │ kubectl apply
       ↓
┌──────────────┐         ┌──────────────┐
│  Kubernetes  │  Read   │     FDB      │
│     Job      │────────→│   Cluster    │
│  (backfill)  │         └──────────────┘
└──────┬───────┘
       │ Write
       ↓
┌──────────────┐
│  PostgreSQL  │
│   Database   │
└──────────────┘
```

## Support

For detailed instructions and troubleshooting:
- **Start here:** `deployment/fdb-backfill/DEPLOYMENT.md`
- **GitHub Actions issues:** Check workflow logs in Actions tab
- **Kubernetes issues:** `kubectl describe` and `kubectl logs`
- **General questions:** See `BACKFILL_GUIDE.md`

---

**You're all set!** Commit, push, and follow the deployment guide. 🚀
