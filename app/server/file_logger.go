package server

import (
	"io"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/umputun/dkll/app/core"
)

// FileLogger contains writers for containers and merged writer for all sources
type FileLogger struct {
	merged         io.Writer
	writersFactory WritersFactory
	writers        map[dkKey]io.Writer
	lock           sync.Mutex
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
		writers:        map[dkKey]io.Writer{},
	}
}

// Write log entry to local files
func (r *FileLogger) Write(rec core.LogEntry) error {
	r.lock.Lock()
	defer r.lock.Unlock()

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
	return errs.ErrorOrNil()
}
