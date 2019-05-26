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

	"github.com/umputun/dkll/app/agent/discovery"
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
}

// AgentCmd wraps agent mode
type AgentCmd struct {
	AgentOpts
	Revision string
}

// Run agent
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

	events, err := discovery.NewEventNotif(client, a.Excludes, a.Includes)
	if err != nil {
		return errors.Wrap(err, "failed to make event notifier")
	}

	a.runEventLoop(ctx, events, client)
	return nil
}

func (a AgentCmd) runEventLoop(ctx context.Context, events *discovery.EventNotif, client *docker.Client) {
	logStreams := map[string]logger.LogStreamer{}

	procEvent := func(event discovery.Event) {

		if event.Status {
			// new/started container detected
			logWriter, errWriter := a.makeLogWriters(event.ContainerName, event.Group)
			ls := logger.LogStreamer{
				DockerClient:  client,
				ContainerID:   event.ContainerID,
				ContainerName: event.ContainerName,
				LogWriter:     logWriter,
				ErrWriter:     errWriter,
			}
			ls = *ls.Go(ctx)
			logStreams[event.ContainerID] = ls
			log.Printf("[DEBUG] streaming for %d containers", len(logStreams))
			return
		}

		// removed/stopped container detected
		ls, ok := logStreams[event.ContainerID]
		if !ok {
			log.Printf("[DEBUG] close loggers event %+v for non-mapped container ignored", event)
			return
		}

		log.Printf("[DEBUG] close loggers for %+v", event)
		ls.Close()

		if e := ls.LogWriter.Close(); e != nil {
			log.Printf("[WARN] failed to close log writer for %+v, %s", event, e)
		}

		if !a.MixErr { // don't close err writer in mixed mode, closed already by LogWriter.Close()
			if e := ls.ErrWriter.Close(); e != nil {
				log.Printf("[WARN] failed to close err writer for %+v, %s", event, e)
			}
		}
		delete(logStreams, event.ContainerID)
		log.Printf("[DEBUG] streaming for %d containers", len(logStreams))
	}

	for {
		select {
		case <-ctx.Done():
			log.Print("[WARN] event loop terminated")
			for _, v := range logStreams {
				v.Close()
				log.Printf("[INFO] close logger stream for %s", v.ContainerName)
			}
			return
		case event := <-events.Channel():
			log.Printf("[DEBUG] received event %+v", event)
			procEvent(event)
		}
	}

}

// makeLogWriters creates io.Writer with rotated out and separate err files. Also adds writer for remote syslog
func (a AgentCmd) makeLogWriters(containerName, group string) (logWriter, errWriter io.WriteCloser) {
	log.Printf("[DEBUG] create log writer for %s", strings.TrimPrefix(group+"/"+containerName, "/"))
	if !a.EnableFiles && !a.EnableSyslog {
		log.Fatalf("[ERROR] either files or syslog has to be enabled")
	}

	var logWriters []io.WriteCloser // collect log writers here, for MultiWriter use
	var errWriters []io.WriteCloser // collect err writers here, for MultiWriter use

	if a.EnableFiles {

		logDir := a.FilesLocation
		if group != "" {
			logDir = fmt.Sprintf("%s/%s", a.FilesLocation, group)
		}
		if err := os.MkdirAll(logDir, 0750); err != nil {
			log.Fatalf("[ERROR] can't make directory %s, %v", logDir, err)
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

		logWriters = append(logWriters, logFileWriter)
		errWriters = append(errWriters, errFileWriter)
		log.Printf("[INFO] loggers created for %s and %s, max.size=%dM, max.files=%d, max.days=%d",
			logName, errFname, a.MaxFileSize, a.MaxFilesCount, a.MaxFilesAge)
	}

	if a.EnableSyslog && syslog.IsSupported() {
		syslogWriter, err := syslog.GetWriter(a.SyslogHost, a.SyslogPrefix, containerName)

		if err == nil {
			logWriters = append(logWriters, syslogWriter)
			errWriters = append(errWriters, syslogWriter)
		} else {
			log.Printf("[WARN] can't connect to syslog, %v", err)
		}
	}

	lw := logger.NewMultiWriterIgnoreErrors(logWriters...)
	ew := logger.NewMultiWriterIgnoreErrors(errWriters...)
	if a.ExtJSON {
		lw = lw.WithExtJSON(containerName, group)
		ew = ew.WithExtJSON(containerName, group)
	}

	return lw, ew
}
