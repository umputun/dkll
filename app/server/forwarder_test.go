package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
	"github.com/umputun/dkll/app/server/mocks"
)

func TestForwarderTickHappened(t *testing.T) {
	log.Setup(log.Debug)

	mp := mockPublisher{}
	fw := mockFileWriter{}
	f := Forwarder{
		Publisher:  &mp,
		Syslog:     &mockSyslogBackgroundReader{},
		FileWriter: &fw,
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond*700, func() { // tick every 500ms
		cancel()
	})

	_ = f.Run(ctx)
	assert.Equal(t, 100, len(mp.get()), "all valid records sent to publisher")
	assert.Equal(t, 100, len(fw.get()), "all valid records sent to file log")
}

func TestForwarderFastClose(t *testing.T) {
	log.Setup(log.Debug)

	mp := mockPublisher{}
	fw := mockFileWriter{}
	f := Forwarder{Syslog: &mockSyslogBackgroundReader{}, Publisher: &mp, FileWriter: &fw}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond*200, func() { // tick every 500ms. close before first tick
		cancel()
	})

	_ = f.Run(ctx)

	assert.Equal(t, 100, len(mp.get()), "all valid records sent to publisher")
	assert.Equal(t, 100, len(fw.get()), "all valid records sent to file log")
}

func TestForwarderMalformedLine(t *testing.T) {
	log.Setup(log.Debug)

	mp := mockPublisher{}
	fw := mockFileWriter{}
	f := Forwarder{
		Publisher: &mp,
		Syslog: &mockSyslogLinesReader{lines: []string{
			"Oct 19 15:29:43 ", // truncated line, used to panic on parsing
			"May 30 18:03:28 BigMac.local docker/test123[63415]: some msg",
		}},
		FileWriter: &fw,
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond*700, func() { // tick every 500ms
		cancel()
	})

	_ = f.Run(ctx)

	recs := mp.get()
	require.Equal(t, 1, len(recs), "valid record processed after the malformed one")
	assert.Equal(t, "some msg", recs[0].Msg)
	assert.Equal(t, 1, len(fw.get()), "valid record sent to file log")
}

func TestForwarder_DrainsSyslogOnCancel(t *testing.T) {
	// regression: run returned after one value instead of waiting for the syslog channel to close
	synctest.Test(t, func(t *testing.T) {
		ch := make(chan string, 2)
		sr := &mocks.SyslogBackgroundReaderMock{GoFunc: func(context.Context) (<-chan string, error) { return ch, nil }}
		f := Forwarder{Syslog: sr, Publisher: &mockPublisher{}, FileWriter: &mockFileWriter{}}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan error, 1)
		go func() { done <- f.Run(ctx) }()
		synctest.Wait()

		ch <- "May 30 18:03:28 BigMac.local docker/test123[63415]: late msg 1"
		ch <- "May 30 18:03:28 BigMac.local docker/test123[63415]: late msg 2"
		close(ch)
		require.ErrorIs(t, <-done, context.Canceled)
		_, open := <-ch
		assert.False(t, open, "syslog channel drained to closure")
	})
}

func TestForwarder_CancelWithFullQueue(t *testing.T) {
	// regression: run blocked forever on a full queue after the writer exited on cancel
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		mp := &mocks.PublisherMock{
			PublishFunc: func([]core.LogEntry) error {
				<-ctx.Done()
				return nil
			},
			LastPublishedFunc: func() (core.LogEntry, error) { return core.LogEntry{}, nil },
		}
		sr := &mocks.SyslogBackgroundReaderMock{GoFunc: func(ctx context.Context) (<-chan string, error) {
			ch := make(chan string)
			go func() {
				defer close(ch)
				for ctx.Err() == nil {
					select {
					case ch <- "May 30 18:03:28 BigMac.local docker/test123[63415]: some msg":
					case <-ctx.Done():
					}
				}
			}()
			return ch, nil
		}}
		f := Forwarder{Syslog: sr, Publisher: mp, FileWriter: &mockFileWriter{}}

		done := make(chan error, 1)
		go func() { done <- f.Run(ctx) }()
		synctest.Wait()
		require.Len(t, mp.PublishCalls(), 1, "writer is parked in publish and the queue is full")

		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
	})
}

type mockSyslogLinesReader struct{ lines []string }

func (m *mockSyslogLinesReader) Go(context.Context) (<-chan string, error) {
	ch := make(chan string, len(m.lines))
	for _, line := range m.lines {
		ch <- line
	}
	close(ch)
	return ch, nil
}

type mockSyslogBackgroundReader struct{}

func (m *mockSyslogBackgroundReader) Go(context.Context) (<-chan string, error) {
	ch := make(chan string, 101)
	for i := range 100 {
		ch <- fmt.Sprintf("May 30 18:03:28 BigMac.local docker/test123[63415]: some msg %d", i)
	}
	ch <- "May 30 18:03:28 BigMac.local docker/err[63415]: some bad msg"
	close(ch)
	return ch, nil
}

type mockFileWriter struct {
	recs []core.LogEntry
	sync.Mutex
}

func (m *mockFileWriter) Write(rec core.LogEntry) error {
	m.Lock()
	defer m.Unlock()
	if rec.Container == "err" {
		return errors.New("file write error")
	}
	m.recs = append(m.recs, rec)
	return nil
}

func (m *mockFileWriter) get() []core.LogEntry {
	m.Lock()
	defer m.Unlock()
	res := make([]core.LogEntry, len(m.recs))
	copy(res, m.recs)
	return res
}

type mockPublisher struct {
	recs []core.LogEntry
	sync.Mutex
}

func (m *mockPublisher) Publish(records []core.LogEntry) (err error) {
	m.Lock()
	defer m.Unlock()
	for _, rec := range records {
		if rec.Container == "err" {
			err = errors.New("publisher error")
			continue
		}
		m.recs = append(m.recs, rec)
	}
	return err
}

func (m *mockPublisher) get() []core.LogEntry {
	m.Lock()
	defer m.Unlock()
	res := make([]core.LogEntry, len(m.recs))
	copy(res, m.recs)
	return res
}

func (m *mockPublisher) LastPublished() (entry core.LogEntry, err error) {
	return core.LogEntry{}, nil
}
