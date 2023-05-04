package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
	"github.com/wlcx/debanator"
	"tailscale.com/tsnet"
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
	listenAddr := flag.String("listen", ":1612", "HTTP listen address")
	debPath := flag.String("debpath", "debs", "Path to directory containing deb files.")
	httpUser := flag.String("httpuser", "debanator", "Username for HTTP basic auth")
	httpPass := flag.String("httppass", "", "Enable HTTP basic auth with this password")
	showVersion := flag.Bool("version", false, "Show version")
	tailscaleHostname := flag.String("tailscalehostname", "debanator", "Hostname for this instance on tailscale")
	flag.Parse()
	if *showVersion {
		fmt.Printf("debanator %s, (git#%s)\n", debanator.Version, debanator.Commit)
		os.Exit(0)
	}

	log.WithFields(log.Fields{
		"version": debanator.Version,
		"commit":  debanator.Commit,
	}).Info("Starting debanator...")
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
	be := debanator.NewFileBackend(*debPath)
	repo := debanator.NewRepoFromBackend(be, "/dists/stable")
	debanator.Md(repo.Populate())
	if err := repo.GenerateFiles(); err != nil {
		log.Fatal(err)
	}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	if *httpPass != "" {
		log.Infof("HTTP basic auth enabled")
		// We're using auth
		authMap := map[string]string{
			*httpUser: *httpPass,
		}
		r.Use(middleware.BasicAuth("", authMap))
	}
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

	var listen func(string, string) (net.Listener, error)
	if _, gotTsKey := os.LookupEnv("TS_AUTHKEY"); gotTsKey {
		log.Infof("Tailscale mode enabled. Using hostname %s", *tailscaleHostname)
		s := &tsnet.Server{
			Hostname: *tailscaleHostname,
			// tsnet is a bit logspammy so send it all to debug
			Logf: func(fmt string, args ...any) {
				log.Debugf("[tsnet] "+fmt, args...)
			},
		}
		defer s.Close()
		listen = s.Listen
	} else {
		listen = net.Listen
	}

	listener := debanator.Unwrap(listen("tcp", *listenAddr))
	defer listener.Close()

	log.Infof("Listening on %s", *listenAddr)
	http.Serve(listener, r)
}
