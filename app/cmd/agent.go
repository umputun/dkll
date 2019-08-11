package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/go-pkgz/lgr"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-syslog"
	"github.com/pkg/errors"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/umputun/dkll/app/agent"
	"github.com/umputun/dkll/app/agent/syslog"
)

// AgentOpts holds all flags and env for agent mode
type AgentOpts struct {
	DockerHost string `short:"d" long:"docker" env:"DOCKER_HOST" default:"unix:///var/run/docker.sock" description:"docker host"`

	EnableSyslog bool   `long:"syslog" env:"LOG_SYSLOG" description:"enable logging to syslog"`
	SyslogHost   string `long:"syslog-host" env:"SYSLOG_HOST" default:"127.0.0.1:514" description:"syslog host"`
	SyslogPrefix string `long:"syslog-prefix" env:"SYSLOG_PREFIX" default:"docker/" description:"syslog prefix"`
	SyslogProt   string `long:"syslog-proto" env:"SYSLOG_PROTO" default:"udp4" description:"syslog protocol"`

	EnableFiles   bool   `long:"files" env:"LOG_FILES" description:"enable logging to files"`
	MaxFileSize   int    `long:"max-size" env:"MAX_SIZE" default:"10" description:"size of log triggering rotation (MB)"`
	MaxFilesCount int    `long:"max-files" env:"MAX_FILES" default:"5" description:"number of rotated files to retain"`
	MaxFilesAge   int    `long:"max-age" env:"MAX_AGE" default:"30" description:"maximum number of days to retain"`
	MixErr        bool   `long:"mix-err" env:"MIX_ERR" description:"send error to std output log file"`
	FilesLocation string `long:"loc" env:"LOG_FILES_LOC" default:"logs" description:"log files locations"`

	Excludes     []string      `short:"x" long:"exclude" env:"EXCLUDE" env-delim:"," description:"excluded container names"`
	Includes     []string      `short:"i" long:"include" env:"INCLUDE" env-delim:"," description:"included container names"`
	ExtJSON      bool          `short:"j" long:"json" env:"JSON" description:"wrap message with JSON envelope"`
	DemoMode     bool          `long:"demo" env:"DEMO" description:"demo mode, generates simulated log entries"`
	DemoRecEvery time.Duration `long:"demo-every" env:"DEMO_EVERY" default:"3s" description:"demo interval"`
}

// AgentCmd wraps agent mode
type AgentCmd struct {
	AgentOpts
	Revision string
}

// Run agent app
func (a AgentCmd) Run(ctx context.Context) error {
	fmt.Printf("dkll agent %s\n", a.Revision)

	if a.Includes != nil && a.Excludes != nil {
		return errors.New("only single option Excludes/Includes are allowed")
	}

	if a.EnableSyslog && !syslog.IsSupported() {
		return errors.New("syslog is not supported on this OS")
	}

	if a.DemoMode {
		log.Printf("[WARN] running agent in demo mode")
	}

	loop, err := a.makeEventLoop(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to make event loop")
	}

	loop.Run(ctx)
	return nil
}

func (a AgentCmd) makeEventLoop(ctx context.Context) (agent.EventLoop, error) {

	if a.DemoMode {
		loop := agent.EventLoop{
			LogEmitter:    &agent.DemoEmitter{Duration: a.DemoRecEvery},
			MixOuts:       a.MixErr,
			WriterFactory: a.makeLogWriters,
			Events:        agent.NewDemoEventNotifier(ctx),
		}
		return loop, nil
	}

	client, err := docker.NewClient(a.DockerHost)
	if err != nil {
		return agent.EventLoop{}, errors.Wrapf(err, "failed to make docker client %s", err)
	}

	events, err := agent.NewEventNotifier(client, a.Excludes, a.Includes)
	if err != nil {
		return agent.EventLoop{}, errors.Wrap(err, "failed to make event notifier")
	}

	return agent.EventLoop{
		LogEmitter:    client,
		MixOuts:       a.MixErr,
		WriterFactory: a.makeLogWriters,
		Events:        events,
	}, nil
}

