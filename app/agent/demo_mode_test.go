package agent

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestDemoEmitter_Logs(t *testing.T) {
	d := DemoEmitter{time.Millisecond * 100}
	wr := mockDemoWriter{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1090)
	defer cancel()
	err := d.Logs(docker.LogsOptions{Context: ctx, OutputStream: &wr})
	assert.EqualError(t, err, "context deadline exceeded")
	t.Logf("%+v", wr.Get())
	assert.Equal(t, 10+1, len(wr.Get()), "10 messages with extra \n")
}

type mockDemoWriter struct {
	sync.Mutex
	bytes.Buffer
}

func (m *mockDemoWriter) Write(p []byte) (int, error) {
	m.Lock()
	defer m.Unlock()
	return m.Buffer.Write(p)
}

func (m *mockDemoWriter) Get() []string {
	m.Lock()
	res := m.String()
	m.Unlock()
	return strings.Split(res, "\n")
}
