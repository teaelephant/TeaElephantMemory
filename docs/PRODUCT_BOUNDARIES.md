# TeaElephant product responsibilities and access boundaries

This document defines the roles of the whole product, not just the image feature. The same policy is maintained in `docs/PRODUCT_BOUNDARIES.md` in the iOS, Editor and backend repositories. Changes to responsibilities or access rules must update all three copies and the affected feature documentation.

## Roles of the applications

| Component | Purpose | Main scenarios |
| --- | --- | --- |
| TeaElephant on iOS | Everyday user client for the tea collection | Read NFC/QR labels; explore jars in AR; view tea information and photos; create new catalogue teas; associate labels with jar data; manage personal collections; get recommendations and Tea of the Day; receive notifications |
| TeaElephantEditor on macOS | Administrator's workspace for maintaining the shared drink catalogue | Create and edit catalogue entries, descriptions and classifications; manage shared photos; request AI descriptions; perform authorized destructive catalogue actions |
| TeaElephantMemory backend | Shared source of data and authority for permissions | Store catalogue, QR records and user collections; validate user/admin identity and collection ownership; serve GraphQL, images, subscriptions, AI and notifications |

Editor is a separate macOS application and repository. The unused macOS target in the iOS repository is not the administrative Editor.

The separation concerns product responsibilities and verified identities, rather than a particular device or operating system. Apple Sign In gives a regular user session. Editor uses separate administrative credentials; the backend validates its signed admin JWT. Opening Editor, owning a phone/Mac or having a QR code does not confer administrative rights. An admin JWT also does not substitute for a user's session when accessing that user's personal collections.

## Three different scopes of data

| Scope | Data | Effect of a change |
| --- | --- | --- |
| Shared catalogue Tea | Drink name, type, description, assigned tags and photo gallery/cover | Seen by every user and every QR record referring to that Tea |
| Individual jar/package | QR record pointing to a Tea, brewing temperature and expiry; NFC labels carry package metadata | Identifies a particular jar/package; two jars of one Tea can have different expiry and brewing data |
| Collection | Grouping of references to QR labels; personal or shareable with family in the product model | Changes the grouping and recommendation input, without changing catalogue administration rights |

A QR is a label/reference to a Tea, outside the shared catalogue. Changing which existing Tea a QR points to changes the drink in that jar; it does not create or edit a shared Tea. Access to this jar action is not conditional on owning a collection or adding the QR to one. A QR record is not a separate copy of the Tea catalogue entry. Its Tea photos and description remain shared. Current QR records are addressed globally by QR ID and do not have a `user_id`; their jar-specific attributes must not be described as private, user-owned records.

Collections may be shared with family in the product model, independently of QR label access. The current API preserves owner-scoped personal collections; family invitations and participant permissions are a separate roadmap item and are not implemented by this role-alignment change.

Deleting a user's own collection, or removing a QR record from it, removes that grouping or membership. It does not delete the shared Tea, its photos or the QR record. This is a normal iOS user action and is distinct from deleting a shared catalogue resource.

## Product rules for shared catalogue changes

The shared library is maintained through the administrator's Editor workflow. Destructive operations on shared resources, including deleting a Tea or one of its images, require a verified administrator. Removing a shared tag or category is also catalogue administration. Uploading content does not give its author the right to delete shared resources.

Mobile users retain non-destructive contributions after signing in: creating a new Tea with optional existing tags chosen as part of that creation, uploading tea photos and selecting a shared cover. Tea creation always inserts a new record; it must not overwrite an existing Tea with the same name. These changes affect the shared Tea and must not be presented as personal photos or QR-specific content. Their permission does not grant general administrative access.

The iOS UI must keep normal collection/jar use separate from administrative catalogue controls. The current live photo gallery hides deletion in normal and AR details; its library also blocks a direct delete or Retry. The backend remains the authority for permission checks, including requests sent without the official UI.

## Current implementation and compatibility

The following describes the prepared source implementation and its verified API contract. It does not claim deployment or physical-device verification.

| Operation | API boundary |
| --- | --- |
| Read shared Tea, tags, categories, QR records and images | Public catalogue access |
| Create Tea (`newTea`) with optional initial `tagIDs` | Authenticated user or admin; new Tea and existing tag associations are created atomically |
| Update an existing Tea or add/remove its tags (`updateTea`, `addTagToTea`, `deleteTagFromTea`) | Verified admin; users cannot change tags after creation, including for a Tea they created |
| Upload a Tea image or choose its cover | Authenticated user or admin; supported shared contributions |
| Delete a shared Tea or Tea image | Verified admin |
| Create/update/delete shared tags/categories; change a tag's category | Verified admin; not every schema mutation has an Editor UI |
| Read/create/delete personal collections or change their QR membership | User session and collection ownership; removing membership does not delete catalogue data |
| Collection recommendations and Tea of the Day | User session, with the user's collections/history |
| Generate an AI description | Public query/subscription returning text; using it when creating a new Tea is allowed to a user/admin; replacing an existing Tea description requires admin |
| `writeToQR` | Authenticated user or admin; global QR upsert, with no exclusive user ownership |
| `registerDeviceToken` | User session; the device must already be bound to that same user |

