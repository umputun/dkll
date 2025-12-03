package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/go-pkgz/lgr"
	"github.com/jessevdk/go-flags"

	"github.com/umputun/dkll/app/cmd"
)

var opts struct {
	Server cmd.ServerOpts `command:"server" description:"server mode"`
	Client cmd.ClientOpts `command:"client" description:"client mode"`
	Agent  cmd.AgentOpts  `command:"agent" description:"agent mode"`

	Dbg     bool `long:"dbg"  env:"DEBUG" description:"show debug info"`
	Help    bool `long:"help" description:"show help"`
	Version bool `long:"version" description:"show version info"`
}

var revision = "unknown"

type commander interface {
	Run(ctx context.Context) error
}

func main() {

	p := flags.NewParser(&opts, flags.PrintErrors|flags.PassDoubleDash)
	if _, err := p.Parse(); err != nil {
		os.Exit(1)
	}

	if opts.Help {
		p.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	if opts.Version {
		fmt.Printf("dkll %s\n", revision)
		os.Exit(2)
	}

	setupLog(opts.Dbg)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { // catch signal and invoke graceful termination
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Printf("[DEBUG] interrupt signal")
		cancel()
	}()

	var dispatch = map[string]commander{
		"server": cmd.ServerCmd{ServerOpts: opts.Server, Revision: revision},
		"client": cmd.ClientCmd{ClientOpts: opts.Client},
		"agent":  cmd.AgentCmd{AgentOpts: opts.Agent, Revision: revision},
	}

	for name, command := range dispatch {
		if p.Active != nil && p.Find(name) == p.Active {
			if err := command.Run(ctx); err != nil {
				log.Printf("[ERROR] %s failed, %v", name, err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	log.Printf("[ERROR] unknown command")
}

func setupLog(dbg bool) {
	if dbg {
		log.Setup(log.Debug, log.CallerFile, log.CallerFunc, log.Msec, log.LevelBraces)
		return
	}
	log.Setup(log.Msec, log.LevelBraces)
}
