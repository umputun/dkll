package server

// server provides REST api v1.
// As <id> used mongo's _id. Two entry points implemented:
//  1. GET /v1/recs/<id> - returns messages > id
//  2. POST /v1/recs/<id>, body = <Request> - returns messages > id and filtered by Request fields

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/rest"

	"github.com/umputun/dkll/app/core"
)

// RestServer basic rest server to access msgs from mongo
type RestServer struct {
	DataService DataService
	Limit       int
	Version     string
}

type DataService interface {
	Find(req core.Request) ([]core.LogEntry, error)
	LastPublished() (entry core.LogEntry, err error)
}

// Run the lister and request's router
func (s *RestServer) Run() {

	log.Print("[INFO] activate rest server on :8080")

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP)
	router.Use(middleware.Throttle(100), middleware.Timeout(60*time.Second))
	router.Use(rest.AppInfo("dkll", "umputun", s.Version))
	router.Use(rest.Ping, rest.SizeLimit(1024))

	router.Route("/v1", func(r chi.Router) {
		r.Post("/find", s.findCtrl)
	})

	err := http.ListenAndServe(":8080", router)
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

	if req.Max == 0 || req.Max > s.Limit {
		req.Max = s.Limit
	}

	recs, err := s.DataService.Find(req)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, rest.JSON{"error": err.Error()})
		return
	}

	render.JSON(w, r, recs)
}
