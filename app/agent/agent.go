package agent

import (
	"context"
	"io"
	"log"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/umputun/dkll/app/agent/logger"
)

// EventLoop reacts on messages from Events, adds+activate LogStreamer as well as stop+remove them.
type EventLoop struct {
	MixOuts         bool
	WriterFactory   func(ctx context.Context, containerName, group string) (logWriter, errWriter io.WriteCloser, err error)
	StreamerFactory func(params *logger.LogStreamerParams) LogStreamer

	LogClient  LogClient
	Events     Eventer
	logStreams map[string]LogStreamer // keep streams per containerID
}

type LogStreamer interface {
	Go(ctx context.Context)
	Close() error
	Name() string
}

// LogClient wraps DockerClient with the minimal interface
type LogClient interface {
	Logs(opts docker.LogsOptions) error
}

// Eventer returns chan with events
type Eventer interface {
	Channel() <-chan Event
}

// Run blocking even loop. Receives events from Eventer and makes new log streams. Also deregister terminated streams.
func (l *EventLoop) Run(ctx context.Context) {
	l.logStreams = map[string]LogStreamer{}

	for {
		select {
		case <-ctx.Done():
			log.Print("[WARN] event loop terminated")
			for _, v := range l.logStreams {
				if err := v.Close(); err != nil {
					log.Printf("[WARN] failed to close %s, %v", v.Name(), err)
					continue
				}
				log.Printf("[INFO] close logger stream for %s", v.Name())
			}
			return
		case event, ok := <-l.Events.Channel():
			if ok {
				log.Printf("[DEBUG] received event %+v", event)
				l.onEvent(ctx, event)
			}
		}
	}
}

func (l *EventLoop) onEvent(ctx context.Context, event Event) {

	if event.Status {
		// new/started container detected
		logWriter, errWriter, err := l.WriterFactory(ctx, event.ContainerName, event.Group)
		if err != nil {
			log.Printf("[WARN] ingore event %+v, %v", event, err)
			return
		}

		ls := l.StreamerFactory(&logger.LogStreamerParams{
			DockerClient:  l.LogClient,
			ContainerID:   event.ContainerID,
			ContainerName: event.ContainerName,
			LogWriter:     logWriter,
			ErrWriter:     errWriter,
		})

		ls.Go(ctx) // activate log stream, stream log content to ls.LogWriter and ls.ErrWriter
		l.logStreams[event.ContainerID] = ls
		log.Printf("[DEBUG] streaming for %d containers", len(l.logStreams))
		return
	}

	// removed/stopped container detected
	ls, ok := l.logStreams[event.ContainerID]
	if !ok {
		log.Printf("[DEBUG] close loggers event %+v for non-mapped container ignored", event)
		return
	}

	log.Printf("[DEBUG] close loggers for %+v", event)
	ls.Close() // close stream and log files
	delete(l.logStreams, event.ContainerID)
	log.Printf("[DEBUG] streaming for %d containers", len(l.logStreams))
}
