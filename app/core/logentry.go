package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// LogEntry represents a single event for forwarder and rest server and client
type LogEntry struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	Container string    `json:"container"`
	Pid       int       `json:"pid"`
	Msg       string    `json:"msg"`
	Ts        time.Time `json:"ts"`
	CreatedTs time.Time `json:"cts"`
}

// NewEntry makes the LogEntry from a log line.
// example:	"Oct 19 15:29:43 host-1 docker/mongo[888]: 2015-10-19T19:29:43 blah blah blah"
func NewEntry(line string, tz *time.Location) (entry LogEntry, err error) {

	if len(line) < 16 { // 16 is minimal size of "Jan _2 15:04:05" timestamp
		return entry, fmt.Errorf("line is too short, line=[%s]", line)
	}

	entry = LogEntry{Container: "syslog", Pid: 0, CreatedTs: time.Now()}

	entry.Ts, line, err = parseTime(line, tz)
	if err != nil {
		return entry, err
	}

	// get host
	entry.Host = strings.Split(line, " ")[0]
	line = line[len(entry.Host)+1:]

	// get service/container[pid]
	serviceContainerPid := strings.Split(line, " ")[0]
	serviceContainerPidElems := strings.Split(serviceContainerPid, "/")
	if strings.HasPrefix(serviceContainerPidElems[0], "docker") && len(serviceContainerPidElems) > 1 { // skip non-docker msgs
		containerAndPid := serviceContainerPidElems[1]
		pidElems := strings.Split(containerAndPid, "[")
		entry.Container = pidElems[0]
		if len(pidElems) > 1 {
			pidStr := strings.TrimSuffix(pidElems[1], ":")
			pidStr = strings.TrimSuffix(pidStr, "]")
			if pid, err := strconv.Atoi(pidStr); err == nil {
				entry.Pid = pid
			}
		}
	}

	entry.Msg = strings.TrimSpace(line[len(serviceContainerPid)+1:])
	return entry, nil
}

// parseTime gets date-time part of the log line and extracts. Returns tx and trimmed line
// supports "2006 Jan _2 15:04:05" and RFC3339 layouts
func parseTime(line string, tz *time.Location) (ts time.Time, trimmedLine string, err error) {

	if len(line) < 16 {
		return ts, trimmedLine, errors.Errorf("line %q too short to extract time", line)
	}
	tsy := fmt.Sprintf("%d %s", time.Now().Year(), line[0:15]) // try ts like "Oct 19 15:29:43"
	ts, err = time.ParseInLocation("2006 Jan _2 15:04:05", tsy, tz)
	if err == nil {
		trimmedLine = line[16:]
		return ts, trimmedLine, nil
	}

	// try RFC3339
	dt := strings.Split(line, " ")[0]
	if ts, err = time.Parse(time.RFC3339, dt); err != nil {
		return time.Time{}, line, errors.Wrapf(err, "can't extract time from %q", line)
	}
	return ts.In(tz), line[len(dt)+1:], nil
}

func (entry LogEntry) String() string {
	return fmt.Sprintf("%s : %s/%s [%d] - %s", entry.Ts.In(time.Local), entry.Host, entry.Container, entry.Pid, entry.Msg)
}
