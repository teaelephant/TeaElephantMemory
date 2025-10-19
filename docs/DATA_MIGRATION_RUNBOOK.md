# Data Migration Runbook (FoundationDB → PostgreSQL)

This runbook is a concise, execution‑oriented plan for migrating TeaElephant data from FoundationDB (FDB) to PostgreSQL (PG). It complements the detailed design in docs/FDB_TO_POSTGRES_MIGRATION_PLAN.md and assumes sqlc scaffolding and the backfill tool are present (they are in this repo).

Audience: SREs and backend engineers executing the migration with minimal downtime.

---

## 0) Preconditions and Ownership
- DRI: Migration lead (backend)
- Approvers: Tech lead, SRE lead
- Change window: Low-traffic period (announce 48h in advance)
- Backups:
  - Snapshot FDB cluster
  - Take PG base backup/snapshot (if already provisioned)

---

## 1) Environment and Tooling
- Services available:
  - FoundationDB (clients installed on host; DATABASEPATH accessible)
  - PostgreSQL (reachable via PG_DSN; migrations applied)
- Repo prerequisites (already in repo):
  - sqlc config: db/sqlc.yaml
  - schema: db/schema.sql
  - queries: db/queries/*.sql
  - backfill tool: cmd/backfill
- Local toolchain:
  - Go 1.25
  - sqlc (optional; for regen)

Commands:
- Build server: go build -v -o ./bin/server ./cmd/server
- Build backfill: go build -v -o ./bin/backfill ./cmd/backfill
- Run backfill: PG_DSN=postgres://… ./bin/backfill

---

## 2) Configuration Flags
Add or confirm runtime flags/vars (as per migration plan; wire-up may vary by environment):
- DATABASE_BACKEND=foundationdb|postgres (initially foundationdb)
- PG_DSN=postgres://user:pass@host:5432/db?sslmode=disable
- Feature flags (if implemented):
  - FF_PG_DUAL_WRITE=true|false (start false → true before backfill catch-up)
  - FF_PG_READ_PERCENT=0..100 (per method; start 0)

---

## 3) Migration Steps (Checklist)

Step A — Prepare (Pre-Backfill)
- [ ] Deploy PG and run schema migrations from db/schema.sql (or your migration tool)
- [ ] Verify connectivity: psql "$PG_DSN" -c "SELECT 1"
- [ ] Dry-run backfill in staging with prod-like data volume
- [ ] Enable metrics/dashboards for backfill rate, error count, and server READ_SOURCE

Step B — Enable Dual-Write (Shadow PG)
- [ ] Deploy server with FF_PG_DUAL_WRITE=true (primary: FDB, shadow: PG)
- [ ] Validate writes succeed in FDB; monitor shadow write errors/outbox if present

Step C — Initial Full Backfill
- [ ] Execute domain order respecting FK dependencies:
  1. users
  2. tag_categories, tags
  3. teas
  4. tea_tags
  5. qr_records
  6. collections
  7. collection_qr_items
  8. devices
  9. notifications
  10. consumptions
- [ ] Command example:
  - Build: go build -v -o ./bin/backfill ./cmd/backfill
  - Run: PG_DSN=postgres://user:pass@host:5432/tea?sslmode=disable ./bin/backfill
- [ ] For large tables, prefer batched INSERT or COPY (future optimization)

Step D — Verification (After Full Backfill)
- [ ] Row count checks (examples):
  - SELECT COUNT(*) FROM teas;
  - SELECT COUNT(*) FROM tags; SELECT COUNT(*) FROM tea_tags;
  - SELECT COUNT(*) FROM qr_records; SELECT COUNT(*) FROM collections; SELECT COUNT(*) FROM collection_qr_items;
  - SELECT COUNT(*) FROM devices; SELECT COUNT(*) FROM notifications; SELECT COUNT(*) FROM consumptions;
- [ ] Spot-check samples (join semantics):
  - Verify a collection’s items match between FDB-backed API and PG queries
  - Verify prefix search on teas works via ILIKE 'abc%'
- [ ] Optional checksum sampling per table (concatenate business fields, hash, compare samples)

Step E — Incremental Catch-up
- [ ] Keep dual-write on; re-run backfill in incremental mode (only new/changed) if implemented
- [ ] Confirm zero lag in outbox/shadow error queue (if used)

Step F — Read Cutover (Gradual)
- [ ] Increase FF_PG_READ_PERCENT per endpoint/domain: 10% → 50% → 100%
- [ ] Monitor latencies, error rates, and correctness (support dashboards, logs)

Step G — Write Cutover
- [ ] Switch primary writes to PG (turn off FDB writes for cut-over domains)
- [ ] Keep dual-write for a grace period with PG→FDB shadow or disable entirely per policy
- [ ] Final verification: production parity spot-checks, user actions sanity

Step H — Decommissioning
- [ ] Disable dual-write
- [ ] Archive FDB snapshot; update runbooks and infra manifests
- [ ] Remove FDB code paths once stable (or guard behind build tags)

---

## 4) Backfill Details & Notes
- The provided backfill tool (cmd/backfill) currently includes:
  - Teas: streaming from FDB (Records prefix), inserting into PG with ON CONFLICT upsert
  - Skeleton for consumptions (needs per-user iteration implementation)
- Extend it to cover all domains; ensure operations are idempotent (ON CONFLICT DO NOTHING/UPDATE)
- Use encoder package to decode FDB values; use key_builder helpers to parse composite keys (e.g., ParseConsumptionKey)
- Recommended batch size: 1k–10k rows; consider COPY for large tables

---

## 5) Rollback Plan
At any point:
- Set FF_PG_READ_PERCENT=0 (reads return to FDB)
- Keep dual-write or disable shadow writes if causing pressure
- If write cutover already occurred, revert primary to FDB and queue PG reconciliation jobs
- Data safety: PG can be truncated per table and re-backfilled; keep FDB snapshot till final sign-off

---

## 6) Operational Metrics and Alerts
- Backfill: rows/sec, error count, retries, ETA
- Dual-write: shadow write failures, outbox depth, retry age
- API: read latency p95/p99 by backend, error rate, query counts
- DB: PG connections, slow queries, index scans vs seq scans, bloat

---

## 7) Acceptance Criteria (Success Conditions)
- Data parity validated (counts and sample checksums) for all tables
- 100% reads served from PG with stable latency and error rates
- Writes cut over to PG; dual-write disabled
- FDB decommissioned after retention period; documentation updated

---

## 8) Quick Commands Reference
- Build tools: go build -v -o ./bin/backfill ./cmd/backfill
- Run backfill: PG_DSN=postgres://user:pass@host:5432/tea?sslmode=disable ./bin/backfill
- Verify search: SELECT id,name FROM teas WHERE lower(name) LIKE lower('earl%') ORDER BY name LIMIT 20;
- Check recent consumptions: SELECT * FROM consumptions WHERE user_id=$1 ORDER BY ts DESC LIMIT 50;

For deeper context and schema specifics, see docs/FDB_TO_POSTGRES_MIGRATION_PLAN.md.


---

## 9) Kubernetes Backfill Job — Re-run guidance and immutable template
- Kubernetes Jobs have an immutable Pod template. If you try to re-apply a Job manifest with the same name after changing image/env, you will see: `The Job "backfill" is invalid: spec.template: ... field is immutable`.
- To avoid this, the repo’s manifest uses `metadata.generateName: backfill-`. Important: `kubectl apply` cannot be used with `generateName`; use `kubectl create -f` to create a new Job each time.

Common commands:
- Create a new Job run:
  ```
  kubectl -n teaelephant create -f deployment/backfill-job.yml
  kubectl -n teaelephant get jobs -l app=backfill
  ```
- Tail logs of the latest pod:
  ```
  kubectl -n teaelephant get pods -l job-name=$(kubectl -n teaelephant get jobs -l app=backfill -o jsonpath='{.items[-1].metadata.name}')
  # pick the pod name
  kubectl -n teaelephant logs <pod-name>
  ```
- If you previously created a fixed-name Job (metadata.name: backfill), delete it before re-creating:
  ```
  kubectl -n teaelephant delete job/backfill || true
  kubectl -n teaelephant create -f deployment/backfill-job.yml
  ```
- Cleanup completed Jobs and their pods by label:
  ```
  kubectl -n teaelephant delete job -l app=backfill --field-selector=status.successful==1 || true
  ```
