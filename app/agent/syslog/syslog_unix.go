// +build !windows,!nacl,!plan9

package syslog

import (
	"context"
	"io"
	"log/syslog"
	"time"

	log "github.com/go-pkgz/lgr"

	"github.com/go-pkgz/repeater"
	"github.com/pkg/errors"
)

// GetWriter returns syslog writer for non-win platform
func GetWriter(ctx context.Context, host, proto, prefix, containerName string) (res io.WriteCloser, err error) {

	var wr *syslog.Writer
	switch proto {
	case "udp", "udp4":
		e := repeater.NewDefault(10, time.Second).Do(ctx, func() error {
			res, err = syslog.Dial("udp4", host, syslog.LOG_WARNING|syslog.LOG_DAEMON, prefix+containerName)
			return err
		})
		return res, e
	case "tcp", "tcp4":
		e := repeater.NewDefault(10, time.Second).Do(ctx, func() error {
			wr, err = syslog.Dial("tcp4", host, syslog.LOG_WARNING|syslog.LOG_DAEMON, prefix+containerName)
			return err
		})
		if e != nil {
			log.Printf("[FATAL] failed to make writer for tcp syslog transport, %v", e)
		}
		return &syslogRetryWriter{ctx: ctx, swr: wr}, e
	}
	return nil, errors.Errorf("unknown syslog protocol %s", proto)
}

// IsSupported indicates if the platform supports syslog
func IsSupported() bool {
	return true
}

// syslogRetryWriter wraps syslog.Writer with connection close on write errors and causes con=nil
// syslog.Write will redial if conn=nil
type syslogRetryWriter struct {
	swr *syslog.Writer
	ctx context.Context
}

func (s *syslogRetryWriter) Write(p []byte) (n int, err error) {
	e := repeater.NewDefault(10, 500*time.Millisecond).Do(s.ctx, func() error {
		if n, err = s.swr.Write(p); err != nil {
			log.Printf("[DEBUG] write to syslog failed, %v", err)
			_ = s.swr.Close()
		}
		return err
	})
	if e != nil {
		log.Printf("[FATAL] all write retries to syslog failed, %v", err)
	}
	return n, e
}

func (s *syslogRetryWriter) Close() error {
	return s.swr.Close()
}
