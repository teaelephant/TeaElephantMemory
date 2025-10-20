# FDB Backfill Deployment Guide

Complete guide for running the FDB → PostgreSQL backfill using GitHub Actions and Kubernetes Jobs.

## Overview

This approach uses:
1. **GitHub Actions** - Builds Docker image with FDB support
2. **GitHub Container Registry** - Stores the built image
3. **Kubernetes Job** - Runs the backfill as a one-time job

## Prerequisites

Before starting, ensure you have:

- [ ] Access to the FoundationDB cluster
- [ ] PostgreSQL database provisioned with schema applied (`db/schema.sql`)
- [ ] Kubernetes cluster access with `kubectl` configured
- [ ] GitHub repository access (to trigger workflows)
- [ ] Kubernetes secrets configured (see below)

### Required Kubernetes Secrets

```bash
# PostgreSQL connection string
kubectl create secret generic postgres-dsn \
  --from-literal=dsn='postgres://user:pass@host:5432/teaelephant?sslmode=disable' \
  -n teaelephant

# GitHub Container Registry credentials (if not already configured)
kubectl create secret docker-registry regcred \
  --docker-server=ghcr.io \
  --docker-username=YOUR_GITHUB_USERNAME \
  --docker-password=YOUR_GITHUB_TOKEN \
  -n teaelephant
```

## Step-by-Step Deployment

### Step 1: Update FDB Cluster Configuration

Edit `deployment/fdb-backfill/job.yml` and update the ConfigMap with your FDB cluster connection:

```yaml
data:
  fdb.cluster: |
    YOUR_CLUSTER_ID:YOUR_CLUSTER_DESC@HOST:PORT
```

Example:
```yaml
data:
  fdb.cluster: |
    docker:docker@10.5.0.6:4500
```

### Step 2: Commit and Push Changes

```bash
# Add all backfill-related files
git add cmd/backfill/
git add deployment/fdb-backfill/
git add .github/workflows/fdb-backfill-image.yml
git add Dockerfile.fdb
git add config/fdb.cluster
git add BACKFILL_GUIDE.md

# Commit
git commit -m "Add FDB backfill tool and deployment configuration"

# Push to trigger the workflow
git push origin postgres-implementation
```

### Step 3: Trigger GitHub Actions Build

**Option A: Automatic (on push)**
The workflow triggers automatically when you push changes to `postgres-implementation` branch.

**Option B: Manual trigger**
1. Go to GitHub repository → Actions tab
2. Select "FDB Backfill Image" workflow
3. Click "Run workflow"
4. Enter image tag (e.g., `fdb-backfill-v1`)
5. Click "Run workflow"

### Step 4: Wait for Build Completion

Monitor the workflow:
1. GitHub → Actions → "FDB Backfill Image"
2. Watch the build progress
3. Note the image tag from the workflow summary

Expected build time: 5-10 minutes

The workflow will build and push:
- `ghcr.io/teaelephant/teaelephantmemory:fdb-backfill-latest`
- `ghcr.io/teaelephant/teaelephantmemory:fdb-backfill-YYYYMMDD-HHMMSS`

### Step 5: Verify Image was Pushed

```bash
# Check if image exists (requires GitHub token)
docker pull ghcr.io/teaelephant/teaelephantmemory:fdb-backfill-latest
```

### Step 6: Apply ConfigMap (if not exists)

```bash
# Apply just the ConfigMap first
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: fdb-cluster
  namespace: teaelephant
data:
  fdb.cluster: |
    docker:docker@10.5.0.6:4500
EOF
```

Update the connection string to match your FDB cluster.

### Step 7: Deploy the Kubernetes Job

```bash
# Apply the Job
kubectl apply -f deployment/fdb-backfill/job.yml

# Verify Job was created
kubectl get jobs -n teaelephant
```

### Step 8: Monitor Backfill Progress

```bash
# Watch the Job status
kubectl get jobs -n teaelephant -w

# Stream logs in real-time
kubectl logs -n teaelephant -f job/fdb-backfill

# Check pod status
kubectl get pods -n teaelephant -l app=fdb-backfill
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

### Step 9: Verify Data Migration

```bash
# Connect to PostgreSQL
kubectl run psql-temp --rm -it --image=postgres:15 -n teaelephant -- \
  psql "$PG_DSN"

# Or use your local psql
export PG_DSN="postgres://user:pass@host:5432/teaelephant"
psql "$PG_DSN"
```

Run verification queries:
```sql
-- Check row counts
SELECT 'users' as table_name, COUNT(*) as count FROM users
UNION ALL
SELECT 'teas', COUNT(*) FROM teas
UNION ALL
SELECT 'qr_records', COUNT(*) FROM qr_records
UNION ALL
SELECT 'devices', COUNT(*) FROM devices;

-- Verify QR records are linked to teas
SELECT qr.id, t.name, qr.boiling_temp, qr.expiration_date
FROM qr_records qr
JOIN teas t ON qr.tea_id = t.id
LIMIT 10;

-- Verify devices are linked to users
SELECT d.id, u.apple_id, d.token
FROM devices d
JOIN users u ON d.user_id = u.id
LIMIT 10;
```

### Step 10: Cleanup

After successful verification:

```bash
# Delete the Job (keeps logs for 24h due to ttlSecondsAfterFinished)
kubectl delete job fdb-backfill -n teaelephant

# Or wait for automatic cleanup after 24 hours