// makeLogWriters creates io.Writer with rotated out and separate err files. Also adds writer for remote syslog
func (a AgentCmd) makeLogWriters(ctx context.Context, containerName, group string) (logWriter, errWriter io.WriteCloser, err error) {
	log.Printf("[DEBUG] create log writer for %s", strings.TrimPrefix(group+"/"+containerName, "/"))
	if !a.EnableFiles && !a.EnableSyslog {
		return nil, nil, errors.New("either files or syslog has to be enabled")
	}

	var logWriters []io.WriteCloser // collect log writers here, for MultiWriter use
	var errWriters []io.WriteCloser // collect err writers here, for MultiWriter use

	var fileErr, syslogErr error
	if a.EnableFiles {
		logFileWriter, errFileWriter, err := a.makeFileWriters(containerName, group)
		if err != nil {
			fileErr = err
			log.Printf("[WARN] failed to make log file writers for %s, %v", containerName, err)
		} else {
			logWriters = append(logWriters, logFileWriter)
			errWriters = append(errWriters, errFileWriter)
		}
	}

	if a.EnableSyslog {
		syslogWriter, errFileWriter, err := a.makeSyslogWriters(containerName, group)

		if err != nil {
			syslogErr = err
			log.Printf("[WARN] can't connect to syslog, %v", err)
		} else {
			logWriters = append(logWriters, syslogWriter)
			errWriters = append(errWriters, errFileWriter)
		}
	}

	lw := agent.NewMultiWriterIgnoreErrors(logWriters...)
	ew := agent.NewMultiWriterIgnoreErrors(errWriters...)
	if a.ExtJSON {
		lw = lw.WithExtJSON(containerName, group)
		ew = ew.WithExtJSON(containerName, group)
	}

	if len(logWriters) == 0 {
		errs := new(multierror.Error)
		errs = multierror.Append(errs, fileErr)
		errs = multierror.Append(errs, syslogErr)
		return nil, nil, errors.Errorf("all log writers failed, %+v", errs.Error())
	}

	return lw, ew, nil
}

func (a AgentCmd) makeFileWriters(containerName, group string) (logWriter, errWriter io.WriteCloser, err error) {
	logDir := a.FilesLocation
	if group != "" {
		logDir = fmt.Sprintf("%s/%s", a.FilesLocation, group)
	}
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to make logs directory %s", logDir)
	}

	logName := fmt.Sprintf("%s/%s.log", logDir, containerName)
	logFileWriter := &lumberjack.Logger{
		Filename:   logName,
		MaxSize:    a.MaxFileSize, // megabytes
		MaxBackups: a.MaxFilesCount,
		MaxAge:     a.MaxFilesAge, // in days
		Compress:   true,
	}

	// use std writer for errors by default
	errFileWriter := logFileWriter
	errFname := logName

	if !a.MixErr { // if writers not mixed make error writer
		errFname = fmt.Sprintf("%s/%s.err", logDir, containerName)
		errFileWriter = &lumberjack.Logger{
			Filename:   errFname,
			MaxSize:    a.MaxFileSize, // megabytes
			MaxBackups: a.MaxFilesCount,
			MaxAge:     a.MaxFilesAge, // in days
			Compress:   true,
		}
	}

	log.Printf("[INFO] loggers created for %s and %s, max.size=%dM, max.files=%d, max.days=%d",
		logName, errFname, a.MaxFileSize, a.MaxFilesCount, a.MaxFilesAge)

	return logFileWriter, errFileWriter, nil
}

func (a AgentCmd) makeSyslogWriters(containerName, group string) (logWriter, errWriter io.WriteCloser, err error) {
	errs := new(multierror.Error)
	logWriter, err = gsyslog.DialLogger(a.SyslogProt, a.SyslogHost, gsyslog.LOG_INFO, "DAEMON", a.SyslogPrefix+containerName)
	errs = multierror.Append(errs, err)

	errWriter, err = gsyslog.DialLogger(a.SyslogProt, a.SyslogHost, gsyslog.LOG_ERR, "DAEMON", a.SyslogPrefix+containerName)
	errs = multierror.Append(errs, err)
	return logWriter, errWriter, errs.ErrorOrNil()
}
