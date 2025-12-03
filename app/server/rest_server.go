package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/rest"
	"github.com/go-pkgz/rest/logger"
	"github.com/go-pkgz/routegroup"

	"github.com/umputun/dkll/app/core"
)

// RestServer is a basic rest server to access msgs from DataService
type RestServer struct {
	Port           int
	DataService    DataService
	Limit          int // request limit, i.e. max number of records any single Find can return
	Version        string
	StreamDuration time.Duration
}

// DataService is accessor to store
type DataService interface {
	Find(req core.Request) ([]core.LogEntry, error)
	LastPublished() (entry core.LogEntry, err error)
}

// Run the lister and request's router
func (s *RestServer) Run(ctx context.Context) error {
	log.Printf("[INFO] activate rest server on :%d", s.Port)

	if s.StreamDuration == 0 {
		s.StreamDuration = 250 * time.Millisecond // default duration for streaming mode. Defines how often it will repeat DataService.Find
	}

	router := s.router()
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		<-ctx.Done()
		if e := srv.Close(); e != nil {
			log.Printf("[WARN] failed to close http server, %v", e)
		}
	}()

	return srv.ListenAndServe()
}

func (s *RestServer) router() http.Handler {
	router := routegroup.New(http.NewServeMux())
	router.Use(rest.Recoverer(log.Default()))
	router.Use(rest.Throttle(100))
	router.Use(rest.AppInfo("dkll", "umputun", s.Version))
	router.Use(rest.Ping, rest.SizeLimit(1024))
	router.Use(logger.New(logger.Log(log.Default()), logger.WithBody, logger.Prefix("[DEBUG]")).Handler)

	router.Mount("/v1").Route(func(r *routegroup.Bundle) {
		r.HandleFunc("POST /find", s.findCtrl)
		r.HandleFunc("POST /stream", s.streamCtrl)
		r.HandleFunc("GET /last", s.lastCtrl)
	})
	return router
}

// POST /v1/find, body is Request.  Returns list of LogEntry
// containers,hosts and excludes lists support regexp in "//", i.e. /regex/
func (s *RestServer) findCtrl(w http.ResponseWriter, r *http.Request) {
	req := core.Request{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rest.SendErrorJSON(w, r, log.Default(), http.StatusBadRequest, err, "failed to decode request")
		return
	}

	if req.Limit == 0 || req.Limit > s.Limit {
		req.Limit = s.Limit
	}

	recs, err := s.DataService.Find(req)
	if err != nil {
		rest.SendErrorJSON(w, r, log.Default(), http.StatusBadRequest, err, "failed to find records")
		return
	}

	rest.RenderJSON(w, recs)
}

// POST /v1/stream?timeout=30s, body is Request.  Stream list of LogEntry, breaks on timeout
// containers,hosts and excludes lists support regexp in "//", i.e. /regex/
func (s *RestServer) streamCtrl(w http.ResponseWriter, r *http.Request) {
	req := core.Request{}

	timeout := 5 * time.Minute // max timeout
	if tm, err := time.ParseDuration(r.URL.Query().Get("timeout")); err == nil && tm <= timeout {
		timeout = tm
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rest.SendErrorJSON(w, r, log.Default(), http.StatusBadRequest, err, "failed to decode request")
		return
	}

	if req.Limit == 0 || req.Limit > s.Limit {
		req.Limit = s.Limit
	}

	st := time.Now()
	for {
		recs, err := s.DataService.Find(req)
		if err != nil {
			rest.SendErrorJSON(w, r, log.Default(), http.StatusBadRequest, err, "failed to find records")
			return
		}
		if len(recs) > 0 {
			for _, rec := range recs {
				if err := json.NewEncoder(w).Encode(rec); err != nil {
					rest.SendErrorJSON(w, r, log.Default(), http.StatusInternalServerError, err, "failed to encode record")
					return
				}
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(s.StreamDuration / 2)
			continue
		}

		if len(recs) == 0 {
			if time.Since(st) > timeout {
				return
			}
			time.Sleep(s.StreamDuration)
			continue
		}
	}
}

// GET /v1/last
// Returns latest published LogEntry from DataService
func (s *RestServer) lastCtrl(w http.ResponseWriter, r *http.Request) {
	last, err := s.DataService.LastPublished()
	if err != nil {
		rest.SendErrorJSON(w, r, log.Default(), http.StatusBadRequest, err, "failed to get last published")
		return
	}
	rest.RenderJSON(w, last)
}
