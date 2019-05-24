package client

// implements client.Cli for remote log-client.
// works with logger's REST (server.RestServer)

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/repeater"

	"github.com/umputun/dkll/app/core"
)

// Remote access to dkll rest API
type Remote struct {
	api            string
	updateInterval time.Duration
	request        core.Request
	id             string
	version        int
	client         *http.Client
}

// NewRemote makes new remote client
func NewRemote(api string, interval int, req core.Request, version int) *Remote {
	return &Remote{
		api:            api,
		updateInterval: time.Second * time.Duration(interval),
		request:        req, id: "0", version: version,
		client: &http.Client{Timeout: time.Second * 30},
	}

}

func (c *Remote) getNext(fromID string) (items []core.LogEntry, lastID string, ok bool) {

	uri := fmt.Sprintf("http://%s/v%d/recs/%s", c.api, c.version, c.id)
	body := &bytes.Buffer{}
	if e := json.NewEncoder(body).Encode(c.request); e != nil {
		return items, c.id, false
	}

	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return items, c.id, false
	}
	var resp *http.Response
	err = repeater.NewDefault(10, time.Second).Do(context.Background(), func() error {
		resp, err = c.client.Do(req)
		if err == nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return errors.New("status")
		}
		return nil
	})

	if err != nil {
		log.Printf("[WARN] failed to send request %s", uri)
		return items, c.id, false
	}
	defer func() {
		if e := resp.Body.Close(); e != nil {
			log.Printf("[WARN] failed to close response, %v", e)
		}
	}()

	if err = json.NewDecoder(resp.Body).Decode(&items); err != nil || len(items) == 0 {
		return items, c.id, false
	}

	return items, items[len(items)-1].ID, true
}

func (c *Remote) interval() time.Duration { return c.updateInterval }
func (c *Remote) lastID() string          { return c.id }
func (c *Remote) setLastID(id string)     { c.id = id }
