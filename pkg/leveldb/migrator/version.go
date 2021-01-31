package migrator

const (
	currentVersion = 1
)

type versionDB interface {
	WriteVersion(version uint32) error
	GetVersion() (uint32, error)
}
