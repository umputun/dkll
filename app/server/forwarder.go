package server

import (
	"context"
	"time"

	log "github.com/go-pkgz/lgr"

	"github.com/umputun/dkll/app/core"
)

// Forwarder tails syslog messages, parses entries and pushes Publisher
type Forwarder struct {
	Publisher  Publisher
	Syslog     Syslog
	FileLogger *FileLogger
}

type Publisher interface {
	Publish(buffer []core.LogEntry) (err error)
	LastPublished() (entry core.LogEntry, err error)
}

// Run executes forwarder in endless (blocking) loop
func (f *Forwarder) Run(ctx context.Context) {
	log.Print("[INFO] run forwarder from syslog")
	messages := make(chan core.LogEntry, 10000)
	f.backgroundPublisher(ctx, messages)

	go func() {
		if err := f.FileLogger.Do(ctx); err != nil {
			log.Printf("[WARN] file logger error %v", err)
		}
	}()

	pe, _ := f.Publisher.LastPublished()
	log.Printf("[DEBUG] last published = [%s : %s]", pe.ID.Hex(), pe)

	syslogCh := f.Syslog.Go(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[WARN] forwarder terminated, %v", ctx.Err())
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

func (f *Forwarder) backgroundPublisher(ctx context.Context, messages <-chan core.LogEntry) {
	log.Print("[INFO] mongo publisher activated")
	go func() {
		buffer := make([]core.LogEntry, 0, 1001)

		writeBuff := func() (wrote int) {
			if len(buffer) == 0 {
				return 0
			}

			if err := f.Publisher.Publish(buffer); err != nil {
				log.Printf("[ERROR] failed to insert, error=%s", err)
				return 0
			}
			wrote = len(buffer)
			for _, r := range buffer {
				f.FileLogger.Submit(r)
			}
			log.Printf("[DEBUG] wrote %d entries", len(buffer))
			buffer = buffer[:0]
			return wrote
		}

		ticks := time.Tick(time.Millisecond * 500)
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-messages:
				buffer = append(buffer, msg)
				if len(buffer) >= 1000 { // forced flush every 1000
					writeBuff()
				}
			case <-ticks: // flush every 1/2 second
				writeBuff()
			}
		}
	}()

}
