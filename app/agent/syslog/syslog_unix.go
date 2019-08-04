// +build !windows,!nacl,!plan9

package syslog

import (
	"context"
	"io"
	"log/syslog"
	"time"

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
		return &syslogRetryWriter{swr: wr}, e
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
}

func (s *syslogRetryWriter) Write(p []byte) (n int, err error) {
	n, err = s.swr.Write(p)
	if err != nil {
		_ = s.swr.Close()
		return 0, err
	}
	return n, err
}

func (s *syslogRetryWriter) Close() error {
	return s.swr.Close()
}
