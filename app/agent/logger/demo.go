package logger

import (
	"context"
	"io"
)

type DemoStreamer struct {
	ContainerName string
	LogWriter     io.WriteCloser
	ErrWriter     io.WriteCloser
}

// go activate demo stream, non blocking
func (l *DemoStreamer) Go(ctx context.Context) {

}

// Close kills streamer
func (l *DemoStreamer) Close() (err error) {
	return nil
}

// Name of the streamed container
func (l *DemoStreamer) Name() string {
	return l.ContainerName
}
