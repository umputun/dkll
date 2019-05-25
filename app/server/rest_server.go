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

// RestServer basic rest server to access msgs from mongo
type RestServer struct {
	Port        int
	DataService DataService
	Limit       int
	Version     string
}

// DataService is accessor to store
type DataService interface {
	Find(req core.Request) ([]core.LogEntry, error)
	LastPublished() (entry core.LogEntry, err error)
}

// Run the lister and request's router
func (s *RestServer) Run(ctx context.Context) {

	log.Print("[INFO] activate rest server on :8080")

	router := s.router()
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	err := srv.ListenAndServe()

	log.Printf("[ERROR] rest server failed, %v", err)
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

// findCtrl gets Request json from POST body.
// containers,hosts and excludes lists supports regexp in mongo format, i.e. /regex/
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

func (s *RestServer) lastCtrl(w http.ResponseWriter, r *http.Request) {
	last, err := s.DataService.LastPublished()
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, rest.JSON{"error": err.Error()})
	}
	render.JSON(w, r, last)
}
