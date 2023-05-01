package debanator

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"strconv"
	"strings"

	"pault.ag/go/debian/control"
	"pault.ag/go/debian/deb"
	"pault.ag/go/debian/dependency"
	"pault.ag/go/debian/version"
)

// A group of debs of a package for different arches/version
type LogicalPackage struct {
	Name string
	// arch:version:package
	Arches map[dependency.Arch]map[version.Version]control.BinaryIndex
}

func BinaryIndexFromDeb(r ReaderAtCloser, filePath string) (*control.BinaryIndex, error) {
	debFile, err := deb.Load(r, "fakepath")
	if err != nil {
		return nil, fmt.Errorf("read deb: %w", err)
	}
	md5sum := md5.New()
	sha1sum := sha1.New()
	sha256sum := sha256.New()
	hashWriter := io.MultiWriter(md5sum, sha1sum, sha256sum)
	size, err := io.Copy(hashWriter, r)
	if err != nil {
		return nil, fmt.Errorf("hash file: %w", err)
	}
	bi := control.BinaryIndex{
		Paragraph: control.Paragraph{
			Values: make(map[string]string),
		},
		Package:       debFile.Control.Package,
		Source:        debFile.Control.Source,
		Version:       debFile.Control.Version,
		InstalledSize: fmt.Sprintf("%d", debFile.Control.InstalledSize),
		Size:          strconv.Itoa(int(size)),
		Maintainer:    debFile.Control.Maintainer,
		Architecture:  debFile.Control.Architecture,
		MultiArch:     debFile.Control.MultiArch,
		Description:   debFile.Control.Description,
		Homepage:      debFile.Control.Homepage,
		Section:       debFile.Control.Section,
		// FIXME: gross, make this more centrally managed somehow
		Filename: strings.TrimPrefix(filePath, "/"),
		Priority: debFile.Control.Priority,
		MD5sum:   fmt.Sprintf("%x", md5sum.Sum(nil)),
		SHA1:     fmt.Sprintf("%x", sha1sum.Sum(nil)),
		SHA256:   fmt.Sprintf("%x", sha256sum.Sum(nil)),
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
