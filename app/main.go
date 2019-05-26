package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/globalsign/mgo"
	log "github.com/go-pkgz/lgr"
	"github.com/jessevdk/go-flags"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/umputun/dkll/app/client"
	"github.com/umputun/dkll/app/core"
	"github.com/umputun/dkll/app/server"
)

var opts struct {
	Server struct {
		Port               int           `long:"api-port" env:"API_PORT" default:"8080" description:"rest server port"`
		SyslogPort         int           `long:"syslog-port" env:"SYSLOG_PORT" default:"5514" description:"syslog server port"`
		Mongo              []string      `long:"mongo" env:"MONGO" required:"true" env-delim:", " description:"mongo host:port"`
		MongoPasswd        string        `long:"mongo-passwd" env:"MONGO_PASSWD" default:"" description:"mongo password"`
		MongoDelay         time.Duration `long:"mongo-delay" env:"MONGO_DELAY" default:"0s" description:"mongo initial delay"`
		MongoTimeout       time.Duration `long:"mongo-timeout" env:"MONGO_TIMEOUT" default:"5s" description:"mongo timeout"`
		MongoDB            string        `long:"mongo-db" env:"MONGO_DB" default:"dkll" description:"mongo database name"`
		MongoColl          string        `long:"mongo-coll" env:"MONGO_COLL" default:"msgs" description:"mongo collection name"`
		FileBackupLocation string        `long:"backup" default:"" env:"BACK_LOG" description:"backup log files location"`
		EnableMerged       bool          `long:"merged"  env:"BACK_MRG" description:"enable merged log file"`
		LogLimits          struct {
			Container LogLimit `group:"container" namespace:"container" env-namespace:"CONTAINER" description:"container limits"`
			Merged    LogLimit `group:"merged" namespace:"merged" env-namespace:"MERGED" description:"merged log limits"`
		} `group:"limit" namespace:"limit" env-namespace:"LIMIT"`
	} `command:"server" description:"server mode"`

	Client struct {
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
	} `command:"client" description:"client mode"`

	Dbg bool `long:"dbg"  env:"DEBUG" description:"show debug info"`
}

type LogLimit struct {
	MaxSize    int `long:"max-size" env:"MAX_SIZE" default:"100" description:"max log size, in megabytes"`
	MaxBackups int `long:"max-backups" env:"MAX_BACKUPS" default:"10" description:"max number of rotated files"`
	MaxAge     int `long:"max-age" env:"MAX_AGE" default:"30" description:"max age of rotated files"`
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
		if err := runServer(ctx); err != nil {
			log.Printf("[ERROR] server failed, %v", err)
			os.Exit(1)
		}
	}

	if p.Active != nil && p.Command.Find("client") == p.Active {
		if err := runClient(ctx); err != nil {
			log.Printf("[ERROR] client failed, %v", err)
			os.Exit(1)
		}
	}

}

func runServer(ctx context.Context) error {
	fmt.Printf("dkll server %s\n", revision)
	log.Printf("[DEBUG] server mode activated %s", revision)

	// default loggers empty
	mergeLogWriter := ioutil.Discard
	containerLogFactory := func(instance, container string) io.Writer { return ioutil.Discard }

	if opts.Server.FileBackupLocation != "" {
		log.Printf("[INFO] backup files location %s", opts.Server.FileBackupLocation)
		var err error

		if opts.Server.EnableMerged {
			if err = os.MkdirAll(opts.Server.FileBackupLocation, 0755); err != nil {
				return err
			}
			mergeLogWriter = &lumberjack.Logger{
				Filename:   path.Join(opts.Server.FileBackupLocation, "/dkll.log"),
				MaxSize:    opts.Server.LogLimits.Merged.MaxSize,
				MaxBackups: opts.Server.LogLimits.Merged.MaxBackups,
				MaxAge:     opts.Server.LogLimits.Merged.MaxAge,
				Compress:   true,
			}
			log.Printf("[DEBUG] make merged rotated, %+v", mergeLogWriter)
		}

		containerLogFactory = func(instance, container string) io.Writer {
			fname := path.Join(opts.Server.FileBackupLocation, instance, container+".log")
			if err = os.MkdirAll(path.Dir(fname), 0755); err != nil {
				log.Printf("[WARN] can't make directory %s, %v", path.Dir(fname), err)
				return ioutil.Discard
			}
			singleWriter := &lumberjack.Logger{
				Filename:   fname,
				MaxSize:    opts.Server.LogLimits.Container.MaxSize,
				MaxBackups: opts.Server.LogLimits.Container.MaxBackups,
				MaxAge:     opts.Server.LogLimits.Container.MaxAge,
				Compress:   true,
			}
			if err != nil {
				log.Fatalf("[ERROR] failed to open %s, %v", fname, err)
			}
			log.Printf("[DEBUG] make container rotated log for %s/%s, %+v", instance, container, singleWriter)
			return singleWriter
		}
	}

	dial := mgo.DialInfo{
		Addrs:    opts.Server.Mongo,
		AppName:  "dkll",
		Timeout:  opts.Server.MongoTimeout,
		Database: "admin",
	}
	if opts.Server.MongoPasswd != "" {
		dial.Username = "admin"
		dial.Password = opts.Server.MongoPasswd
	}
	mg, err := server.NewMongo(dial, opts.Server.MongoDelay, opts.Server.MongoDB, opts.Server.MongoColl)
	if err != nil {
		return err
	}
	log.Printf("[DEBUG] mongo prepared")

	restServer := server.RestServer{
		Port:        opts.Server.Port,
		DataService: mg,
		Limit:       100,
		Version:     revision,
	}
	go restServer.Run(ctx)

	forwarder := server.Forwarder{
		Publisher:  mg,
		Syslog:     &server.Syslog{Port: opts.Server.SyslogPort},
		FileWriter: server.NewFileLogger(containerLogFactory, mergeLogWriter),
	}

	forwarder.Run(ctx) // blocking on forwarder
	return nil
}

func runClient(ctx context.Context) error {

	tz := func() *time.Location {
		if opts.Client.TimeZone != "Local" {
			ttz, err := time.LoadLocation(opts.Client.TimeZone)
			if err != nil {
				log.Printf("[WARN] can't use TZ %s, %v", opts.Client.TimeZone, err)
				return time.Local
			}
			return ttz
		}
		return time.Local
	}

	request := core.Request{
		Limit:      100,
		Containers: opts.Client.Containers,
		Hosts:      opts.Client.Hosts,
		Excludes:   opts.Client.Excludes,
	}

	display := client.DisplayParams{
		ShowPid:    opts.Client.ShowPid,
		ShowTs:     opts.Client.ShowTs,
		FollowMode: opts.Client.FollowMode,
		TailMode:   opts.Client.TailMode,
		ShowSyslog: opts.Client.ShowSyslog,
		Grep:       opts.Client.Grep,
		UnGrep:     opts.Client.UnGrep,
		TimeZone:   tz(),
	}

	api := client.APIParams{
		API:            opts.Client.API,
		UpdateInterval: time.Second,
		Client:         &http.Client{},
	}
	cli := client.NewCLI(api, display)
	_, err := cli.Activate(ctx, request)
	return err
}

func setupLog(dbg bool) {
	if dbg {
		log.Setup(log.Debug, log.CallerFile, log.CallerFunc, log.Msec, log.LevelBraces)
		return
	}
	log.Setup(log.Msec, log.LevelBraces)
}
