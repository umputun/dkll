package server

// server provides REST api v1.
// As <id> used mongo's _id. Two entry points implemented:
//  1. GET /v1/recs/<id> - returns messages > id
//  2. POST /v1/recs/<id>, body = <Request> - returns messages > id and filtered by Request fields

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

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP, rest.Recoverer(log.Default()))
	router.Use(middleware.Throttle(100), middleware.Timeout(60*time.Second))
	router.Use(rest.AppInfo("dkll", "umputun", s.Version))
	router.Use(rest.Ping, rest.SizeLimit(1024))
	router.Use(logger.New(logger.Log(log.Default()), logger.WithBody, logger.Prefix("[DEBUG]")).Handler)

	router.Route("/v1", func(r chi.Router) {
		r.Post("/find", s.findCtrl)
	})

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
