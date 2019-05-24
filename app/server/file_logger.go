package server

import (
	"context"
	"io"
	"sync"

	log "github.com/go-pkgz/lgr"
	"github.com/hashicorp/go-multierror"

	"github.com/umputun/dkll/app/core"
)

// FileLogger contains writers for containers and merged writer for all sources
type FileLogger struct {
	merged         io.Writer
	writersFactory WritersFactory

	writers map[dkKey]io.Writer
	ch      chan core.LogEntry
	once    sync.Once
}

type WritersFactory func(instance, container string) io.Writer

type dkKey struct {
	host      string
	container string
}

func NewFileLogger(wrf WritersFactory, m io.Writer) *FileLogger {
	return &FileLogger{
		merged:         m,
		writersFactory: wrf,
		ch:             make(chan core.LogEntry, 10000),
		writers:        map[dkKey]io.Writer{},
	}
}

// Submit log entry to local channel
func (r *FileLogger) Submit(rec core.LogEntry) {
	r.ch <- rec
}

// Do runs blocking publisher to local files
func (r *FileLogger) Do(ctx context.Context) error {

	log.Print("[INFO] activate file logger")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case rec := <-r.ch:
			errs := new(multierror.Error)
			key := dkKey{host: rec.Host, container: rec.Container}
			_, err := r.merged.Write([]byte(rec.String() + "\n"))
			errs = multierror.Append(errs, err)

			wr, ok := r.writers[key]
			if !ok {
				wr = r.writersFactory(rec.Host, rec.Container)
				r.writers[key] = wr
			}
			_, err = wr.Write([]byte(rec.Msg + "\n"))
			errs = multierror.Append(errs, err)

			if errs.ErrorOrNil() != nil {
				log.Printf("[WARN] failed to write to log file(s) %v, %v", key, errs.ErrorOrNil())
			}
		}
	}
}
