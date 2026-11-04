# Editor agent catalogue API

The Editor's local agent bridge uses the existing catalogue GraphQL boundary. It
never gives an agent database access or an unrestricted GraphQL executor. The
backend changes in this document are prepared source changes, not a live rollout.

## Compatibility and permission checks

`Tea`, `Tag` and `TagCategory` expose `revision: String!`. Existing catalogue
mutations accept optional `expectedRevision: String`; omitting it preserves older
clients' wire contract. `newTea` and `addTeaImage` also accept an optional
`idempotencyKey: String`, a nonzero UUID. `addTeaImage` accepts both arguments.
Existing administrative, mobile contribution, QR and collection guards remain.
Authorization is checked before mutation argument validation or receipt replay.

The bridge requires an authenticated administrative response to:

```graphql
query {
  editorCapabilities {
    protocolVersion
    supportsRevisions
    supportsIdempotency
    supportsTeaMerge
  }
}
```

Protocol version is `"1"`. Revision/idempotency flags are true only when the required
migration objects are present and revision triggers are enabled. The additional
`supportsTeaMerge` flag also requires the explicit merge migration. Missing fields,
a failed capability request, a different protocol version or false flags must
keep bridge writes disabled. A supported API does not mean the bridge is enabled:
local Editor authorization and its tool allowlist are independent requirements.

Capabilities and receipt queries require a verified administrator. A mobile user
gets `FORBIDDEN`; an anonymous caller gets `UNAUTHENTICATED`. Administrative
credentials still do not grant a user's personal collection session. The agent
bridge exposes no QR, personal collection or device-token tools.

## Atomic revisions

A revision is opaque: clients echo it, never increment it or assume a particular
initial value. Name, type and description are read together with the Tea revision
from one row snapshot. Initial tag assignments can advance a new Tea's revision
within its creation transaction.

A Tea revision covers its owned metadata, assigned tag IDs, gallery membership
and cover. Tag and category revisions cover their own fields. Renaming a shared
tag does not overwrite Tea-owned data and does not fan out updates to every Tea.
Removing a tag/category does remove assignments and advances affected Tea
revisions through the same database transaction.

All catalogue state-changing writes, including legacy callers omitting
`expectedRevision` and mobile image/cover contributions, advance revisions.
Triggers enforce this at the database boundary. Image cover changes or multiple
initial tag assignments can advance a counter more than once; its arithmetic is
not part of the API contract. QR and collection membership changes do not advance
Tea metadata revisions.

The mutation checks the expected revision while holding the entity row lock and
writes in the same transaction. A stale revision produces `CONFLICT` with `currentRevision` in its error
extensions, without
side effects. The small household catalogue serializes catalogue transactions
with a transaction advisory lock before individual row locks, so taxonomy
cascades and Tea/image operations acquire locks in a consistent order. Category
removal and its tag removals commit or roll back together. The first MCP tool set does not expose Tea/tag/category deletion or bulk cleanup.
The separately gated duplicate-merge extension is described in [Tea merge](TEA_MERGE.md);
existing administrative deletion semantics remain separate.

Malformed revisions/keys produce `BAD_USER_INPUT`. An absent expected revision
keeps legacy last-writer-wins behavior. The bridge always supplies one on edits
and treats a conflict as a reason to read fresh data and review a new change.

## Durable creation/upload receipts

Editor persists an operation UUID before dispatching a creation or upload and
retains it across connection errors, application restart and JWT refresh. The
server commits a receipt, normalized request hash and resource IDs in the same
transaction as the new Tea/photo. Failed operations leave no receipt or partial
resource. Concurrent identical requests commit only one resource.

Tea creation normalizes and deduplicates initial tag IDs for the request hash.
Image hashing uses validated decoded bytes, Tea ID and the supplied revision
precondition. A retry repeats the original arguments. Reusing a key with different
arguments or an operation from a different actor returns
`IDEMPOTENCY_CONFLICT`. Actor identity is the mobile user's UUID or the verified
administrative identity; it is not a short-lived JWT JTI.

An authorized replay returns the existing Tea's current data without repeating
the creation/upload. Replay is checked before a stale image revision guard, so a
completed upload can be recovered after the Tea changes. Creation replay does
not publish a second creation event. No image bytes, JWTs or private keys are
stored in operation receipts.

An administrator can recover a committed operation with:

```graphql
query Receipt($key: String!) {
  editorOperationReceipt(idempotencyKey: $key) {
    idempotencyKey
    operation
    teaID
    resourceID
    resultRevision
    completedAt
  }
}
```

`operation` is `newTea`, `addTeaImage` or `mergeTeas`; `resourceID` is respectively
the created Tea ID, image ID or removed duplicate Tea ID. Unknown keys return null. A receipt proves that creation
committed, not that its resource is still unchanged. Editor reads the current Tea
after recovery. Receipts survive resource deletion; retrying an old operation
never recreates a deliberately removed resource. Receipts are retained without
automatic expiry in this version.

The local Editor journal and readback provide visible history. They do not
constitute a guaranteed rollback system. [Duplicate merge](TEA_MERGE.md) now has a separate guarded domain transaction
that preserves QR IDs, collection references, photos, classifications and history.
Bulk cleanup and automatic rollback remain outside this slice. Cascading deletion
must not substitute for a merge.

## Migration and local verification

Apply `db/migrations/20261012_catalog_agent_operations.sql` explicitly after a
backup and the existing tea-image migration. It is additive and repeatable; it
adds revision columns, a receipt table and revision triggers without deleting
catalogue, QR or collection data. Fresh databases include the same definitions
in `db/schema.sql`. Server startup does not apply migrations.

Configured gqlgen and sqlc generation remain required. Runtime catalogue methods
use scoped direct SQL because the historical `pkg/pgstore` versus
`db/pkg/pgstore` generation split still exists; legacy generated runtime files are
not hand-edited. Integration tests use only `TEA_IMAGES_TEST_PG_DSN` and an
isolated schema in an explicitly disposable PostgreSQL fixture. They cover
concurrent compare-and-swap, concurrent creation replay, image replay with an old
revision, actor isolation, authorization before receipt access, taxonomy failure
rollback, legacy revision invalidation and migration preserving jar/collection
references and images. Source checks and these tests are separate from server
rollout and end-to-end Editor acceptance.
