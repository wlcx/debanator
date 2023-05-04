package debanator

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// A deb file existing on the local filesystem
type fsDebFile struct {
	path string
}

func (f fsDebFile) GetReader() (ReaderAtCloser, error) {
	return os.Open(f.path)
}

func (f fsDebFile) GetName() string {
	_, name := filepath.Split(f.path)
	return name
}

type FileBackend struct {
	path string
}

func NewFileBackend(path string) FileBackend {
	return FileBackend{path}
}

func (fb FileBackend) ServeFiles(prefix string) http.Handler {
	return http.StripPrefix(path.Join(prefix, "pool"), http.FileServer(http.Dir(fb.path)))
}

func (fb FileBackend) GetFiles() ([]DebFile, error) {
	var debs []DebFile
	fs.WalkDir(os.DirFS(fb.path), ".", func(dirpath string, dir fs.DirEntry, err error) error {
		if err != nil {
			log.WithFields(log.Fields{
				"path":  dirpath,
				"error": err,
			}).Warn("Error scanning for debs")
			return nil
		}
		if !dir.IsDir() && strings.HasSuffix(dir.Name(), ".deb") {
			debs = append(debs, DebFile(fsDebFile{
				filepath.Join(fb.path, dirpath),
			}))
		}
		return nil
	})
	log.Infof("got files: %v", debs)
	return debs, nil
}
