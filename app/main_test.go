package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/globalsign/mgo"
	"github.com/go-pkgz/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestServer(t *testing.T) {
	log.Printf("start server test")
	mg, err := mongo.NewServer(mgo.DialInfo{Addrs: []string{"127.0.0.1"}, Database: "test"}, mongo.ServerParams{})
	require.NoError(t, err)
	mgconn := mongo.NewConnection(mg, "test", "msgs")
	os.Args = []string{"dkll", "server", "--dbg", "--mongo=mongo:27017", "--mongo-db=test",
		"--backup=/tmp/dkll-test", "--merged", "--syslog-port=15514"}
	defer func() {
		os.RemoveAll("/tmp/dkll-test")
		mgconn.WithCollection(func(coll *mgo.Collection) error {
			return coll.DropCollection()
		})
	}()

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
