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

	switch proto {
	case "udp", "udp4":
		e := repeater.NewDefault(10, time.Second).Do(ctx, func() error {
			res, err = syslog.Dial("udp4", host, syslog.LOG_WARNING|syslog.LOG_DAEMON, prefix+containerName)
			return err
		})
		return res, e
	case "tcp", "tcp4":
		e := repeater.NewDefault(10, time.Second).Do(ctx, func() error {
			res, err = syslog.Dial("tcp4", host, syslog.LOG_WARNING|syslog.LOG_DAEMON, prefix+containerName)
			return err
		})
		return res, e
	}
	return nil, errors.Errorf("unknown syslog protocol %s", proto)
}

// IsSupported indicates if the platform supports syslog
func IsSupported() bool {
	return true
}