The iOS jar flow can choose an existing shared Tea or deliberately create a new one with an ordinary signed-in user session, then write the label's brewing/expiry metadata. It does not overwrite an existing shared Tea or detach its tags. Existing `newTea` user access is preserved; existing-Tea updates and tag detachment are catalogue administration.

For operations requiring catalogue administration, a valid non-admin receives `FORBIDDEN` and retains their session; an anonymous request receives `UNAUTHENTICATED`. Guards run before catalogue storage or update notifications. User Tea creation with optional initial tags and photo upload/cover selection remain allowed. Personal collection removal/deletion remains a normal user action, with the existing ownership checks.

QR authentication does not invent an owner for existing labels. Two signed-in users can continue working with the same household QR ID, including changing its Tea reference to another existing catalogue entry even without a personal collection. Jar updates retain global upsert semantics. Any authenticated user or admin can therefore overwrite that QR's existing jar metadata; authentication is not a household-membership check. Anonymous reads remain available, anonymous writes are refused. Adding exclusive QR ownership or a family membership model would require a separate design and migration; this change adds neither.

Notification-token registration passes the authenticated user ID through to an atomic database update matching both `devices.id` and `devices.user_id`. A missing or foreign device returns the existing device-not-found error (GraphQL numeric code `5`) without changing its token or owner. Without a mobile user identity, including an admin-only context, the existing user-not-found error (numeric code `3`) is returned. These are separate from catalogue `FORBIDDEN` and do not invalidate a valid user session.

`authApple` still binds the device after Apple authentication. Existing binding uses `ON CONFLICT (id) DO NOTHING`; it does not transfer a device to a different account. The iOS client must retain an APNs token received before sign-in and register it once a user session is available. Account switching cannot silently bypass the server's device ownership check.

The newTea mutation adds the optional/defaulted `tagIDs: [ID!]! = []` argument; callers omitting it retain the existing creation behaviour. Existing field names, argument/result types and Editor operations remain compatible. No QR owner column, device-owner reassignment or data migration is introduced. Initial tag IDs must refer to existing tags; duplicate IDs are deduplicated. An invalid/missing tag refuses the whole creation transaction without a partial Tea, tag link or creation event. Once created, adding or removing tags requires administrator credentials. Creating tags/categories also remains administrative.

The shared schema descriptions document the role checks; the backend resolvers and database predicate enforce them.

## Validation and rollout boundary

Backend regression tests cover valid user/admin JWTs over HTTP and direct resolver calls, refused catalogue changes, shared QR updates by two users, personal collection isolation/removal, and owner-only token updates. Configured generators are rerun, including SQL helpers; the legacy SQL generation/runtime adapter split remains documented in the backend storage guide (`docs/POSTGRES_STORAGE.md`).

Prepared code, local verification, installation on a device and server rollout are distinct states. The stricter catalogue/QR/device API and the matching mobile flow must be delivered together. The existing tea-image feature still requires its separate image-table migration before live image use. Permission alignment itself needs no new database migration.

## Local demo boundary

The explicit iOS DEBUG **AR Demo** uses real camera/QR tracking with fictional tea entries and photographs stored only on that device. Local photo addition, cover selection and deletion do not change the shared catalogue or grant administrative access. The demo is unavailable in Release. Testing it does not establish live backend permissions or deployment.

## References for maintainers

- iOS: `Shared/Views/EnhancedMenu.swift`, `Shared/TagManagement/`, `Shared/QRManagement/`, `Shared/UserArea/Collections/`, `Shared/UserArea/Auth/`, `Shared/TeaImages/`.
- Editor: `TeaElephantEditor/Managers/DetailManager.swift`, `TagsSelectManager.swift`, `TeaImagesManager.swift`, `AdminAuth.swift`, `AuthInterceptor.swift`, and `README.md`.
- Backend: `pkg/api/v2/graphql/schema.resolvers.go`, `schema.graphql`, `internal/auth/auth.go`, `internal/managers/collection/`, `internal/managers/qr/`, `db/schema.sql`, `db/queries/collections.sql` and `db/queries/devices.sql`.

The initial permission inventory was recorded from iOS `ad97581`, Editor `bfa72ae`, backend `8f7850f`. The role-alignment source changes replace the previously documented catalogue, QR and device gaps. Prepared changes and tests are separate from rollout to the live service.
