package debanator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/dependency"
	"pault.ag/go/debian/hashio"
	"pault.ag/go/debian/version"
)

// A group of debs of a package for different arches/version
type LogicalPackage struct {
	Name string
	// arch:version:package
	Arches map[dependency.Arch]map[version.Version]control.BinaryIndex
}

type hashedFile struct {
	buf []byte
	md5Hash control.MD5FileHash
	sha1Hash control.SHA1FileHash
	sha256Hash control.SHA256FileHash
}

type Repo struct {
	packages []LogicalPackage
	cache map[string]hashedFile
	release []byte
}

func (r *Repo) GetArches() []dependency.Arch {
	arches := make(map[dependency.Arch]struct{})
	for _, lp := range r.packages {
		for arch := range lp.Arches {
			arches[arch] = struct{}{}
		}
	}
	return maps.Keys(arches)
}

// Find the latest versions of all packages for the given arch
func (r *Repo) GetPackagesForArch(a dependency.Arch) []control.BinaryIndex {
	out := []control.BinaryIndex{}
	for _, p := range r.packages {
		if versions, ok := p.Arches[a]; ok {
			var latest version.Version
			for v := range versions {
				if version.Compare(v, latest) > 0 {
					latest = v 
				}
			}
			out = append(out, p.Arches[a][latest])
		}
	}
	return out
}

func (r *Repo) makePackagesFileForArch(arch dependency.Arch) error {
	var b bytes.Buffer
	w, hashers, err := hashio.NewHasherWriters([]string{"md5", "sha256", "sha1"}, &b)
	enc, _ := control.NewEncoder(w)
	for _, d := range r.GetPackagesForArch(arch) {
		if err = enc.Encode(d); err != nil {
			return fmt.Errorf("encoding package %s: %w", d.Package, err)
		}
	}
	fname := fmt.Sprintf("main/binary-%s/Packages", arch)
	hashes := make(map[string]control.FileHash)
	for _, h := range hashers {
		hashes[h.Name()] = control.FileHashFromHasher(fname, *h)
	}
	r.cache[fname] = hashedFile{
		buf: b.Bytes(),
		sha256Hash: control.SHA256FileHash{hashes["sha256"]},
		sha1Hash: control.SHA1FileHash{hashes["sha1"]},
		md5Hash: control.MD5FileHash{hashes["md5"]},
	}
	return nil
}

// Generate and cache all the Package/Repo files
func (r *Repo) GenerateFiles() error {
	for _, arch := range r.GetArches() {
		if err := r.makePackagesFileForArch(arch); err != nil {
			return fmt.Errorf("generating files for arch %s: %w", arch, err)
		}
	}
	r.makeRelease()
	return nil
}

func (r *Repo) makeRelease() {
	var rel bytes.Buffer
		enc, _ := control.NewEncoder(&rel)
		const dateFmt = "Mon, 02 Jan 2006 15:04:05 MST"
		var md5s []control.MD5FileHash
		var sha1s []control.SHA1FileHash
		var sha256s []control.SHA256FileHash
		for _, f := range r.cache {
			md5s = append(md5s, f.md5Hash)
			sha1s = append(sha1s, f.sha1Hash)
			sha256s = append(sha256s, f.sha256Hash)
		}
		if err := enc.Encode(Release{
			Suite: "stable",
			Architectures: r.GetArches(),
			Components: "main",
			Date: time.Now().UTC().Format(dateFmt),
			MD5Sum: md5s,
			SHA1: sha1s,
			SHA256: sha256s,
		}); err != nil {
			log.Fatal(err)
		}
		r.release = rel.Bytes()
		return
	}


// Handle a deb/apt repository http request
func (r *Repo) GetHandler(keyring *crypto.KeyRing) http.Handler {
	router := chi.NewRouter()
	router.Get("/Release", func(w http.ResponseWriter, req *http.Request) {
		if _, err := w.Write(r.release); err != nil {
			log.Fatal(err)
		}
	})
	router.Get("/Release.gpg", func(w http.ResponseWriter, req *http.Request) {
		msg := crypto.NewPlainMessage(r.release)
		sig, err := keyring.SignDetached(msg)
		if err != nil {
			log.Fatal(err)
		}
		sigStr, err := sig.GetArmored()
		if err != nil {
			log.Fatal(err)
		}
		io.WriteString(w, sigStr)
	})
	router.Get("/main/{arch}/Packages", func(w http.ResponseWriter, req *http.Request) {
		h, ok := r.cache[fmt.Sprintf("main/%s/Packages", chi.URLParam(req, "arch"))]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, err := w.Write(h.buf); if err != nil {
			log.Error(err)
		}
	})
	return router
}
