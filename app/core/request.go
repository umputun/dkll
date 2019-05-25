package core

import (
	"fmt"
	"strings"
	"time"
)

// Request with filters and params for store queries
// Every filter is optional. If not defined means "any"
type Request struct {
	LastID     string    `json:"id"`
	Limit      int       `json:"max"`                  // max size of response, i.e. number of messages one request can return
	Hosts      []string  `json:"hosts,omitempty"`      // list of hosts, can be exact match or regex in from of /regex/
	Containers []string  `json:"containers,omitempty"` // list of containers, can be regex as well
	Excludes   []string  `json:"excludes,omitempty"`   // list of excluded containers, can be regex
	FromTS     time.Time `json:"from_ts,omitempty"`
	ToTS       time.Time `json:"to_ts,omitempty"`
}

func (r Request) String() string {
	var elems []string
	elems = append(elems, fmt.Sprintf("hosts=%s, containers=%s, excludes=%s, max=%d", r.Hosts, r.Containers, r.Excludes, r.Limit))
	if !r.FromTS.IsZero() {
		elems = append(elems, "from="+r.FromTS.Format(time.RFC3339))
	}
	if !r.ToTS.IsZero() {
		elems = append(elems, "to="+r.ToTS.Format(time.RFC3339))
	}
	elems = append(elems, "last-id="+r.LastID)
	return strings.Join(elems, ", ")
}
