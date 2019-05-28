package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/umputun/dkll/app/agent"
	"github.com/umputun/dkll/app/agent/logger"
	"github.com/umputun/dkll/app/agent/syslog"
)

// AgentOpts holds all flags and env for agent mode
type AgentOpts struct {
	DockerHost string `short:"d" long:"docker" env:"DOCKER_HOST" default:"unix:///var/run/docker.sock" description:"docker host"`

	EnableSyslog bool   `long:"syslog" env:"LOG_SYSLOG" description:"enable logging to syslog"`
	SyslogHost   string `long:"syslog-host" env:"SYSLOG_HOST" default:"127.0.0.1:514" description:"syslog host"`
	SyslogPrefix string `long:"syslog-prefix" env:"SYSLOG_PREFIX" default:"docker/" description:"syslog prefix"`

	EnableFiles   bool   `long:"files" env:"LOG_FILES" description:"enable logging to files"`
	MaxFileSize   int    `long:"max-size" env:"MAX_SIZE" default:"10" description:"size of log triggering rotation (MB)"`
	MaxFilesCount int    `long:"max-files" env:"MAX_FILES" default:"5" description:"number of rotated files to retain"`
	MaxFilesAge   int    `long:"max-age" env:"MAX_AGE" default:"30" description:"maximum number of days to retain"`
	MixErr        bool   `long:"mix-err" env:"MIX_ERR" description:"send error to std output log file"`
	FilesLocation string `long:"loc" env:"LOG_FILES_LOC" default:"logs" description:"log files locations"`

	Excludes []string `short:"x" long:"exclude" env:"EXCLUDE" env-delim:"," description:"excluded container names"`
	Includes []string `short:"i" long:"include" env:"INCLUDE" env-delim:"," description:"included container names"`
	ExtJSON  bool     `short:"j" long:"json" env:"JSON" description:"wrap message with JSON envelope"`
	DemoMode bool     `long:"demo" env:"DEMO" description:"demo mode, generates simulated log entries"`
}

// AgentCmd wraps agent mode
type AgentCmd struct {
	AgentOpts
	Revision string
	DemoMode bool
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

	client, err := docker.NewClient(a.DockerHost)
	if err != nil {
		return errors.Wrapf(err, "failed to make docker client %s", err)
	}

	events, err := agent.NewEventNotif(client, a.Excludes, a.Includes)
	if err != nil {
		return errors.Wrap(err, "failed to make event notifier")
	}

	loop := agent.EventLoop{
		LogClient:     client,
		MixOuts:       a.MixErr,
		WriterFactory: a.makeLogWriters,
		Events:        events,
	}

	loop.Run(ctx)
	return nil
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

	if a.EnableSyslog && syslog.IsSupported() {
		syslogWriter, err := syslog.GetWriter(ctx, a.SyslogHost, a.SyslogPrefix, containerName)

		if err == nil {
			logWriters = append(logWriters, syslogWriter)
			errWriters = append(errWriters, syslogWriter)
		} else {
			syslogErr = err
			log.Printf("[WARN] can't connect to syslog, %v", err)
		}
	}

	lw := logger.NewMultiWriterIgnoreErrors(logWriters...)
	ew := logger.NewMultiWriterIgnoreErrors(errWriters...)
	if a.ExtJSON {
		lw = lw.WithExtJSON(containerName, group)
		ew = ew.WithExtJSON(containerName, group)
	}

	if len(logWriters) == 0 {
		return nil, nil, errors.Errorf("all log writers failed. file %+v, syslog %+v", fileErr, syslogErr)
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
