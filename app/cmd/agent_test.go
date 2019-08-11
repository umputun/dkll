package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
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

func TestDemoMode(t *testing.T) {
	defer os.RemoveAll("/tmp/logger.test")
	opts := AgentOpts{FilesLocation: "/tmp/logger.test", EnableFiles: true, MaxFileSize: 1, MaxFilesCount: 10,
		DemoMode: true, DemoRecEvery: time.Millisecond * 100}
	a := AgentCmd{AgentOpts: opts}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancel()
	require.NoError(t, a.Run(ctx))

	b, err := ioutil.ReadFile("/tmp/logger.test/system/nginx.log")
	assert.NoError(t, err)
	t.Log(string(b))
}

func Test_makeLogWriters(t *testing.T) {
	defer os.RemoveAll("/tmp/logger.test")

	opts := AgentOpts{FilesLocation: "/tmp/logger.test", EnableFiles: true, MaxFileSize: 1, MaxFilesCount: 10}
	a := AgentCmd{AgentOpts: opts}
	stdWr, errWr, err := a.makeLogWriters(context.Background(), "container1", "gr1")
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
	stdWr, errWr, err := a.makeLogWriters(context.Background(), "container1", "gr1")
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
	stdWr, errWr, err := a.makeLogWriters(context.Background(), "container1", "gr1")
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
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)
	_, _, err := a.makeLogWriters(ctx, "container1", "gr1")
	require.NotNil(t, err)
}

func Test_makeLogWritersSyslogPassed(t *testing.T) {
	opts := AgentOpts{EnableSyslog: true, SyslogHost: "127.0.0.1:514", SyslogProt: "udp", SyslogPrefix: "docker/"}
	a := AgentCmd{AgentOpts: opts}
	stdWr, errWr, err := a.makeLogWriters(context.Background(), "container1", "gr1")
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

func Test_makeLogWritersSyslogTCP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(1*time.Second, cancel)

	var buf syncedBuffer
	var wg sync.WaitGroup
	startTcpServer(t, ctx, 5514, &wg, &buf)

	opts := AgentOpts{EnableSyslog: true, SyslogHost: "127.0.0.1:5514", SyslogProt: "tcp", SyslogPrefix: "docker/"}
	a := AgentCmd{AgentOpts: opts}

	stdWr, errWr, err := a.makeLogWriters(ctx, "container1", "gr1")
	require.NoError(t, err)
	assert.NotEqual(t, stdWr, errWr, "not the same writer for out and err in syslog")

	// write to out writer
	_, err = stdWr.Write([]byte("abc line 1\n"))
	assert.NoError(t, err)
	_, err = stdWr.Write([]byte("xxx123 line 2\n"))
	assert.NoError(t, err)

	// write to err writer
	_, err = errWr.Write([]byte("err line 1\n"))
	assert.NoError(t, err)
	_, err = errWr.Write([]byte("err xxx123 line 2345\n"))
	assert.NoError(t, err)

	wg.Wait()
	t.Log(buf.String())
	res := strings.Split(buf.String(), "\n")
	assert.Equal(t, 5, len(res), "4 messages + final eol")
	assert.Contains(t, res[0], "<30>")
	assert.Contains(t, res[0], "docker/container1")
	assert.Contains(t, res[0], ": abc line 1")
	assert.Contains(t, res[3], "<27>")
	assert.Contains(t, res[3], "docker/container1")
	assert.Contains(t, res[3], ": err xxx123 line 2345")
}

func startTcpServer(t *testing.T, ctx context.Context, port int, wg *sync.WaitGroup, buf *syncedBuffer) {

	closed := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	log.Print("start test server on ", port)
	wg.Add(1)

	go func() {
		defer wg.Done()
		ts, err := net.Listen("tcp4", fmt.Sprintf("localhost:%d", port))
		require.NoError(t, err)
		log.Print("test server listen on ", port)

		go func() {
			<-ctx.Done()
			ts.Close()
		}()

		for {
			var conn net.Conn
			if closed() {
				ts.Close()
				return
			}
			log.Print("listen to accept")
			if conn, err = ts.Accept(); err != nil {
				break
			}

			log.Printf("connection accepted from %v", conn.RemoteAddr())
			go func(conn net.Conn) {
				defer conn.Close()
				for {
					select {
					case <-ctx.Done():
						log.Print("canceled test server")
						return
					case <-time.After(1 * time.Millisecond):
						require.NoError(t, conn.SetDeadline(time.Now().Add(time.Millisecond*100)))
						b := make([]byte, 1500)

						l, err := conn.Read(b)
						if err != nil {
							continue
						}
						if _, e := buf.Write(b[:l]); e == nil {
							log.Printf("> wrote %s", string(b[:l]))
						} else {
							log.Printf("! write failed %s, %v", string(b[:l]), e)
						}
					}
				}
			}(conn)
		}
	}()
	time.Sleep(10 * time.Millisecond)
}

type syncedBuffer struct {
	buff bytes.Buffer
	sync.Mutex
}

func (b *syncedBuffer) Write(data []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.buff.Write(data)
}

func (b *syncedBuffer) String() string {
	b.Lock()
	defer b.Unlock()
	return b.buff.String()
}
