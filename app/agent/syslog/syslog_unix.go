// +build !windows,!nacl,!plan9

package syslog

import (
	"io"
	"log/syslog"
)

// GetWriter returns syslog writer for non-win platform
func GetWriter(syslogHost, syslogPrefix, containerName string) (io.WriteCloser, error) {
	return syslog.Dial("udp4", syslogHost, syslog.LOG_WARNING|syslog.LOG_DAEMON, syslogPrefix+containerName)
}

// IsSupported indicates if the platform supports syslog
func IsSupported() bool {
	return true
}
