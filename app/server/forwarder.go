package server

import (
	"context"
	"sync"
	"time"

	log "github.com/go-pkgz/lgr"

	"github.com/umputun/dkll/app/core"
)

// Forwarder tails syslog messages, parses entries and pushes to Publisher (store) and file logger(s)
type Forwarder struct {
	Publisher  Publisher
	Syslog     SyslogBackgroundReader
	FileWriter FileWriter
}

// Publisher to store
type Publisher interface {
	Publish(records []core.LogEntry) (err error)
	LastPublished() (entry core.LogEntry, err error)
}

// SyslogBackgroundReader provides aysnc runner returning the channel for incoming messages
type SyslogBackgroundReader interface {
	Go(ctx context.Context) <-chan string
}

// FileSubmitter writes entry to all log files
type FileWriter interface {
	Write(rec core.LogEntry) error
}

// Run executes forwarder in endless (blocking) loop
func (f *Forwarder) Run(ctx context.Context) {
	log.Print("[INFO] run forwarder from syslog")
	messages := make(chan core.LogEntry, 10000)
	writerWg := f.backgroundWriter(ctx, messages)

	if pe, err := f.Publisher.LastPublished(); err == nil {
		log.Printf("[DEBUG] last published [%s : %s]", pe.ID, pe)
	}

	syslogCh := f.Syslog.Go(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[WARN] forwarder terminated, %v", ctx.Err())
			writerWg.Wait() // wait for backgroundWriter completion
			return
		case line := <-syslogCh:
			ent, err := core.NewEntry(line, time.Local)
			if err != nil {
				log.Printf("[WARN] failed to make entry from %q, %v", line, err)
				continue
			}
			messages <- ent
		}
	}

}

func (f *Forwarder) backgroundWriter(ctx context.Context, messages <-chan core.LogEntry) *sync.WaitGroup {
	log.Print("[INFO] forwarder's writer activated")
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		buffer := make([]core.LogEntry, 0, 1001)

		// send buffer to publisher and file logger
		writeBuff := func() (wrote int) {
			if len(buffer) == 0 {
				return 0
			}

			if err := f.Publisher.Publish(buffer); err != nil {
				log.Printf("[WARN] failed to publish, error=%s", err)
			}
			wrote = len(buffer)
			for _, r := range buffer {
				if err := f.FileWriter.Write(r); err != nil {
					log.Printf("[WARN] failed to write to logs, %v", err)
				}
			}
			log.Printf("[DEBUG] wrote %d entries", len(buffer))
			buffer = buffer[0:0]
			return wrote
		}

		ticks := time.NewTicker(time.Millisecond * 500)
		for {
			select {
			case <-ctx.Done():
				writeBuff()
				log.Print("[DEBUG] background writer terminated")
				return
			case msg := <-messages:
				buffer = append(buffer, msg)
				if len(buffer) >= 1000 { // forced flush every 1000
					writeBuff()
				}
			case <-ticks.C: // flush every 1/2 second
				writeBuff()
			}
		}
	}()

	return &wg
}
