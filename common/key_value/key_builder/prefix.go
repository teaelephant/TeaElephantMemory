package key_builder

const (
	// data
	version      = 'v'
	record       = 'r'
	qr           = 'q'
	tagCategory  = 'c'
	tag          = 't'
	collection   = 'l'
	user         = 'u'
	notification = 'o'
	device       = 'd'
	// indexes
	recordNameIndex = 'n'
)

var (
	tagCategoryIndexName   = []byte{'i', 'c'}
	tagIndexCategoryName   = []byte{'i', 't'}
	tagIndexName           = []byte{'i', 'n'}
	tagIndexTea            = []byte{'i', 'a'}
	teaIndexTag            = []byte{'i', 'b'}
	collectionIndexTea     = []byte{'i', 'l'}
	userIndexAppleID       = []byte{'i', 'i'}
	userIndexNotifications = []byte{'i', 'u'}
	userIndexDevices       = []byte{'i', 'd'}
	userIndexConsumption   = []byte{'i', 'x'}
)
