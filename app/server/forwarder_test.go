package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/umputun/dkll/app/core"
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

type mockSyslogBackgroundReader struct{}

func (m *mockSyslogBackgroundReader) Go(ctx context.Context) (<-chan string, error) {
	ch := make(chan string, 101)
	for i := 0; i < 100; i++ {
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
