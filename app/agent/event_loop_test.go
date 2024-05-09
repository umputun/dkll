package agent

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestAgent(t *testing.T) {

	lwr, ewr := mockWriter{}, mockWriter{}
	el := EventLoop{
		WriterFactory: func(context.Context, string, string) (logWriter, errWriter io.WriteCloser, err error) {
			return &lwr, &ewr, nil
		},
		Events:     newMockEventer(),
		LogEmitter: &mockLogClient{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond*100, cancel)
	el.Run(ctx)
	assert.Equal(t, 2, len(el.logStreams), "2 streams in, 1 removed")
	assert.Equal(t, "c2", el.logStreams["id2"].Name())
	assert.Equal(t, "c3", el.logStreams["id3"].Name())
}

func TestDemo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	lwr, ewr := mockWriter{}, mockWriter{}
	el := EventLoop{
		WriterFactory: func(context.Context, string, string) (logWriter, errWriter io.WriteCloser, err error) {
			return &lwr, &ewr, nil
		},
		Events:     NewDemoEventNotifier(ctx),
		LogEmitter: &DemoEmitter{Duration: 100 * time.Millisecond},
	}
	time.AfterFunc(time.Millisecond*1000, cancel)
	el.Run(ctx)
	wrStrings := lwr.Get()
	t.Logf("%v", wrStrings)
	assert.True(t, len(wrStrings) >= 25 && len(wrStrings) <= 31, len(wrStrings))
}

type mockLogClient struct{}

func (m *mockLogClient) Logs(docker.LogsOptions) error {
	return nil
}

type mockWriter struct {
	sync.Mutex
	bytes.Buffer
}

func (m *mockWriter) Write(p []byte) (int, error) {
	m.Lock()
	defer m.Unlock()
	return m.Buffer.Write(p)
}

func (m *mockWriter) Get() []string {
	m.Lock()
	res := m.Buffer.String()
	m.Unlock()
	return strings.Split(res, "\n")
}

func (m *mockWriter) Close() error { return nil }

func (m *mockWriter) String() string {
	return "mockWriter"
}

type mockEventer struct {
	ch chan Event
}

func newMockEventer() *mockEventer {
	ch := make(chan Event, 10)
	ch <- Event{Status: true, ContainerName: "c1", Group: "g1", ContainerID: "id1"}
	ch <- Event{Status: true, ContainerName: "c2", Group: "g1", ContainerID: "id2"}
	ch <- Event{Status: true, ContainerName: "c3", Group: "g2", ContainerID: "id3"}
	ch <- Event{Status: true, ContainerName: "c3", Group: "g2", ContainerID: "id3"} // dbl start
	ch <- Event{Status: false, ContainerName: "c1", Group: "g1", ContainerID: "id1"}
	close(ch)
	return &mockEventer{ch: ch}
}

func (m *mockEventer) Channel() <-chan Event { return m.ch }
