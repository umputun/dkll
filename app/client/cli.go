// Package client implement remote client to get and print records
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/repeater"
	"github.com/go-pkgz/repeater/strategy"
	"github.com/pkg/errors"

	"github.com/umputun/dkll/app/core"
)

// CLI is a client accessing remote dkll rest and printing output.
type CLI struct {
	DisplayParams
	APIParams
}

// APIParams define how and where access remote endpoint
type APIParams struct {
	UpdateInterval   time.Duration
	Client           *http.Client
	API              string
	RepeaterStrategy strategy.Interface
}

// DisplayParams customizes how records will be showed
type DisplayParams struct {
	ShowPid    bool           // include pid
	ShowTs     bool           // include time stamp as "2006-01-02 15:04:05.999999" in given TZ
	FollowMode bool           // follow mode, like -f in grep
	TailMode   bool           // tail mode, like -t in grep
	ShowSyslog bool           // show non-docker messages from syslog, off by default
	Grep       []string       // filter the final output line
	UnGrep     []string       // inverse filter for the final output line
	TimeZone   *time.Location // custom TZ, default is local
	Out        io.Writer      // custom out stream, default is stdout
}

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	white  = color.New(color.FgWhite).SprintFunc()
)

// NewCLI makes cli client
func NewCLI(apiParams APIParams, displayParams DisplayParams) *CLI {
	res := &CLI{DisplayParams: displayParams, APIParams: apiParams}
	if res.TimeZone == nil {
		res.TimeZone = time.Local
	}
	if res.Out == nil {
		res.Out = os.Stdout
	}
	return res
}

// Activate shows tail-like, colorized output. For FollowMode will run endless loop
// doesn't return error on context cancellation, but exit func.
func (c *CLI) Activate(ctx context.Context, request core.Request) (req core.Request, err error) {

	var items []core.LogEntry
	var id string

	if c.TailMode {
		if id, err = c.getLastID(ctx); err != nil {
			return request, errors.Wrapf(c.resetCtxError(err), "can't get last ID for tail mode")
		}
		request.LastID = id
	}

	for {
		if items, id, err = c.getNext(ctx, request); err != nil {
			return request, errors.Wrapf(c.resetCtxError(err), "can't get data for request %+v", request)
		}

		if len(items) == 0 && !c.FollowMode {
			break
		}

		for _, e := range items {
			line, ok := c.makeOutLine(e)
			if !ok {
				continue
			}
			if (len(c.Grep) > 0 && !contains(line, c.Grep)) || (len(c.UnGrep) > 0 && contains(line, c.UnGrep)) {
				continue
			}
			_, _ = fmt.Fprint(c.Out, line)
		}
		request.LastID = id

		select {
		case <-ctx.Done():
			log.Printf("[DEBUG] terminated, %v", ctx.Err())
			return request, nil // don't return error, we don't want error status on ctrl-c in the client
		case <-time.After(c.UpdateInterval):
			continue
		}
	}

	return request, nil
}

func (c *CLI) resetCtxError(err error) error {
	if err == context.Canceled {
		return nil
	}
	return err
}

func (c *CLI) makeOutLine(e core.LogEntry) (string, bool) {
	if !c.ShowSyslog && e.Container == "syslog" {
		return "", false
	}

	pid := ""
	if c.ShowPid {
		pid = fmt.Sprintf(" [%d]", e.Pid)
	}

	ts := ""
	if c.ShowTs {
		ts = fmt.Sprintf(" - %s", e.TS.In(c.TimeZone).Format("2006-01-02 15:04:05.999999"))
	}
	line := fmt.Sprintf("%s:%s%s%s - %s\n", red(e.Host), green(e.Container), yellow(ts), yellow(pid), white(e.Msg))
	return line, true
}

func (c *CLI) getLastID(ctx context.Context) (string, error) {

	lastEntry := core.LogEntry{}
	err := repeater.New(c.RepeaterStrategy).Do(ctx, func() (e error) {
		resp, e := c.Client.Get(fmt.Sprintf("%s/last", c.API))
		if e != nil {
			return errors.Wrapf(e, "can't get last id")
		}
		defer func() { _ = resp.Body.Close() }() // nolint
		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("http code %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(&lastEntry)
	})

	if err != nil {
		return "", err
	}

	return lastEntry.ID, nil
}

func (c *CLI) getNext(ctx context.Context, request core.Request) (items []core.LogEntry, lastID string, err error) {

	items = []core.LogEntry{}

	uri := fmt.Sprintf("%s/find", c.API)
	body := &bytes.Buffer{}
	if e := json.NewEncoder(body).Encode(request); e != nil {
		return items, "", e
	}
	req, e := http.NewRequest("POST", uri, body)
	if e != nil {
		return items, "", e
	}

	err = repeater.New(c.RepeaterStrategy).Do(ctx, func() error {
		var resp *http.Response
		resp, err = c.Client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close() // nolint
			return errors.New("status")
		}
		defer func() { _ = resp.Body.Close() }() // nolint
		body, e := ioutil.ReadAll(resp.Body)
		if e != nil {
			return errors.Wrap(e, "can't read next response body")
		}

		if len(body) == 0 { // empty body shouldn't be an error
			return nil
		}
		return json.Unmarshal(body, &items)
	})

	if err != nil {
		log.Printf("[DEBUG] failed to send request %s", uri)
		return items, "", err
	}

	lastID = request.LastID
	if len(items) > 0 {
		lastID = items[len(items)-1].ID
	}

	return items, lastID, nil
}

func contains(inp string, values []string) bool {
	for _, v := range values {
		if strings.Contains(inp, v) {
			return true
		}
	}
	return false
}
