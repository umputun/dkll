package client

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	log "github.com/go-pkgz/lgr"

	"github.com/umputun/dkll/app/core"
)

// Cli specifies client interface. No direct use outside of the package
type Cli interface {
	getNext(fromID string) (items []core.LogEntry, lastID string, ok bool)
	lastID() string
	setLastID(id string)
	interval() time.Duration
}

// Params for Activate call
type Params struct {
	ShowPid    bool
	ShowTs     bool
	Follow     bool
	ShowSyslog bool
	Grep       []string
	UnGrep     []string
}

// Activate showing tail-like, colorized output for passed Cli client
func Activate(c Cli, params Params) (err error) {

	defer func(err *error) {
		if rec := recover(); rec != nil {
			log.Printf("%s", rec)
			*err = fmt.Errorf("cli failed for record=%s", rec)
		}
	}(&err)

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	white := color.New(color.FgWhite).SprintFunc()

	for {
		if items, id, ok := c.getNext(c.lastID()); ok {

			for _, e := range items {

				if !params.ShowSyslog && e.Container == "syslog" {
					continue
				}

				pid := ""
				if params.ShowPid {
					pid = fmt.Sprintf(" [%d]", e.Pid)
				}

				ts := ""
				if params.ShowTs {
					lts, err := time.LoadLocation("America/New_York")
					if err != nil {
						log.Fatalf("[ERROR] failed to load TZ location, %v", err)
					}
					ts = fmt.Sprintf(" - %s", e.Ts.In(lts).Format("2006-01-02 15:04:05.999999"))
				}
				line := fmt.Sprintf("%s:%s%s%s - %s\n", red(e.Host), green(e.Container), yellow(ts), yellow(pid), white(e.Msg))

				if len(params.UnGrep) > 0 && contains(line, params.UnGrep) {
					continue
				}

				if len(params.Grep) > 0 && contains(line, params.Grep) {
					continue
				}
				fmt.Print(line)

			}
			c.setLastID(id)
			continue
		}
		if !params.Follow {
			break
		}
		time.Sleep(c.interval())
	}
	return nil
}

func contains(inp string, values []string) bool {
	for _, v := range values {
		if v == inp {
			return true
		}
	}
	return false
}
