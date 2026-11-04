# Personal Tea ratings

A rating is one mutable integer vote (1–5) per authenticated user and shared
Tea. Several jars or collections referencing the same Tea still give one vote.
Ratings do not affect Tea of the Day or purchasing.

The API adds `Tea.rating: TeaRating!`,
`setTeaRating(teaID: ID!, rating: Int!): TeaRating!` and
`clearTeaRating(teaID: ID!): TeaRating!`. `TeaRating` contains `teaID`, nullable
`average`, `count`, nullable `myRating`, and `canRate`. No votes means a null
average and count zero. Anonymous and admin-only sessions see public aggregates,
null `myRating` and false `canRate`. An admin session cannot substitute for a
user identity for rating writes. Clients should query ratings separately so an
old or unavailable rating API does not break ordinary catalogue/QR operations.

Setting or changing a vote requires at least one QR linked to that Tea in any
collection owned by the user. The backend validates ownership and persists the
vote in one transaction. Removing the last membership or deleting a collection
retains the vote, including its contribution to public aggregates. An authenticated
user may clear their own vote after removal, but cannot change or set it again
until eligible. Set/clear return the confirmed aggregate and viewer state.

Values outside 1–5 return `BAD_USER_INPUT`; ineligible setting returns `FORBIDDEN`;
anonymous/admin-only writes return `UNAUTHENTICATED`; a missing Tea returns
`NOT_FOUND`. These errors do not invalidate a valid user session. The database
also checks the integer range and unique `(user_id, tea_id)` key. Updated times
are server generated. The last committed change wins; setting an unchanged value
preserves its timestamp, and clearing an absent vote succeeds without a write.
Clients must reload after an uncertain outcome instead of blindly retrying writes.

## Merge and concurrency

Rating writes share the existing catalogue advisory transaction lock and merge
dependency-table lock order. This serializes catalogue merges and legacy
membership/QR writes with rating eligibility and persistence. Read results use a
single SQL snapshot. No production data or credentials are needed by local tests.

A duplicate Tea merge moves source votes to the target. An overlapping user keeps
the newest `updated_at`, with equal timestamps preserving the target. The chosen
time is unchanged. Vote identities, values and timestamps enter only the opaque
merge-preview fingerprint, so a vote change invalidates an old preview even if
Tea revisions are unchanged. No individual votes or identities are exposed in
merge API responses. Preview and result fields `ratingsMoved` count all source
votes processed, while `ratingConflictsResolved` counts the overlapping subset.
Both are persisted in the receipt, including on replay; historical pre-rating
receipts receive zero counters. Late merge failures roll back votes and receipts.

## Schema upgrade and validation

Fresh databases use `db/schema.sql`. Existing installations require a backup and
explicit application of `db/migrations/20261104_tea_ratings.sql` after both existing
20261012 migrations. The migration is additive and repeatable. Startup does not
run it automatically. This source implementation does not establish rollout.

The integration suite uses `TEA_IMAGES_TEST_PG_DSN` pointed only at disposable
PostgreSQL 18.6. It covers HTTP user/admin/anonymous boundaries, ownership,
range, one vote across duplicate memberships, aggregate changes, retention/clear,
server timestamps/idempotence, repeated/upgrading migrations, merge source/newer
target/timestamp ties, stale previews, historical replays, late rollback and actual
concurrent membership-removal/vote/merge ordering.

The authoritative SDL is `pkg/api/v2/graphql/schema.graphql`; gqlgen outputs come
from the repository-pinned generator. Client introspection may be generated
locally from `generated.NewExecutableSchema` and the gqlgen introspection query
through an in-memory handler; no production schema fetch is needed.

## Coordinated delivery and rollback

Deploy the ratings-aware backend and explicit migration before enabling the new
client feature. Old clients may continue using that backend; new clients report
ratings as unavailable against an older API while retaining baseline browsing.

Once votes exist, a backend binary from before this feature is not a safe merge
rollback target. Its merge deletes the source Tea without transferring votes, and
the rating foreign key then cascades those votes. Retaining the additive schema
does not make that old merge implementation safe. Use a ratings-aware rollback
build or a forward fix; if an older binary must be used during maintenance, keep
catalogue merges disabled and establish a verified data-recovery procedure first.
This limitation does not apply to old clients using the new backend's merge code.

The implementation change and local validation do not execute a production
migration, deploy an image, replace installed clients, or validate physical AR
appearance. Those are separate delivery steps.
