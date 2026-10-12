# Administrative duplicate Tea merge

This source feature merges one reviewed duplicate Tea into one canonical Tea.
It does not automatically identify equal drinks, perform bulk cleanup or delete
unused taxonomy. Matching names are candidates for review, not proof of identity.
The feature is prepared locally; deployment and real-library acceptance remain
separate. The existing mobile jar and personal-collection permissions stay intact.

## Preview and commit

A verified administrator requests `previewTeaMerge(sourceTeaID, targetTeaID)`.
It returns both complete Tea snapshots, both revisions, an opaque `previewToken`,
aggregate transfer counts, blockers and the exact `resultingCoverImageID`.
Personal user IDs, consumption timestamps and collection contents are not returned.

The preview uses these fixed policies:

- `metadataPolicy: target_wins`: preserve the target's name, type and description.
  The source's differing fields are visible for review, and both original metadata
  snapshots are retained with the merge completion. No automatic text concatenation.
- Tags become the union of existing assignments. Tag/category definitions are unchanged.
- QR rows keep their IDs, brewing temperature, expiration and creation time. Only
  their Tea reference changes. Their collection membership and collection owners
  are preserved without rewriting memberships.
- Photographs keep their UUIDs, original bytes, dimensions and timestamps. The
  target cover wins; otherwise the source cover survives. Imported galleries
  without a cover select the oldest image by creation time, then UUID. The preview
  explicitly reports that choice. An empty gallery has a null cover.
- Consumption events retain their user and timestamp. The `consumption_collision`
  blocker refuses any same-user/same-time source and target entries rather than
  collapsing historical events. The `image_limit` blocker refuses more than ten
  combined images rather than silently dropping photographs.

`mergeTeas` requires the two Tea IDs, `expectedSourceRevision`,
`expectedTargetRevision`, `previewToken` and a nonzero UUID `idempotencyKey`.
Clients persist these exact original arguments before dispatch and keep them on
retry. A changed preview must be reviewed again and becomes a new operation.

The server validates both revisions and the preview inside one transaction,
transfers all supported dependencies, removes the duplicate Tea, then commits
its immutable completion receipt and historical counters. A stale revision,
stale preview, blocker or storage failure changes nothing. Only the completed
first application publishes a target-update and source-removal notification.

## Snapshot and concurrency contract

A Tea revision covers metadata, tags, images and cover; it does not cover QR
bindings, collection membership or consumption history. Therefore the preview
hash also includes ordered affected QR data, membership pairs and consumption
state for both Tea IDs. Displayed tag/category metadata and explicit cover choice
are included too. A dependent change invalidates the token even when both Tea
revisions stayed unchanged.

The small household catalogue takes the existing transaction advisory lock,
then `SHARE ROW EXCLUSIVE` table locks for `collection_qr_items`, `consumptions`
and `qr_records`, then ordered parent Tea row locks. This lets legacy dependent
writers finish before parent locks and serializes the short preview/commit
against them. Catalogue writers already take the same advisory lock. No network
request, remote scoring or user confirmation occurs while these locks are held.
The design intentionally favors correctness for a small household catalogue;
these short operations temporarily serialize writes to the dependent tables.

Tea of the Day keeps its selected QR and original recommendation timestamp across
a merge. Cache hits reload that label's current Tea and record no additional
consumption. A fresh selection is refreshed after remote scoring; PostgreSQL
records the current Tea of the selected QR in a short transaction using the same
catalogue lock, then reloads the label for the response/cache. An uncertain history
insert is not retried under a new timestamp.

## Durable completion and read-only recovery

`catalog_operation_receipts.operation` is `mergeTeas`, `teaID` is the canonical
ID and `resourceID` is the removed duplicate ID. The original completion revision
and timestamp remain immutable. `tea_merge_receipt_details` retains both input
revisions, preview token, historical transfer counters and original Tea metadata.
No user identities, history timestamps or photograph bytes are copied to receipts.

Both ordinary replay and the read-only `editorTeaMergeReceipt` require the exact
six original arguments. They verify actor, operation and request hash before
returning the receipt. An absent key returns null from the query; an existing key
for a different operation or arguments returns `IDEMPOTENCY_CONFLICT`. The result
contains historical counters and nullable current `targetTea`. Later target
changes/deletion do not change the receipt, repeat transfers or recreate either
Tea. Old Tea-creation and photo-upload receipts are not retargeted or rewritten;
they remain evidence of the original operation and cannot resurrect the source.

## API errors and prerequisites

- `UNAUTHENTICATED` / `FORBIDDEN`: anonymous/non-admin request.
- `BAD_USER_INPUT`: malformed pair, revision, token or key; identical Tea IDs.
- `NOT_FOUND`: either Tea is absent before an initial merge.
- `CONFLICT`: a Tea revision changed; `currentRevision` identifies its current value.
- `PREVIEW_CONFLICT`: affected state changed since preview.
- `MERGE_BLOCKED`: blockers such as `image_limit` or `consumption_collision`.
- `IDEMPOTENCY_CONFLICT`: an existing key does not authenticate the same operation.
- `UNSUPPORTED`: required migration/capability is unavailable.

The bridge gates this feature on administrative `editorCapabilities.supportsTeaMerge`
in addition to revisions/idempotency. Missing old-API fields must disable merge
writes. Public catalogue, QR and personal collection operations stay compatible.

Apply `db/migrations/20261012_tea_merge.sql` explicitly after a PostgreSQL backup,
the image migration and `20261012_catalog_agent_operations.sql`. The migration
is repeatable, expands the receipt operation constraint and creates only the
merge-detail table. Startup does not apply it. Fresh schema and configured sqlc
outputs carry the same definitions. Restore/undo and bulk merge are outside this
slice; the stored metadata is not a promise of automatic rollback.

## Verification

`pkg/api/v2/graphql/tea_merge_integration_test.go` uses the existing explicit
`TEA_IMAGES_TEST_PG_DSN` disposable-schema fixture. It covers dependency preservation,
exact cover choice and immutable image content, stale dependency fingerprints with
unchanged Tea revisions, both revision conflicts, overflow/history blockers,
late failure rollback, role/capability guards, repeatable migration, old receipts,
strict receipt-query argument authentication, replay after changes/deletion,
concurrent same-key operations and overlap with a legacy QR transaction, and a selected-QR history transaction waiting
for a concurrent jar rebind. It also
checks cached Tea of the Day and a merge while remote scoring is delayed, without
extra consumption on cached reads. No real-library writes are needed for these checks.
