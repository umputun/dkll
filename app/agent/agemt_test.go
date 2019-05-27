package agent

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"gotest.tools/assert"

	"github.com/umputun/dkll/app/agent/discovery"
)

func TestAgent(t *testing.T) {

	lwr, ewr := mockWriter{}, mockWriter{}
	el := EventLoop{
		WriterFactory: func(containerName, group string) (logWriter, errWriter io.WriteCloser) {
			return &lwr, &ewr
		},
		Events:    newMockEventer(),
		LogClient: &mockLogClient{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond*100, cancel)
	el.Run(ctx)
	assert.Equal(t, 2, len(el.logStreams), "2 streams in, 1 removed")
	assert.Equal(t, "c2", el.logStreams["id2"].ContainerName)
	assert.Equal(t, "c3", el.logStreams["id3"].ContainerName)
}

type mockLogClient struct {
}

func (m *mockLogClient) Logs(opts docker.LogsOptions) error {
	return nil
}

type mockWriter struct {
	bytes.Buffer
}

func (m *mockWriter) Close() error { return nil }

type mockEventer struct {
	ch chan discovery.Event
}

func newMockEventer() *mockEventer {
	ch := make(chan discovery.Event, 10)
	ch <- discovery.Event{Status: true, ContainerName: "c1", Group: "g1", ContainerID: "id1"}
	ch <- discovery.Event{Status: true, ContainerName: "c2", Group: "g1", ContainerID: "id2"}
	ch <- discovery.Event{Status: true, ContainerName: "c3", Group: "g2", ContainerID: "id3"}
	ch <- discovery.Event{Status: false, ContainerName: "c1", Group: "g1", ContainerID: "id1"}
	close(ch)
	return &mockEventer{ch: ch}
}

func (m *mockEventer) Channel() <-chan discovery.Event { return m.ch }
