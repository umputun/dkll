package agent

import (
	"context"
	"io"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/go-pkgz/lgr"
)

// EventLoop reacts on messages from Events, adds+activate LogStreamer as well as stop+remove them.
type EventLoop struct {
	MixOuts       bool
	WriterFactory func(ctx context.Context, containerName, group string) (logWriter, errWriter io.WriteCloser, err error)

	LogEmitter LogEmitter
	Events     Eventer
	logStreams map[string]LogStreamer // keep streams per containerID
}

// LogStreamer defines runnable interface created on event
type LogStreamer interface {
	Run() error
	Close(ctx context.Context) error
	Name() string
}

// LogEmitter wraps DockerClient with the minimal interface
type LogEmitter interface {
	Logs(opts docker.LogsOptions) error
}

// Eventer returns chan with events
type Eventer interface {
	Channel() <-chan Event
}

// Run blocking even loop. Receives events from Eventer and makes new log streams.
// Also deregister terminated streams.
func (l *EventLoop) Run(ctx context.Context) {
	l.logStreams = map[string]LogStreamer{}

	for {
		select {
		case <-ctx.Done():
			log.Print("[WARN] event loop terminated")

			closeCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			for _, v := range l.logStreams {

				if err := v.Close(closeCtx); err != nil {
					log.Printf("[WARN] failed to close %s, %v", v.Name(), err)
					continue
				}
				log.Printf("[INFO] close logger stream for %s", v.Name())
			}
			cancel()
			return
		case event, ok := <-l.Events.Channel():
			if ok {
				log.Printf("[DEBUG] received event %+v", event)
				l.onEvent(ctx, event)
			}
		}
	}
}

// onEvent dispatches add/remove container events from docker
func (l *EventLoop) onEvent(ctx context.Context, event Event) {

	if event.Status {
		// new/started container detected
		logWriter, errWriter, err := l.WriterFactory(ctx, event.ContainerName, event.Group)
		if err != nil {
			log.Printf("[WARN] ignore event %+v, %v", event, err)
			return
		}

		if _, found := l.logStreams[event.ContainerID]; found {
			log.Printf("[WARN] ignore dbl-start %+v, %v", event, err)
			return
		}

		ls := NewContainerLogStreamer(ContainerStreamerParams{
			LogsEmitter: l.LogEmitter,
			ID:          event.ContainerID,
			Name:        event.ContainerName,
			LogWriter:   logWriter,
			ErrWriter:   errWriter,
		})

		// activate log stream, stream log content to ls.LogWriter and ls.ErrWriter
		go func() {
			if e := ls.Run(); e != nil {
				log.Printf("[WARN] streamer terminated for %s, %v", ls.Name(), e)
			}
		}()

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

	// close stream and log files
	log.Printf("[DEBUG] close loggers for %+v", event)
	closeCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if e := ls.Close(closeCtx); e != nil {
		log.Printf("[WARN] close error for %s, %v", event.ContainerName, e)
	}
	delete(l.logStreams, event.ContainerID)

	log.Printf("[DEBUG] streaming for %d containers", len(l.logStreams))
}
