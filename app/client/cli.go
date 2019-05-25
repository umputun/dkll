package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fatih/color"
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
	Follow     bool
	Tail       bool
	ShowSyslog bool
	Grep       []string
	UnGrep     []string
}

func NewCLI(apiParams APIParams, displayParams DisplayParams) *CLI {
	return &CLI{DisplayParams: displayParams, APIParams: apiParams}
}

// Activate showing tail-like, colorized output for passed Cli client
func (c *CLI) Activate(request core.Request) (err error) {

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	white := color.New(color.FgWhite).SprintFunc()

	var items []core.LogEntry
	var ok bool
	var id string

	if c.Tail {
		id, err := c.getLastID()
		if err == nil {
			request.LastID = id
			log.Printf("[DEBUG] tail mode from %s", id)
		}
	}

	for {
		if items, id, ok = c.getNext(request); ok {
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
					// lts, err := time.LoadLocation("America/New_York")
					// if err != nil {
					// 	log.Fatalf("[ERROR] failed to load TZ location, %v", err)
					// }
					ts = fmt.Sprintf(" - %s", e.Ts.In(time.Local).Format("2006-01-02 15:04:05.999999"))
				}
				line := fmt.Sprintf("%s:%s%s%s - %s\n", red(e.Host), green(e.Container), yellow(ts), yellow(pid), white(e.Msg))

				if len(c.UnGrep) > 0 && contains(line, c.UnGrep) {
					continue
				}

				if len(c.Grep) > 0 && contains(line, c.Grep) {
					continue
				}
				fmt.Print(line)
			}
			request.LastID = id
		}
		if !c.Follow {
			break
		}
		time.Sleep(c.UpdateInterval)
	}
	return nil
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

func (c *CLI) getNext(request core.Request) (items []core.LogEntry, lastID string, ok bool) {

	uri := fmt.Sprintf("%s/find", c.API)
	body := &bytes.Buffer{}
	if e := json.NewEncoder(body).Encode(request); e != nil {
		return items, "", false
	}
	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return items, "", false
	}
	var resp *http.Response
	err = repeater.New(c.RepeaterStrategy).Do(context.Background(), func() error {
		resp, err = c.Client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return errors.New("status")
		}
		return nil
	})

	if err != nil {
		log.Printf("[DEBUG] failed to send request %s", uri)
		return items, "", false
	}
	defer func() {
		if e := resp.Body.Close(); e != nil {
			log.Printf("[DEBUG] failed to close response, %v", e)
		}
	}()

	if err = json.NewDecoder(resp.Body).Decode(&items); err != nil || len(items) == 0 {
		return items, "", false
	}

	return items, items[len(items)-1].ID, true
}

func contains(inp string, values []string) bool {
	for _, v := range values {
		if v == inp {
			return true
		}
	}
	return false
}