# Optionally, delete the ConfigMap if no longer needed
# kubectl delete configmap fdb-cluster -n teaelephant
```

## Troubleshooting

### Job Failed or Stuck

```bash
# Check Job status
kubectl describe job fdb-backfill -n teaelephant

# Check pod events
kubectl describe pod -n teaelephant -l app=fdb-backfill

# View logs from failed pod
kubectl logs -n teaelephant -l app=fdb-backfill --previous
```

### Common Issues

#### "PG_DSN is required"
The secret is not properly configured:
```bash
kubectl get secret postgres-dsn -n teaelephant
kubectl describe secret postgres-dsn -n teaelephant
```

Recreate if needed:
```bash
kubectl delete secret postgres-dsn -n teaelephant
kubectl create secret generic postgres-dsn \
  --from-literal=dsn='postgres://user:pass@host:5432/teaelephant' \
  -n teaelephant
```

#### "ImagePullBackOff"
Image pull credentials are missing or incorrect:
```bash
kubectl describe pod -n teaelephant -l app=fdb-backfill | grep -A 5 Events
```

Verify `regcred` secret exists and is valid.

#### "Cannot connect to FDB"
- FDB cluster file is incorrect in ConfigMap
- Network policies block access to FDB coordinator
- FDB cluster is down

Debug:
```bash
# Check ConfigMap
kubectl get configmap fdb-cluster -n teaelephant -o yaml

# Test connectivity from a debug pod
kubectl run fdb-debug --rm -it --image=foundationdb/foundationdb:7.3.27 -n teaelephant -- bash
# Inside the pod:
# fdbcli --exec status
```

#### "Cannot connect to PostgreSQL"
- DSN is incorrect in secret
- Network policies block access to PostgreSQL
- PostgreSQL is down or credentials are wrong

Debug:
```bash
# Get DSN value
kubectl get secret postgres-dsn -n teaelephant -o jsonpath='{.data.dsn}' | base64 -d

# Test connectivity
kubectl run pg-debug --rm -it --image=postgres:15 -n teaelephant -- \
  psql "$(kubectl get secret postgres-dsn -n teaelephant -o jsonpath='{.data.dsn}' | base64 -d)"
```

#### Job completes but data is missing
Check logs for warnings:
```bash
kubectl logs -n teaelephant -l app=fdb-backfill | grep -i "warning\|error\|skip"
```

Some records may have been skipped due to decode errors. This is normal for corrupted FDB data.

### Re-running the Job

If you need to re-run the backfill:

```bash
# Delete the existing Job
kubectl delete job fdb-backfill -n teaelephant

# Re-apply
kubectl apply -f deployment/fdb-backfill/job.yml
```

The backfill is idempotent - safe to run multiple times.

## Updating the Image

If you need to update the backfill code:

1. Make changes to `cmd/backfill/main.go`
2. Commit and push
3. Wait for GitHub Actions to build new image
4. Delete and re-create the Job:
   ```bash
   kubectl delete job fdb-backfill -n teaelephant
   kubectl apply -f deployment/fdb-backfill/job.yml
   ```

## Production Checklist

Before running in production:

- [ ] FDB cluster connection string is correct in `job.yml`
- [ ] PostgreSQL DSN secret is configured correctly
- [ ] GitHub Container Registry credentials are valid
- [ ] Network connectivity verified (FDB ↔ Job Pod, PG ↔ Job Pod)
- [ ] PostgreSQL schema is applied (`db/schema.sql`)
- [ ] PostgreSQL has sufficient disk space
- [ ] Backup of PostgreSQL database taken
- [ ] Maintenance window scheduled (if required)
- [ ] Monitoring/alerting configured
- [ ] Rollback plan documented

After successful backfill:

- [ ] Data counts verified in PostgreSQL
- [ ] Sample data spot-checked
- [ ] Application tested with new data
- [ ] Job deleted or marked for cleanup
- [ ] Backfill completion documented

## Architecture

```
┌─────────────┐
│   GitHub    │
│  Repository │
└──────┬──────┘
       │ 1. Push code
       ↓
┌─────────────┐
│   GitHub    │
│   Actions   │ 2. Build Docker image with FDB support
└──────┬──────┘
       │ 3. Push image
       ↓
┌─────────────┐
│   ghcr.io   │
│  Container  │
│  Registry   │
└──────┬──────┘
       │ 4. Pull image
       ↓
┌─────────────┐         ┌──────────────┐
│ Kubernetes  │ 5. Read │ FoundationDB │
│     Job     │────────→│   Cluster    │
│  (backfill) │         └──────────────┘
└──────┬──────┘
       │ 6. Write
       ↓
┌─────────────┐
│ PostgreSQL  │
│  Database   │
└─────────────┘
```

## Timeline

Expected duration for various dataset sizes:

| Dataset Size | Approximate Time |
|--------------|------------------|
| Small (<1K records) | 1-5 minutes |
| Medium (1K-10K) | 5-30 minutes |
| Large (>10K) | 30+ minutes |

GitHub Actions build: 5-10 minutes

## Next Steps

After successful backfill:

1. Verify all data is migrated correctly
2. Test the main application with PostgreSQL
3. Monitor for any issues
4. Update migration plan document with completion status
5. Consider decommissioning FDB if no longer needed

## Support

For issues:
- Check Job logs: `kubectl logs -n teaelephant job/fdb-backfill`
- Check GitHub Actions logs: Repository → Actions → Workflow run
- Review troubleshooting section above
- Consult `BACKFILL_GUIDE.md` for additional context
