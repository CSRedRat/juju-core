package store

import (
	"encoding/json"
	"io"
	"launchpad.net/juju/go/charm"
	"launchpad.net/juju/go/log"
	"net/http"
	"strings"
)

// Server is an http.Handler that serves the HTTP API of juju
// so that juju clients can retrieve published charms.
type Server struct {
	store *Store
	mux   *http.ServeMux
}

// New returns a new *Server using store.
func NewServer(store *Store) (*Server, error) {
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
	}
	s.mux.HandleFunc("/charm-info", func(w http.ResponseWriter, r *http.Request) {
		s.serveInfo(w, r)
	})
	s.mux.HandleFunc("/charm/", func(w http.ResponseWriter, r *http.Request) {
		s.serveCharm(w, r)
	})
	return s, nil
}

// ServeHTTP serves an http request.
// This method turns *Server into an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type responseCharm struct {
	// These are the fields effectively used by the client as of
	// this writing.
	Revision int      `json:"revision"` // Zero is valid. Can't omitempty.
	Sha256   string   `json:"sha256,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func (s *Server) serveInfo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/charm-info" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	r.ParseForm()
	response := map[string]*responseCharm{}
	for _, url := range r.Form["charms"] {
		r := &responseCharm{}
		response[url] = r
		curl, err := charm.ParseURL(url)
		var info *CharmInfo
		if err == nil {
			info, err = s.store.CharmInfo(curl)
		}
		if err == nil {
			r.Sha256 = info.BundleSha256()
			r.Revision = info.Revision()
		} else {
			r.Errors = append(r.Errors, err.Error())
		}
	}
	data, err := json.Marshal(response)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
	}
	if err != nil {
		log.Printf("can't write content: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) serveCharm(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/charm/") {
		panic("serveCharm: bad url")
	}
	curl, err := charm.ParseURL("cs:" + r.URL.Path[len("/charm/"):])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_, rc, err := s.store.OpenCharm(curl)
	if err == ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("can't open charm %q: %v", curl, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = io.Copy(w, rc)
	if err != nil {
		log.Printf("failed to stream charm %q: %v", curl, err)
	}
}