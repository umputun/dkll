package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/rest"
	"github.com/go-pkgz/rest/logger"

	"github.com/umputun/dkll/app/core"
)

// RestServer is a basic rest server to access msgs from DataService
type RestServer struct {
	Port        int
	DataService DataService
	Limit       int // request limit, i.e. max number of records any single Find can return
	Version     string
}

// DataService is accessor to store
type DataService interface {
	Find(req core.Request) ([]core.LogEntry, error)
	LastPublished() (entry core.LogEntry, err error)
}

// Run the lister and request's router
func (s *RestServer) Run(ctx context.Context) error {
	log.Printf("[INFO] activate rest server on :%d", s.Port)

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

func (s *RestServer) router() chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP, rest.Recoverer(log.Default()))
	router.Use(middleware.Throttle(100), middleware.Timeout(60*time.Second))
	router.Use(rest.AppInfo("dkll", "umputun", s.Version))
	router.Use(rest.Ping, rest.SizeLimit(1024))
	router.Use(logger.New(logger.Log(log.Default()), logger.WithBody, logger.Prefix("[DEBUG]")).Handler)

	router.Route("/v1", func(r chi.Router) {
		r.Post("/find", s.findCtrl)
		r.Get("/last", s.lastCtrl)
	})
	return router
}

// POST /v1/find, body is Request.  Returns list of LogEntry
// containers,hosts and excludes lists support regexp in "//", i.e. /regex/
func (s *RestServer) findCtrl(w http.ResponseWriter, r *http.Request) {

	req := core.Request{}

	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, rest.JSON{"error": err.Error()})
		return
	}

	if req.Limit == 0 || req.Limit > s.Limit {
		req.Limit = s.Limit
	}

	recs, err := s.DataService.Find(req)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, rest.JSON{"error": err.Error()})
		return
	}

	render.JSON(w, r, recs)
}

// GET /v1/last
// Returns latest published LogEntry from DataService
func (s *RestServer) lastCtrl(w http.ResponseWriter, r *http.Request) {
	last, err := s.DataService.LastPublished()
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, rest.JSON{"error": err.Error()})
	}
	render.JSON(w, r, last)
}
