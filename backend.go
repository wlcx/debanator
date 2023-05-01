package debanator

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/deb"
	"pault.ag/go/debian/dependency"
	"pault.ag/go/debian/version"

	"golang.org/x/exp/maps"
)

// A backend to search for packages in
type Backend interface {
	GetPackages() 
}


type FileBackend struct {
	path string
}


func NewFileBackend(path string) FileBackend {
	return FileBackend{path}
}

func BinaryIndexFromDeb(p string, basePath string) (*control.BinaryIndex, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()
	debFile, err := deb.Load(f, p)
	if err != nil {
		return nil, fmt.Errorf("read deb: %w", err)
	}
	md5sum := md5.New()
	sha1sum := sha1.New()
	sha256sum := sha256.New()
	hashWriter := io.MultiWriter(md5sum, sha1sum, sha256sum)
	size, err := io.Copy(hashWriter, f)
	if err != nil {
		return nil, fmt.Errorf("hash file: %w", err)
	}
	bi := control.BinaryIndex{
		Paragraph: control.Paragraph{
			Values: make(map[string]string),
		},
		Package: debFile.Control.Package,
		Source: debFile.Control.Source,
		Version: debFile.Control.Version,
		InstalledSize: fmt.Sprintf("%d", debFile.Control.InstalledSize),
		Size: strconv.Itoa(int(size)),
		Maintainer: debFile.Control.Maintainer,
		Architecture: debFile.Control.Architecture,
		MultiArch: debFile.Control.MultiArch,
		Description: debFile.Control.Description,
		Homepage: debFile.Control.Homepage,
		Section: debFile.Control.Section,
		// FIXME: gross, make this more centrally managed somehow
		Filename: path.Join("pool/main", strings.TrimPrefix(p, basePath)),
		Priority: debFile.Control.Priority,
		MD5sum: fmt.Sprintf("%x", md5sum.Sum(nil)),
		SHA1: fmt.Sprintf("%x", sha1sum.Sum(nil)),
		SHA256: fmt.Sprintf("%x", sha256sum.Sum(nil)),
	}
	if debFile.Control.Depends.String() != "" {
		bi.Paragraph.Set("Depends", debFile.Control.Depends.String())
	}
	if debFile.Control.Recommends.String() != "" {
		bi.Paragraph.Set("Recommends", debFile.Control.Recommends.String())
	}
	if debFile.Control.Suggests.String() != "" {
		bi.Paragraph.Set("Suggests", debFile.Control.Suggests.String())
	}
	if debFile.Control.Breaks.String() != "" {
		bi.Paragraph.Set("Breaks", debFile.Control.Breaks.String())
	}
	if debFile.Control.Replaces.String() != "" {
		bi.Paragraph.Set("Replaces", debFile.Control.Replaces.String())
	}
	if debFile.Control.BuiltUsing.String() != "" {
		bi.Paragraph.Set("BuiltUsing", debFile.Control.BuiltUsing.String())
	}
	return &bi, nil
}

func ScanDebs(debpath string) Repo {
	var debs []string
	fs.WalkDir(os.DirFS(debpath), ".", func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			log.WithFields(log.Fields{
				"path":  path,
				"error": err,
			}).Warn("Error scanning for debs")
			return nil
		}
		if !dir.IsDir() && strings.HasSuffix(dir.Name(), ".deb"){
			debs = append(debs, path)
		}
		return nil
	})
	packs := make(map[string]LogicalPackage)
	for _, d := range debs {
		p := path.Join(debpath, d)
		bi, err  := BinaryIndexFromDeb(p, debpath)
		if err != nil {
			log.WithFields(log.Fields{
				"path": p,
				"error": err,
			}).Error("Error processing deb file")
			continue
		}

		packageName := bi.Package
		if _, ok := packs[packageName]; !ok {
			packs[packageName] = LogicalPackage{
				Name: packageName,
				Arches: make(map[dependency.Arch]map[version.Version]control.BinaryIndex),
			}
		}
		pack := packs[packageName]
		if _, ok := pack.Arches[bi.Architecture]; !ok {
			pack.Arches[bi.Architecture] = make(map[version.Version]control.BinaryIndex)
		}
		arch := pack.Arches[bi.Architecture]
		if _, ok := arch[bi.Version]; !ok {
			arch[bi.Version] = *bi
		} else {
			log.WithFields(log.Fields{
				"package": packageName,
				"arch": arch,
				"version": bi.Version.String(),
			}).Warn("Duplicate package/arch/version found, ignoring...")
		}
	}

	return Repo{
		packages: maps.Values(packs),
		cache: make(map[string]hashedFile),
	}
}
