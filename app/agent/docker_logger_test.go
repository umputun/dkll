package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/go-pkgz/lgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_WithError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mock := mockLogsPublisher{err: nil, ctx: ctx}
	l := NewContainerLogStreamer(ContainerStreamerParams{ID: "test_id", Name: "test_name", LogsEmitter: &mock})
	st := time.Now()
	go func() {
		e := l.Run()
		require.NotNil(t, e)
		assert.EqualError(t, e, "context canceled", e.Error())
	}()
	time.Sleep(10 * time.Millisecond)
	time.AfterFunc(time.Second, func() {
		cancel()
	})
	l.Wait()
	assert.True(t, time.Since(st) < time.Second*2, "terminated early")
	time.Sleep(10 * time.Millisecond)
}

func TestLogger_Close(t *testing.T) {
	ctx := context.Background()

	mock := mockLogsPublisher{err: nil, ctx: ctx}
	l := NewContainerLogStreamer(ContainerStreamerParams{ID: "test_id", Name: "test_name", LogsEmitter: &mock})
	st := time.Now()

	go func() {
		assert.NoError(t, l.Run())
	}()
	time.Sleep(100 * time.Millisecond)
	time.AfterFunc(2*time.Second, func() {
		assert.NoError(t, l.Close())
	})

	l.Wait()
	assert.True(t, time.Since(st) >= time.Second*2, fmt.Sprintf("elapsed: %v", time.Since(st)))
}

type mockLogsPublisher struct {
	err error
	ctx context.Context
}

func (m *mockLogsPublisher) Logs(opts docker.LogsOptions) error {
	select {
	case <-time.After(2 * time.Second):
		log.Printf("mock log completed %+v", opts)
		m.err = nil
	case <-m.ctx.Done():
		log.Printf("mock log terminated %+v", opts)
		m.err = m.ctx.Err()
	}
	return m.err
}
