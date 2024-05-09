package server

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/umputun/dkll/app/core"
)

func TestFileLogger(t *testing.T) {
	containerWriters := []*bytes.Buffer{
		bytes.NewBuffer(nil),
		bytes.NewBuffer(nil),
		bytes.NewBuffer(nil),
		bytes.NewBuffer(nil),
	}
	containerWritersNum := 0
	wrf := func(string, string) io.Writer {
		res := containerWriters[containerWritersNum]
		containerWritersNum++
		return res
	}
	merged := &bytes.Buffer{}

	l := NewFileLogger(wrf, merged)

	ts := time.Date(2019, 5, 24, 20, 54, 30, 123, time.Local)
	assert.NoError(t, l.Write(core.LogEntry{ID: "01", Host: "h1", Container: "c1", Msg: "msg1", TS: ts}))
	assert.NoError(t, l.Write(core.LogEntry{ID: "02", Host: "h1", Container: "c2", Msg: "msg2", TS: ts}))
	assert.NoError(t, l.Write(core.LogEntry{ID: "02", Host: "h2", Container: "c1", Msg: "msg3", TS: ts}))
	assert.NoError(t, l.Write(core.LogEntry{ID: "03", Host: "h1", Container: "c1", Msg: "msg4", TS: ts}))
	assert.NoError(t, l.Write(core.LogEntry{ID: "04", Host: "h1", Container: "c2", Msg: "msg5", TS: ts}))
	assert.NoError(t, l.Write(core.LogEntry{ID: "05", Host: "h2", Container: "c2", Msg: "msg6", TS: ts}))

	// check containers writes
	assert.Equal(t, 4, containerWritersNum, "4 host+container combos")
	assert.Equal(t, "msg1\nmsg4\n", containerWriters[0].String())
	assert.Equal(t, "msg2\nmsg5\n", containerWriters[1].String())
	assert.Equal(t, "msg3\n", containerWriters[2].String())
	assert.Equal(t, "msg6\n", containerWriters[3].String())

	// check merged writes
	wrMergeLines := strings.Split(merged.String(), "\n")
	assert.Equal(t, 7, len(wrMergeLines))
	assert.Equal(t, "2019-05-24 20:54:30.000000123 -0500 CDT : h1/c1 [0] - msg1", wrMergeLines[0])
	assert.Equal(t, "2019-05-24 20:54:30.000000123 -0500 CDT : h2/c2 [0] - msg6", wrMergeLines[5])
	assert.Equal(t, "", wrMergeLines[6], "last write is \n")
}
