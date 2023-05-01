package debanator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
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

const FILESPREFIX = "pool"

type hashedFile struct {
	buf        []byte
	md5Hash    control.MD5FileHash
	sha1Hash   control.SHA1FileHash
	sha256Hash control.SHA256FileHash
}

type Repo struct {
	// The prefix to serving http paths to the files provided by this package.
	// This is needed so that we can give absolute paths in Package files.
	filePrefix string
	be         Backend
	packages   []LogicalPackage
	cache      map[string]hashedFile
	release    []byte
}

func NewRepoFromBackend(backend Backend, filePrefix string) Repo {
	return Repo{
		be:         backend,
		cache:      make(map[string]hashedFile),
		filePrefix: filePrefix,
	}
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
		sha256Hash: control.SHA256FileHash{
			FileHash: hashes["sha256"],
		},
		sha1Hash: control.SHA1FileHash{
			FileHash: hashes["sha1"]},
		md5Hash: control.MD5FileHash{FileHash: hashes["md5"]},
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
		Suite:         "stable",
		Architectures: r.GetArches(),
		Components:    "main",
		Date:          time.Now().UTC().Format(dateFmt),
		MD5Sum:        md5s,
		SHA1:          sha1s,
		SHA256:        sha256s,
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
		_, err := w.Write(h.buf)
		if err != nil {
			log.Error(err)
		}
	})
	router.Get(fmt.Sprintf("/%s/*", FILESPREFIX), r.be.ServeFiles(r.filePrefix).ServeHTTP)
	return router
}

func (r *Repo) Populate() error {
	packs := make(map[string]LogicalPackage)
	files := Unwrap(r.be.GetFiles())
	for _, f := range files {
		rd := Unwrap(f.GetReader())
		bi, err := BinaryIndexFromDeb(rd, path.Join(r.filePrefix, FILESPREFIX, f.GetName()))
		if err != nil {
			return fmt.Errorf("processing deb file: %w", err)
		}

		packageName := bi.Package
		if _, ok := packs[packageName]; !ok {
			packs[packageName] = LogicalPackage{
				Name:   packageName,
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
				"arch":    arch,
				"version": bi.Version.String(),
			}).Warn("Duplicate package/arch/version found, ignoring...")
		}
	}
	r.packages = maps.Values(packs)
	return nil
}
