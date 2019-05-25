package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type CLI struct {
	DisplayParams
	APIParams

	lastID string
}

type APIParams struct {
	UpdateInterval   time.Duration
	Client           *http.Client
	API              string
	RepeaterStrategy strategy.Interface
}

// Params for Activate call
type DisplayParams struct {
	ShowPid    bool
	ShowTs     bool
	FollowMode bool
	TailMode   bool
	ShowSyslog bool
	Grep       []string
	UnGrep     []string
	TimeZone   *time.Location
	Out        io.Writer
}

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

// Activate showing tail-like, colorized output for passed Cli client
func (c *CLI) Activate(ctx context.Context, request core.Request) (req core.Request, err error) {

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	white := color.New(color.FgWhite).SprintFunc()

	var items []core.LogEntry
	var id string

	if c.TailMode {
		id, err := c.getLastID()
		if err != nil {
			return request, errors.Wrapf(err, "can't get last ID for tail mode")
		}
		request.LastID = id
	}

	for {
		items, id, err = c.getNext(request)
		if err != nil {
			return request, err
		}

		if len(items) == 0 && !c.FollowMode {
			break
		}

		for _, e := range items {
			if !c.ShowSyslog && e.Container == "syslog" {
				continue
			}

			pid := ""
			if c.ShowPid {
				pid = fmt.Sprintf(" [%d]", e.Pid)
			}

			ts := ""
			if c.ShowTs {
				ts = fmt.Sprintf(" - %s", e.Ts.In(c.TimeZone).Format("2006-01-02 15:04:05.999999"))
			}
			line := fmt.Sprintf("%s:%s%s%s - %s\n", red(e.Host), green(e.Container), yellow(ts), yellow(pid), white(e.Msg))

			if len(c.UnGrep) > 0 && contains(line, c.UnGrep) {
				continue
			}

			if len(c.Grep) > 0 && !contains(line, c.Grep) {
				continue
			}
			_, _ = fmt.Fprint(c.Out, line)
		}
		request.LastID = id

		select {
		case <-ctx.Done():
			return request, ctx.Err()
		case <-time.After(c.UpdateInterval):
			continue
		}
	}

	return request, nil
}

func (c *CLI) getLastID() (string, error) {

	var resp *http.Response
	err := repeater.New(c.RepeaterStrategy).Do(context.Background(), func() (e error) {
		resp, e = c.Client.Get(fmt.Sprintf("%s/last", c.API))
		if e != nil {
			return e
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return errors.Errorf("http code %d", resp.StatusCode)
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	lastEntry := core.LogEntry{}
	if e := json.NewDecoder(resp.Body).Decode(&lastEntry); e != nil {
		return "", err
	}
	return lastEntry.ID, nil
}

func (c *CLI) getNext(request core.Request) (items []core.LogEntry, lastID string, err error) {

	uri := fmt.Sprintf("%s/find", c.API)
	body := &bytes.Buffer{}
	if e := json.NewEncoder(body).Encode(request); e != nil {
		return items, "", e
	}
	req, e := http.NewRequest("POST", uri, body)
	if e != nil {
		return items, "", e
	}

	err = repeater.New(c.RepeaterStrategy).Do(context.Background(), func() error {
		var resp *http.Response
		resp, err = c.Client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return errors.New("status")
		}
		defer func() { _ = resp.Body.Close() }()
		return json.NewDecoder(resp.Body).Decode(&items)
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
