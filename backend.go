package debanator

import (
	"io"
	"net/http"
)

// A backend to search for packages in
type Backend interface {
	GetFiles() ([]DebFile, error)
	ServeFiles(string) http.Handler
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.ReadCloser
}

// An abstract interface for reading a debfile. This could be coming from the local fs,
// a remote webdav share, etc...
type DebFile interface {
	GetReader() (ReaderAtCloser, error)
	GetName() string
}

