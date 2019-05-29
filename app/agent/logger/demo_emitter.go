package logger

import (
	"fmt"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

// DemoEmitter is a emitter for fake logs, no docker involved
type DemoEmitter struct {
	Duration time.Duration
}

// Logs generates random log messages
func (d *DemoEmitter) Logs(o docker.LogsOptions) error {
	var n int64
	for {
		select {
		case <-o.Context.Done():
			return o.Context.Err()
		case <-time.After(d.Duration):
			_, _ = o.OutputStream.Write([]byte(fmt.Sprintf("demo message %d\n", n)))
			n++
		}
	}
}
