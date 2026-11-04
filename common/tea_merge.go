package common

import (
	"errors"

	"github.com/google/uuid"
)

// Tea merge errors distinguish invalid inputs, stale previews, blockers and prerequisites.
var (
	ErrInvalidTeaMerge         = errors.New("Tea merge requires distinct nonzero Tea IDs, positive revisions, a preview token and an idempotency key")
	ErrTeaMergePreviewConflict = errors.New("Tea merge preview changed; review a new preview before committing")
	ErrTeaMergeBlocked         = errors.New("Tea merge is blocked; no records were changed")
	ErrTeaMergeUnavailable     = errors.New("Tea merge migration is not installed")
)

// Tea merge policies and blocker codes form the reviewed transfer contract.
const (
	TeaMergeImageLimit           = "image_limit"
	TeaMergeConsumptionCollision = "consumption_collision"
	TeaMergeMetadataPolicy       = "target_wins"
	TeaMergeCoverPolicy          = "target_else_source"
)

// TeaMergeTag keeps category metadata in the same preview snapshot as its tag.
type TeaMergeTag struct {
	Tag      *Tag
	Category *TagCategory
}

// TeaMergeSnapshot excludes image bytes and personal identities from API responses.
type TeaMergeSnapshot struct {
	Tea    *Tea
	Tags   []TeaMergeTag
	Images []TeaImage
}

// TeaMergeCounts contains historical aggregate transfers without personal identifiers.
type TeaMergeCounts struct {
	QRCount                   int `json:"qr_count"`
	CollectionMembershipCount int `json:"collection_membership_count"`
	TagsAdded                 int `json:"tags_added"`
	ImagesMoved               int `json:"images_moved"`
	ConsumptionsMoved         int `json:"consumptions_moved"`
}

// TeaMergePreview binds reviewed snapshots and dependencies to an opaque token.
type TeaMergePreview struct {
	Source                TeaMergeSnapshot
	Target                TeaMergeSnapshot
	Token                 string
	Counts                TeaMergeCounts
	ResultingImageCount   int
	Blockers              []string
	ResultingCoverImageID *uuid.UUID
}

// TeaMergeCommand contains the exact original request authenticated on replay.
type TeaMergeCommand struct {
	SourceTeaID            uuid.UUID
	TargetTeaID            uuid.UUID
	ExpectedSourceRevision string
	ExpectedTargetRevision string
	PreviewToken           string
}

// TeaMergeResult retains historical completion even after either Tea changes or is deleted.
type TeaMergeResult struct {
	Receipt        *CatalogueReceipt
	Target         *Tea
	TargetSnapshot *TeaMergeSnapshot
	Counts         TeaMergeCounts
	Replayed       bool
}

// TeaMergeBlockedError reports all conditions preventing an atomic transfer.
type TeaMergeBlockedError struct{ Blockers []string }

func (*TeaMergeBlockedError) Error() string { return ErrTeaMergeBlocked.Error() }
func (*TeaMergeBlockedError) Unwrap() error { return ErrTeaMergeBlocked }
