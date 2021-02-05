package key_builder

const (
	// data
	version     = 'v'
	record      = 'r'
	qr          = 'q'
	tagCategory = 'c'
	tag         = 't'
	// indexes
	recordNameIndex = 'n'
)

var (
	tagCategoryIndexName = []byte{'i', 'c'}
	tagIndexCategoryName = []byte{'i', 't'}
	tagIndexName         = []byte{'i', 'n'}
	tagIndexTea          = []byte{'i', 'a'}
	teaIndexTag          = []byte{'i', 'b'}
)
