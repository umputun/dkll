package cmd

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Run(t *testing.T) {

	if os.Getenv("TEST_DOCKER") == "" {
		t.Skip("skip docker tests")
	}

	defer os.RemoveAll("/tmp/logger.test")
	opts := AgentOpts{
		DockerHost:    "unix:///var/run/docker.sock",
		FilesLocation: "/tmp/logger.test",
		EnableFiles:   true,
		MaxFileSize:   1,
		MaxFilesCount: 10,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancel()
	a := AgentCmd{AgentOpts: opts}
	err := a.Run(ctx)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond) // let it start
}

func Test_makeLogWriters(t *testing.T) {
	defer os.RemoveAll("/tmp/logger.test")

	opts := AgentOpts{FilesLocation: "/tmp/logger.test", EnableFiles: true, MaxFileSize: 1, MaxFilesCount: 10}
	a := AgentCmd{AgentOpts: opts}
	stdWr, errWr, err := a.makeLogWriters("container1", "gr1")
	require.NoError(t, err)
	assert.NotEqual(t, stdWr, errWr, "different writers for out and err")

	// write to out writer
	_, err = stdWr.Write([]byte("abc line 1\n"))
	assert.NoError(t, err)
	_, err = stdWr.Write([]byte("xxx123 line 2\n"))
	assert.NoError(t, err)

	// write to err writer
	_, err = errWr.Write([]byte("err line 1\n"))
	assert.NoError(t, err)
	_, err = errWr.Write([]byte("xxx123 line 2\n"))
	assert.NoError(t, err)

	r, err := ioutil.ReadFile("/tmp/logger.test/gr1/container1.log")
	assert.NoError(t, err)
	assert.Equal(t, "abc line 1\nxxx123 line 2\n", string(r))

	r, err = ioutil.ReadFile("/tmp/logger.test/gr1/container1.err")
	assert.NoError(t, err)
	assert.Equal(t, "err line 1\nxxx123 line 2\n", string(r))

	assert.NoError(t, stdWr.Close())
	assert.NoError(t, errWr.Close())
}

func Test_makeLogWritersMixed(t *testing.T) {
	defer os.RemoveAll("/tmp/logger.test")

	opts := AgentOpts{FilesLocation: "/tmp/logger.test", EnableFiles: true, MaxFileSize: 1, MaxFilesCount: 10, MixErr: true}
	a := AgentCmd{AgentOpts: opts}
	stdWr, errWr, err := a.makeLogWriters("container1", "gr1")
	require.NoError(t, err)
	assert.Equal(t, stdWr, errWr, "same writer for out and err in mixed mode")

	// write to out writer
	_, err = stdWr.Write([]byte("abc line 1\n"))
	assert.NoError(t, err)
	_, err = stdWr.Write([]byte("xxx123 line 2\n"))
	assert.NoError(t, err)

	// write to err writer
	_, err = errWr.Write([]byte("err line 1\n"))
	assert.NoError(t, err)
	_, err = errWr.Write([]byte("xxx123 line 2\n"))
	assert.NoError(t, err)

	r, err := ioutil.ReadFile("/tmp/logger.test/gr1/container1.log")
	assert.NoError(t, err)
	assert.Equal(t, "abc line 1\nxxx123 line 2\nerr line 1\nxxx123 line 2\n", string(r))

	assert.NoError(t, stdWr.Close())
	assert.NoError(t, errWr.Close())
}

func Test_makeLogWritersWithJSON(t *testing.T) {
	defer os.RemoveAll("/tmp/logger.test")
	opts := AgentOpts{FilesLocation: "/tmp/logger.test", EnableFiles: true, MaxFileSize: 1, MaxFilesCount: 10, ExtJSON: true}
	a := AgentCmd{AgentOpts: opts}
	stdWr, errWr, err := a.makeLogWriters("container1", "gr1")
	require.NoError(t, err)

	// write to out writer
	_, err = stdWr.Write([]byte("abc line 1"))
	assert.NoError(t, err)

	r, err := ioutil.ReadFile("/tmp/logger.test/gr1/container1.log")
	assert.NoError(t, err)
	assert.Contains(t, string(r), `"msg":"abc line 1","container":"container1","group":"gr1"`)

	_, err = os.Stat("/tmp/logger.test/gr1/container1.err")
	assert.NotNil(t, err)

	assert.NoError(t, stdWr.Close())
	assert.NoError(t, errWr.Close())
}

func Test_makeLogWritersSyslogFailed(t *testing.T) {
	opts := AgentOpts{EnableSyslog: true}
	a := AgentCmd{AgentOpts: opts}
	_, _, err := a.makeLogWriters("container1", "gr1")
	require.NotNil(t, err)
}

func Test_makeLogWritersSyslogPassed(t *testing.T) {
	opts := AgentOpts{EnableSyslog: true, SyslogHost: "127.0.0.1:514", SyslogPrefix: "docker/"}
	a := AgentCmd{AgentOpts: opts}
	stdWr, errWr, err := a.makeLogWriters("container1", "gr1")
	require.NoError(t, err)
	assert.Equal(t, stdWr, errWr, "same writer for out and err in syslog")

	// write to out writer
	_, err = stdWr.Write([]byte("abc line 1\n"))
	assert.NoError(t, err)
	_, err = stdWr.Write([]byte("xxx123 line 2\n"))
	assert.NoError(t, err)

	// write to err writer
	_, err = errWr.Write([]byte("err line 1\n"))
	assert.NoError(t, err)
	_, err = errWr.Write([]byte("xxx123 line 2\n"))
	assert.NoError(t, err)
}
