package db

var errClosed = errString("db: closed")

type errString string

func (e errString) Error() string { return string(e) }
