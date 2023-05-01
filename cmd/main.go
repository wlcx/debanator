package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
	"github.com/wlcx/debanator"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func logMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := responseRecorder{ResponseWriter: w}
		h.ServeHTTP(rec, r)
		log.WithFields(log.Fields{
			"remote": r.RemoteAddr,
			"method": r.Method,
			"url":    r.URL,
			"status": rec.status,
		}).Info("Got request")
	}
}

func main() {
	listenAddr := *flag.String("listen", ":1612", "HTTP listen address")
	debPath := *flag.String("debpath", "debs", "Path to directory containing deb files.")
	flag.Parse()
	log.Info("Starting...")
	var ecKey *crypto.Key
	kb, err := os.ReadFile("privkey.gpg")
	if err != nil {
		log.Infof("Generating new key...")
		ecKey = debanator.Unwrap(crypto.GenerateKey("Debanator", "packager@example.com", "x25519", 0))
		f := debanator.Unwrap(os.Create("privkey.gpg"))
		defer f.Close()
		armored := debanator.Unwrap(ecKey.Armor())
		f.WriteString(armored)
	} else {
		log.Infof("Using existing key...")
		ecKey = debanator.Unwrap(crypto.NewKeyFromArmored(string(kb)))
	}

	signingKeyRing, err := crypto.NewKeyRing(ecKey)
	if err != nil {
		log.Fatal(err)
	}
	be := debanator.NewFileBackend(debPath)
	repo := debanator.NewRepoFromBackend(be, "/dists/stable")
	debanator.Md(repo.Populate())
	if err := repo.GenerateFiles(); err != nil {
		log.Fatal(err)
	}
	log.Infof("Listening on %s", listenAddr)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(repo); err != nil {
			log.Errorf("encoding json: %v", err)
		}
	})
	r.Get("/pubkey.gpg", func(w http.ResponseWriter, r *http.Request) {
		pub, err := ecKey.GetArmoredPublicKey()
		if err != nil {
			log.Fatal(err)
		}
		io.WriteString(w, pub)
	})
	r.Mount("/dists/stable", repo.GetHandler(signingKeyRing))
	http.ListenAndServe(listenAddr, r)
}
