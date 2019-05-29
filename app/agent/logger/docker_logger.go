package logger

import (
	"context"
	"io"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/go-pkgz/lgr"
	"github.com/pkg/errors"
)

// LogStreamer connects and activates container's log stream with io.Writer
type DockerLogStreamer struct {
	params LogStreamerParams

	ctx    context.Context
	cancel func()
	done   chan struct{}
}

type LogStreamerParams struct {
	ID          string
	Name        string
	LogWriter   io.WriteCloser
	ErrWriter   io.WriteCloser
	LogsEmitter LogsEmitter
}

// LogsEmitter wraps DockerClient with the minimal interface to emit logs
type LogsEmitter interface {
	Logs(docker.LogsOptions) error // runs endless loop publishing logs to writers from LogsOptions
}

func (l *DockerLogStreamer) Init(params LogStreamerParams) {
	log.Printf("[DEBUG] initialize DockerLogStreamer with %+v", params)
	l.params = params
	l.done = make(chan struct{})
	l.ctx, l.cancel = context.WithCancel(context.Background())
}

// Run activates streamer, blocking
func (l *DockerLogStreamer) Run() error {
	log.Printf("[INFO] start log streamer for %s", l.Name())

	// can be canceled from outside by Close call from another thread
	defer func() { l.done <- struct{}{} }() // indicate completion

	logOpts := docker.LogsOptions{
		Container:         l.params.ID,        // container ID
		OutputStream:      l.params.LogWriter, // logs writer for stdout
		ErrorStream:       l.params.ErrWriter, // err writer for stderr
		Tail:              "10",
		Follow:            true,
		Stdout:            true,
		Stderr:            true,
		InactivityTimeout: time.Hour * 10000,
		Context:           l.ctx,
	}

	var err error
	for {
		err = l.params.LogsEmitter.Logs(logOpts) // this is blocking call. Will run until container up and will publish to streams
		// workaround https://github.com/moby/moby/issues/35370 with empty log, try read log as empty
		if err != nil && strings.HasPrefix(err.Error(), "error from daemon in stream: Error grabbing logs: EOF") {
			logOpts.Tail = ""
			time.Sleep(1 * time.Second) // prevent busy loop
			log.Print("[DEBUG] retry logger")
			continue
		}
		break
	}

	if err != nil {
		log.Printf("[WARN] stream from %s terminated with error %v", l.params.ID, err)
		return err
	}
	log.Printf("[INFO] stream from %s terminated", l.params.ID)
	return nil
}

// Close kills streamer
func (l *DockerLogStreamer) Close() (err error) {
	l.cancel()
	l.Wait()

	if e := l.params.LogWriter.Close(); e != nil {
		return errors.Wrap(e, "failed to close log file (out) writer")
	}

	if l.params.LogWriter != l.params.ErrWriter { // don't close err writer in mixed mode, closed already by LogWriter.Close()
		if e := l.params.ErrWriter.Close(); e != nil {
			return errors.Wrap(e, "failed to close log file (err) writer")
		}
	}
	log.Printf("[DEBUG] close %s", l.params.ID)
	return nil
}

// Name of the streamed container
func (l *DockerLogStreamer) Name() string {
	return l.params.Name
}

// Wait for stream completion
func (l *DockerLogStreamer) Wait() {
	<-l.done
}
