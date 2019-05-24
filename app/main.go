package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	log "github.com/go-pkgz/lgr"

	"github.com/jessevdk/go-flags"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/umputun/dkll/app/client"
	"github.com/umputun/dkll/app/core"
	"github.com/umputun/dkll/app/server"
)

var opts struct {
	Server struct {
		Mongo        []string `long:"mongo" env:"DKLL_MONGO" description:"mongo host:port"`
		MongoPasswd  string   `long:"mongo-passwd" env:"MONGO_PASSWD" default:"" description:"mongo password"`
		MongoDelay   int      `long:"mongo-delay" env:"MONGO_DELAY" default:"0" description:"mongo initial delay"`
		MongoDB      string   `long:"mongo-db" env:"MONGO_DB" default:"dklogger" description:"mongo database name"`
		FileBackup   string   `long:"backup" default:"" env:"BACK_LOG" description:"backup log file location"`
		EnableMerged bool     `long:"merged"  env:"BACK_MRG" description:"enable merged log file"`
	} `command:"server" description:"server mode"`

	Client struct {
		API        string   `short:"a" long:"api" env:"DKLL_API" required:"true" description:"API endpoint (client)"`
		Containers []string `short:"c" description:"show container(s) only"`
		Hosts      []string `short:"h" description:"show host(s) only"`
		Excludes   []string `short:"x" description:"exclude container(s)"`
		ShowTs     bool     `short:"t" description:"show syslog timestamp"`
		ShowPid    bool     `short:"p" description:"show pid"`
		ShowSyslog bool     `short:"s" description:"show syslog messages"`
		Follow     bool     `short:"f" description:"follow mode"`
		MaxRecs    int      `short:"n" description:"show N records"`
		Grep       []string `short:"g" description:"grep on entire record"`
		UnGrep     []string `short:"G" description:"un-grep on entire record"`
	} `command:"client" description:"client mode"`

	Dbg bool `long:"dbg"  env:"DEBUG" description:"show debug info"`
}

var revision = "unknown"

func main() {

	p := flags.NewParser(&opts, flags.Default)
	if _, e := p.ParseArgs(os.Args[1:]); e != nil {
		os.Exit(1)
	}
	setupLog(opts.Dbg)

	if p.Active != nil && p.Command.Find("server") == p.Active {
		if err := runServer(); err != nil {
			log.Printf("[ERROR] server failed, %v", err)
			os.Exit(1)
		}
	}

	if p.Active != nil && p.Command.Find("client") == p.Active {
		if err := runClient(); err != nil {
			log.Printf("[ERROR] client failed, %v", err)
			os.Exit(1)
		}
	}

}

func runServer() error {
	fmt.Printf("dkll server %s\n", revision)
	log.Printf("[DEBUG] server mode activated %s", revision)

	mergeLogWriter := ioutil.Discard
	containerLogFactory := func(instance, container string) io.Writer { return ioutil.Discard }

	if opts.Server.FileBackup != "" {
		log.Printf("[INFO] backup file %s", opts.Server.FileBackup)
		var err error

		mergeLogWriter = &lumberjack.Logger{
			Filename:   opts.Server.FileBackup,
			MaxSize:    1024 * 10, // megabytes
			MaxBackups: 10,
			MaxAge:     30, // in days
			Compress:   true,
		}

		containerLogFactory = func(instance, container string) io.Writer {
			log.Printf("[DEBUG] make rotated log for %s/%s", instance, container)
			if err := os.MkdirAll(path.Dir(opts.Server.FileBackup)+"/"+instance, 0755); err != nil {
				log.Printf("[WARN] can't make directory %s, %v", path.Dir(opts.Server.FileBackup)+"/"+instance, err)
			}
			fname := fmt.Sprintf("%s/%s/%s.log", path.Dir(opts.Server.FileBackup), instance, container)
			singleWriter := &lumberjack.Logger{
				Filename:   fname,
				MaxSize:    1024 * 10, // megabytes
				MaxBackups: 10,
				MaxAge:     30, // in days
				Compress:   true,
			}
			if err != nil {
				log.Fatalf("[ERROR] failed to open %s, %v", fname, err)
			}
			return singleWriter
		}
	}

	mg, err := server.NewMongo(opts.Server.Mongo, opts.Server.MongoPasswd, opts.Server.MongoDB, opts.Server.MongoDelay)
	if err != nil {
		return err
	}

	restServer := server.RestServer{
		DataService: mg,
		Limit:       100,
		Version:     revision,
	}
	go restServer.Run()

	forwarder := server.Forwarder{
		Publisher:  mg,
		Syslog:     server.Syslog{},
		FileLogger: server.NewFileLogger(containerLogFactory, mergeLogWriter),
	}
	forwarder.Run(context.TODO()) // blocking on forwarder
	return nil
}

func runClient() error {

	var cli client.Cli

	request := core.Request{
		Max:        100,
		Containers: opts.Client.Containers,
		Hosts:      opts.Client.Hosts,
		Excludes:   opts.Client.Excludes,
	}

	cli = client.NewRemote(opts.Client.API, 1, request, 1)

	p := client.Params{
		ShowPid:    opts.Client.ShowPid,
		ShowTs:     opts.Client.ShowTs,
		Follow:     opts.Client.Follow,
		ShowSyslog: opts.Client.ShowSyslog,
		Grep:       opts.Client.Grep,
		UnGrep:     opts.Client.UnGrep,
	}
	return client.Activate(cli, p)
}

func setupLog(dbg bool) {
	if dbg {
		log.Setup(log.Debug, log.CallerFile, log.CallerFunc, log.Msec, log.LevelBraces)
		return
	}
	log.Setup(log.Msec, log.LevelBraces)
}
