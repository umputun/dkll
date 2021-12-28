package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/mongo/v2"
	"github.com/pkg/errors"
	mdrv "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/umputun/dkll/app/server"
)

// ServerOpts holds all flags and env for server mode
type ServerOpts struct {
	Port               int           `long:"api-port" env:"API_PORT" default:"8080" description:"rest server port"`
	SyslogPort         int           `long:"syslog-port" env:"SYSLOG_PORT" default:"5514" description:"syslog server port"`
	MongoURL           string        `long:"mongo" env:"MONGO" required:"true" description:"mongo URL"`
	MongoTimeout       time.Duration `long:"mongo-timeout" env:"MONGO_TIMEOUT" default:"5s" description:"mongo timeout"`
	MongoMaxSize       int           `long:"mongo-size" env:"MONGO_SIZE" default:"10000000000" description:"max collection size"`
	MongoMaxDocs       int           `long:"mongo-docs" env:"MONGO_DOCS" default:"50000000" description:"max docs in collection"`
	FileBackupLocation string        `long:"backup" default:"" env:"BACK_LOG" description:"backup log files location"`
	EnableMerged       bool          `long:"merged"  env:"BACK_MRG" description:"enable merged log file"`
	LogLimits          struct {
		Container LogLimit `group:"container" namespace:"container" env-namespace:"CONTAINER" description:"container limits"`
		Merged    LogLimit `group:"merged" namespace:"merged" env-namespace:"MERGED" description:"merged log limits"`
	} `group:"limit" namespace:"limit" env-namespace:"LIMIT"`
}

// LogLimit hold params limiting log size and age
type LogLimit struct {
	MaxSize    int `long:"max-size" env:"MAX_SIZE" default:"100" description:"max log size, in megabytes"`
	MaxBackups int `long:"max-backups" env:"MAX_BACKUPS" default:"10" description:"max number of rotated files"`
	MaxAge     int `long:"max-age" env:"MAX_AGE" default:"30" description:"max age of rotated files, days"`
}

// ServerCmd wraps server mode
type ServerCmd struct {
	ServerOpts
	Revision string
}

// Run server
func (s ServerCmd) Run(ctx context.Context) error {
	fmt.Printf("dkll server %s\n", s.Revision)
	log.Printf("[DEBUG] server mode activated %s", s.Revision)

	// default loggers empty
	containerLogFactory, mergeLogWriter, err := s.makeWriters()
	if err != nil {
		return err
	}

	mclient, ex, err := makeMongoClient(s.MongoURL, s.MongoTimeout)
	if err != nil {
		return errors.Wrap(err, "can't make mongo client")
	}
	mgParams := server.MongoParams{DBName: ex["db"].(string), Collection: ex["collection"].(string),
		MaxDocs: s.MongoMaxDocs, MaxCollectionSize: s.MongoMaxSize}
	mg, err := server.NewMongo(mclient, mgParams)
	if err != nil {
		return err
	}
	log.Printf("[DEBUG] mongo prepared")

	restServer := server.RestServer{
		Port:        s.Port,
		DataService: mg,
		Limit:       100,
		Version:     s.Revision,
	}
	go func() {
		if httpErr := restServer.Run(ctx); httpErr != nil {
			log.Printf("[WARN] rest server terminated, %v", httpErr)
		}
	}()

	forwarder := server.Forwarder{
		Publisher:  mg,
		Syslog:     &server.Syslog{Port: s.SyslogPort},
		FileWriter: server.NewFileLogger(containerLogFactory, mergeLogWriter),
	}

	log.Printf("[WARN] forwarder terminated, %v", forwarder.Run(ctx)) // blocking on forwarder

	return nil
}

func makeMongoClient(mongoURL string, timeout time.Duration) (*mdrv.Client, map[string]interface{}, error) {
	log.Printf("[DEBUG] make mongo client for %q", mongoURL)
	if mongoURL == "" {
		return nil, nil, errors.New("no mongo URL provided")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, ex, err := mongo.Connect(ctx, options.Client().
		SetAppName("dkll-server").SetConnectTimeout(timeout), mongoURL, "db", "collection")

	if err != nil {
		return nil, nil, errors.Wrapf(err, "can't connect to mongo %s", mongoURL)
	}
	return client, ex, nil
}

func (s ServerCmd) makeWriters() (wrf server.WritersFactory, mergeLogWriter io.Writer, err error) {

	// default loggers empty
	wrf = func(instance, container string) io.Writer { return io.Discard }
	mergeLogWriter = io.Discard
	if s.FileBackupLocation == "" {
		return wrf, mergeLogWriter, nil
	}

	log.Printf("[INFO] backup files location %s", s.FileBackupLocation)

	if s.EnableMerged {
		if e := os.MkdirAll(s.FileBackupLocation, 0o750); e != nil {
			return wrf, mergeLogWriter, e
		}
		mergeLogWriter = &lumberjack.Logger{
			Filename:   path.Join(s.FileBackupLocation, "/dkll.log"),
			MaxSize:    s.LogLimits.Merged.MaxSize,
			MaxBackups: s.LogLimits.Merged.MaxBackups,
			MaxAge:     s.LogLimits.Merged.MaxAge,
			Compress:   true,
		}
		log.Printf("[DEBUG] make merged rotated, %+v", mergeLogWriter)
	}

	wrf = func(instance, container string) io.Writer {
		fname := path.Join(s.FileBackupLocation, instance, container+".log")
		if err = os.MkdirAll(path.Dir(fname), 0o750); err != nil {
			log.Printf("[WARN] can't make directory %s, %v", path.Dir(fname), err)
			return io.Discard
		}
		singleWriter := &lumberjack.Logger{
			Filename:   fname,
			MaxSize:    s.LogLimits.Container.MaxSize,
			MaxBackups: s.LogLimits.Container.MaxBackups,
			MaxAge:     s.LogLimits.Container.MaxAge,
			Compress:   true,
		}
		if err != nil {
			log.Fatalf("[ERROR] failed to open %s, %v", fname, err)
		}
		log.Printf("[DEBUG] make container rotated log for %s/%s, %+v", instance, container, singleWriter)
		return singleWriter
	}
	return wrf, mergeLogWriter, nil
}
