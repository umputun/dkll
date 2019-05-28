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

// LogClient wraps DockerClient with the minimal interface to fetch logs
type LogClient interface {
	Logs(docker.LogsOptions) error
}

// LogStreamer connects and activates container's log stream with io.Writer
type LogStreamer struct {
	*LogStreamerParams
	ctx    context.Context
	cancel context.CancelFunc
}

type LogStreamerParams struct {
	DockerClient  LogClient
	ContainerID   string
	ContainerName string

	LogWriter io.WriteCloser
	ErrWriter io.WriteCloser
}

func (l *LogStreamer) Init(params *LogStreamerParams) {
	l.LogStreamerParams = params
}

// Go activates streamer
func (l *LogStreamer) Go(ctx context.Context) {
	log.Printf("[INFO] start log streamer for %s", l.ContainerName)
	l.ctx, l.cancel = context.WithCancel(ctx)

	go func() {
		logOpts := docker.LogsOptions{
			Container:         l.ContainerID,
			OutputStream:      l.LogWriter, // logs writer for stdout
			ErrorStream:       l.ErrWriter, // err writer for stderr
			Tail:              "10",
			Follow:            true,
			Stdout:            true,
			Stderr:            true,
			InactivityTimeout: time.Hour * 10000,
			Context:           l.ctx,
		}

		var err error
		for {
			err = l.DockerClient.Logs(logOpts) // this is blocking call. Will run until container up and will publish to streams
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
			log.Printf("[WARN] stream from %s terminated with error %v", l.ContainerID, err)
			return
		}
		log.Printf("[INFO] stream from %s terminated", l.ContainerID)
	}()
}

// Close kills streamer
func (l *LogStreamer) Close() (err error) {
	l.cancel()
	l.Wait()

	if e := l.LogWriter.Close(); e != nil {
		return errors.Wrap(e, "failed to close log file (out) writer")
	}

	if l.LogWriter != l.ErrWriter { // don't close err writer in mixed mode, closed already by LogWriter.Close()
		if e := l.ErrWriter.Close(); e != nil {
			return errors.Wrap(e, "failed to close log file (err) writer")
		}
	}
	log.Printf("[DEBUG] close %s", l.ContainerID)
	return nil
}

// Name of the streamed container
func (l *LogStreamer) Name() string {
	return l.ContainerName
}

// Wait for stream completion
func (l *LogStreamer) Wait() {
	<-l.ctx.Done()
}
