# TeaElephant product responsibilities and access boundaries

This document defines the roles of the whole product, not just the image feature. The same policy is maintained in `docs/PRODUCT_BOUNDARIES.md` in the iOS, Editor and backend repositories. Changes to responsibilities or access rules must update all three copies and the affected feature documentation.

## Roles of the applications

| Component | Purpose | Main scenarios |
| --- | --- | --- |
| TeaElephant on iOS | Everyday user client for the tea collection | Read NFC/QR labels; explore jars in AR; view tea information and photos; associate labels with jar data; manage personal collections; get recommendations and Tea of the Day; receive notifications |
| TeaElephantEditor on macOS | Administrator's workspace for maintaining the shared drink catalogue | Create and edit catalogue entries, descriptions and classifications; manage shared photos; request AI descriptions; perform authorized destructive catalogue actions |
| TeaElephantMemory backend | Shared source of data and authority for permissions | Store catalogue, QR records and user collections; validate user/admin identity and collection ownership; serve GraphQL, images, subscriptions, AI and notifications |

Editor is a separate macOS application and repository. The unused macOS target in the iOS repository is not the administrative Editor.

The separation concerns product responsibilities and verified identities, rather than a particular device or operating system. Apple Sign In gives a regular user session. Editor uses separate administrative credentials; the backend validates its signed admin JWT. Opening Editor, owning a phone/Mac or having a QR code does not confer administrative rights. An admin JWT also does not substitute for a user's session when accessing that user's personal collections.

## Three different scopes of data

| Scope | Data | Effect of a change |
| --- | --- | --- |
| Shared catalogue Tea | Drink name, type, description, assigned tags and photo gallery/cover | Seen by every user and every QR record referring to that Tea |
| Individual jar/package | QR record pointing to a Tea, brewing temperature and expiry; NFC labels carry package metadata | Identifies a particular jar/package; two jars of one Tea can have different expiry and brewing data |
| User's personal collection | Named grouping owned by a user, containing references to QR records | Changes that user's grouping and the input to their recommendations |

A QR record is not a separate copy of the Tea catalogue entry. Its Tea photos and description remain shared. Current QR records are addressed globally by QR ID and do not have a `user_id`; their jar-specific attributes must not be described as private, user-owned records.

Deleting a user's own collection, or removing a QR record from it, removes that grouping or membership. It does not delete the shared Tea, its photos or the QR record. This is a normal iOS user action and is distinct from deleting a shared catalogue resource.

## Product rules for shared catalogue changes

The shared library is maintained through the administrator's Editor workflow. Destructive operations on shared resources, including deleting a Tea or one of its images, require a verified administrator. Removing a shared tag or category is also catalogue administration. Uploading content does not give its author the right to delete shared resources.

Mobile users retain explicitly supported contributions: uploading tea photos and selecting a shared cover after signing in. These changes affect the shared Tea and must not be presented as personal photos or QR-specific content. Their permission does not grant general administrative access.

The iOS UI must keep normal collection/jar use separate from administrative catalogue controls. The current live photo gallery hides deletion in normal and AR details; its library also blocks a direct delete or Retry. The backend remains the authority for permission checks, including requests sent without the official UI.

## Current implementation and compatibility

The following is verified from the current source, not a claim about deployment or complete UI coverage. In particular, the existing iOS **Add New Tea** scenario can create a shared Tea through `newTea`; it is broader than choosing a jar's existing Tea.

| Operation | Current API boundary |
| --- | --- |
| Read shared Tea, tags, categories, QR records and images | Public catalogue access |
| Create/update Tea; attach/detach an existing tag (`newTea`, `updateTea`, `addTagToTea`, `deleteTagFromTea`) | Authenticated user or admin; these writes affect shared catalogue data |
| Upload a Tea image or choose its cover | Authenticated user or admin |
| Delete a shared Tea or Tea image | Verified admin |
| Create/update/delete shared tags/categories; change a tag's category | Verified admin; not every schema mutation has an Editor UI |
| Read/create/delete personal collections or change their QR membership | User session and collection ownership; removing membership does not delete catalogue data |
| Collection recommendations and Tea of the Day | User session, with the user's collections/history |
| Generate an AI description | Currently public query/subscription returning text; saving it to a Tea is a separate operation |
| `writeToQR` | No explicit user/admin or ownership check in the current resolver; global QR upsert |
| `registerDeviceToken` | No explicit authentication/ownership check in the current resolver |

The four user-or-admin catalogue mutations above are existing compatibility behavior. They do not establish an admin role in the iOS client, and the documentation must not label all catalogue writes as already admin-only.

The broad role separation therefore has implementation follow-ups:

- Review mobile creation/metadata editing and tag detachment against the shared catalogue policy, including which contributions remain available to users. Tag detachment currently removes a shared association even though deleting the tag itself is admin-only.
- Define and enforce authentication and any ownership model for QR writes. A label scan alone is not an authorization boundary.
- Verify device-token registration against the authenticated user/device relationship.

This documentation records those gaps; changing the API requires a separate implementation and compatibility check. Current user upload/cover rights and personal collection deletion remain as described above.

For image deletion, a valid non-admin receives `FORBIDDEN` and retains their session; an anonymous request receives `UNAUTHENTICATED`. Refusal preserves the gallery and cover. This distinction is verified for image deletion and is not a blanket guarantee about every older administrative endpoint.

## Local demo boundary

The explicit iOS DEBUG **AR Demo** uses real camera/QR tracking with fictional tea entries and photographs stored only on that device. Local photo addition, cover selection and deletion do not change the shared catalogue or grant administrative access. The demo is unavailable in Release. Testing it does not establish live backend permissions or deployment.

## References for maintainers

- iOS: `Shared/Views/EnhancedMenu.swift`, `Shared/TagManagement/`, `Shared/QRManagement/`, `Shared/UserArea/Collections/`, `Shared/UserArea/Auth/`, `Shared/TeaImages/`.
- Editor: `TeaElephantEditor/Managers/DetailManager.swift`, `TagsSelectManager.swift`, `TeaImagesManager.swift`, `AdminAuth.swift`, `AuthInterceptor.swift`, and `README.md`.
- Backend: `pkg/api/v2/graphql/schema.resolvers.go`, `schema.graphql`, `internal/auth/auth.go`, `internal/managers/collection/`, `internal/managers/qr/`, `db/schema.sql`, `db/queries/collections.sql` and `db/queries/devices.sql`.

Permissions were read from the isolated worktree revisions iOS `ad97581`, Editor `bfa72ae`, backend `8f7850f`. Prepared changes and tests are separate from rollout to the live service.
