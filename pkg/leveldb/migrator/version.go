package migrator

import "context"

const (
	currentVersion = 1
)

type versionDB interface {
	WriteVersion(ctx context.Context, version uint32) error
	GetVersion(ctx context.Context) (uint32, error)
}
