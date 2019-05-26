package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/globalsign/mgo"
	"github.com/go-pkgz/lgr"
	"github.com/go-pkgz/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestServer(t *testing.T) {
	log.Printf("start server test")
	mg, err := mongo.NewServer(mgo.DialInfo{Addrs: []string{"127.0.0.1:27017"}, Database: "test"}, mongo.ServerParams{})
	require.NoError(t, err)
	mgConn := mongo.NewConnection(mg, "test", "msgs")
	cleanupTestAssets(t, "/tmp/dkll-test", mgConn)
	os.Args = []string{"dkll", "server", "--dbg", "--mongo=127.0.0.1:27017", "--mongo-db=test",
		"--backup=/tmp/dkll-test", "--merged", "--syslog-port=15514"}
	defer cleanupTestAssets(t, "/tmp/dkll-test", mgConn)

	go func() {
		time.Sleep(10 * time.Second)
		log.Printf("kill server")
		e := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		require.Nil(t, e)
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		st := time.Now()
		main()
		assert.True(t, time.Since(st).Seconds() >= 5, "should take about 5s")
		wg.Done()
	}()

	time.Sleep(5 * time.Second) // let server start
	log.Printf("start server checks")
	// send 2 records
	conn, err := net.Dial("tcp", "127.0.0.1:15514")
	require.NoError(t, err)
	n, err := fmt.Fprintf(conn, "2017-05-30T16:13:35-04:00 BigMac.local docker/cont1[63415]: message 123\n")
	assert.NoError(t, err)
	assert.Equal(t, 72, n)
	n, err = fmt.Fprintf(conn, "May 30 16:49:03 BigMac.local docker/cont2[63416]: message blah\n")
	assert.NoError(t, err)
	assert.Equal(t, 63, n)

	time.Sleep(2 * time.Second) // allow background writes to finish

	b, err := ioutil.ReadFile("/tmp/dkll-test/dkll.log")
	assert.NoError(t, err)
	expMerged := "2017-05-30 15:13:35 -0500 CDT : BigMac." +
		"local/cont1 [63415] - message 123\n2019-05-30 16:49:03 -0500 CDT : BigMac.local/cont2 [63416] - message blah\n"
	assert.Equal(t, expMerged, string(b))

	b, err = ioutil.ReadFile("/tmp/dkll-test/BigMac.local/cont1.log")
	assert.NoError(t, err)
	assert.Equal(t, "message 123\n", string(b))

	b, err = ioutil.ReadFile("/tmp/dkll-test/BigMac.local/cont2.log")
	assert.NoError(t, err)
	assert.Equal(t, "message blah\n", string(b))

	// check rest calls
	buff := bytes.Buffer{}
	req := core.Request{}
	require.NoError(t, json.NewEncoder(&buff).Encode(req))

	resp, err := http.Post("http://127.0.0.1:8080/v1/find", "application/json", &buff)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	defer resp.Body.Close()
	var recs []core.LogEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&recs))
	require.Equal(t, 2, len(recs))
	assert.Equal(t, "message 123", recs[0].Msg)
	assert.Equal(t, "BigMac.local", recs[0].Host)
	assert.Equal(t, "cont1", recs[0].Container)
	assert.Equal(t, 63415, recs[0].Pid)

	resp, err = http.Get("http://127.0.0.1:8080/v1/last")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	defer resp.Body.Close()
	var rec core.LogEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rec))
	assert.Equal(t, "message blah", rec.Msg)
	assert.Equal(t, "BigMac.local", rec.Host)
	assert.Equal(t, "cont2", rec.Container)
	assert.Equal(t, 63416, rec.Pid)

	wg.Wait()
	log.Printf("start wait completed")
}

func TestClient(t *testing.T) {
	ts := prepTestServer(t)
	defer ts.Close()

	os.Args = []string{"dkll", "client", "--dbg", "--api=" + ts.URL + "/v1", "--tz=America/New_York", "-m"}

	go func() {
		time.Sleep(5 * time.Second)
		log.Printf("kill client")
		e := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		require.Nil(t, e)
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)

	// disable logging
	lgr.Out(ioutil.Discard)
	defer lgr.Out(os.Stdout)
	go func() { // redirect stdout from main() to pipe
		rescueStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		main()
		_ = w.Close()
		out, _ := ioutil.ReadAll(r)
		os.Stdout = rescueStdout
		exp := "h1:c1 - 2019-05-24 21:54:30 - msg1\nh1:c2 - 2019-05-24 21:54:31 - msg2\n" +
			"h2:c1 - 2019-05-24 21:54:32 - msg3\nh1:c1 - 2019-05-24 21:54:33 - msg4\n" +
			"h1:c2 - 2019-05-24 21:54:34 - msg5\nh2:c2 - 2019-05-24 21:54:35 - msg6\n"
		assert.Equal(t, exp, string(out))
		wg.Done()
	}()

	wg.Wait()
}

func prepTestServer(t *testing.T) *httptest.Server {
	var count int64

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/find" && r.Method == "POST" {

			body, err := ioutil.ReadAll(r.Body)
			assert.NoError(t, err)
			t.Logf("request: %s", string(body))

			if atomic.AddInt64(&count, 1) > 1 {
				var recs []core.LogEntry
				err := json.NewEncoder(w).Encode(recs)
				require.NoError(t, err)
				return
			}

			ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
			recs := []core.LogEntry{
				{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1", Msg: "msg1", Ts: ts.Add(0 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a2f", Host: "h1", Container: "c2", Msg: "msg2", Ts: ts.Add(1 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a3f", Host: "h2", Container: "c1", Msg: "msg3", Ts: ts.Add(2 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a4f", Host: "h1", Container: "c1", Msg: "msg4", Ts: ts.Add(3 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a5f", Host: "h1", Container: "c2", Msg: "msg5", Ts: ts.Add(4 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a6f", Host: "h2", Container: "c2", Msg: "msg6", Ts: ts.Add(5 * time.Second)},
			}
			err = json.NewEncoder(w).Encode(recs)
			require.NoError(t, err)
			return
		}

		if r.URL.Path == "/v1/last" && r.Method == "GET" {
			ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
			rec := core.LogEntry{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1",
				Msg: "msg1", Ts: ts.Add(5 * time.Second)}
			err := json.NewEncoder(w).Encode(&rec)
			require.NoError(t, err)
		}

		w.WriteHeader(404)
	}))

}

func cleanupTestAssets(t *testing.T, loc string, conn *mongo.Connection) {
	_ = os.RemoveAll(loc)
	_ = conn.WithCollection(func(coll *mgo.Collection) error {
		return coll.DropCollection()
	})
}
