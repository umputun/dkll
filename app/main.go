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

	Dbg bool `long:"dbg"  env:"DEBUG" description:"show debug info"`
}

var revision = "unknown"

func main() {

	p := flags.NewParser(&opts, flags.Default)
	if _, err := p.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
	}
	setupLog(opts.Dbg)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { // catch signal and invoke graceful termination
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Printf("[WARN] interrupt signal")
		cancel()
	}()

	if p.Active != nil && p.Command.Find("server") == p.Active {
		srv := cmd.ServerCmd{ServerOpts: opts.Server, Revision: revision}
		if err := srv.Run(ctx); err != nil {
			log.Printf("[ERROR] server failed, %v", err)
			os.Exit(1)
		}
	}

	if p.Active != nil && p.Command.Find("client") == p.Active {
		client := cmd.ClientCmd{ClientOpts: opts.Client}
		if err := client.Run(ctx); err != nil {
			log.Printf("[ERROR] client failed, %v", err)
			os.Exit(1)
		}
	}

	if p.Active != nil && p.Command.Find("agent") == p.Active {
		agent := cmd.AgentCmd{AgentOpts: opts.Agent, Revision: revision}
		if err := agent.Run(ctx); err != nil {
			log.Printf("[ERROR] agent failed, %v", err)
			os.Exit(1)
		}
	}
}

func setupLog(dbg bool) {
	if dbg {
		log.Setup(log.Debug, log.CallerFile, log.CallerFunc, log.Msec, log.LevelBraces)
		return
	}
	log.Setup(log.Msec, log.LevelBraces)
}
