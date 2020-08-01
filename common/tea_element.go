package common

type Record struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type RecordWithID struct {
	ID string
	*Record
}
