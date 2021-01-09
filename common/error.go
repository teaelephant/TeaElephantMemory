package common

type Error struct {
	Code int
	Msg  error
}

type HttpError struct {
	Error string `json:"error"`
}
