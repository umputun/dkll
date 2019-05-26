package cmd

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/umputun/dkll/app/client"
	"github.com/umputun/dkll/app/core"
)

type ClientOpts struct {
	API        string   `short:"a" long:"api" env:"DKLL_API" required:"true" description:"API endpoint (client)"`
	Containers []string `short:"c" description:"show container(s) only"`
	Hosts      []string `short:"h" description:"show host(s) only"`
	Excludes   []string `short:"x" description:"exclude container(s)"`
	ShowTs     bool     `short:"m" description:"show syslog timestamp"`
	ShowPid    bool     `short:"p" description:"show pid"`
	ShowSyslog bool     `short:"s" description:"show syslog messages"`
	FollowMode bool     `short:"f" description:"follow mode"`
	TailMode   bool     `short:"t" description:"tail mode"`
	// MaxRecs    int      `short:"n" description:"show N records"`
	Grep   []string `short:"g" description:"grep on entire record"`
	UnGrep []string `short:"G" description:"un-grep on entire record"`

	// TailNum  int    `long:"tail" default:"10" description:"number of initial records"`
	TimeZone string `long:"tz"  default:"Local" description:"time zone"`
}

// ClientCmd wraps c;ient mode
type ClientCmd struct {
	ClientOpts
}

func (c ClientCmd) Run(ctx context.Context) error {
	tz := func() *time.Location {
		if c.TimeZone != "Local" {
			ttz, err := time.LoadLocation(c.TimeZone)
			if err != nil {
				log.Printf("[WARN] can't use TZ %s, %v", c.TimeZone, err)
				return time.Local
			}
			return ttz
		}
		return time.Local
	}

	request := core.Request{
		Limit:      100,
		Containers: c.Containers,
		Hosts:      c.Hosts,
		Excludes:   c.Excludes,
	}

	display := client.DisplayParams{
		ShowPid:    c.ShowPid,
		ShowTs:     c.ShowTs,
		FollowMode: c.FollowMode,
		TailMode:   c.TailMode,
		ShowSyslog: c.ShowSyslog,
		Grep:       c.Grep,
		UnGrep:     c.UnGrep,
		TimeZone:   tz(),
	}

	api := client.APIParams{
		API:            c.API,
		UpdateInterval: time.Second,
		Client:         &http.Client{},
	}
	cli := client.NewCLI(api, display)
	_, err := cli.Activate(ctx, request)
	return err
}
