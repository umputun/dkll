package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/mongo/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestServer(t *testing.T) {
	log.Printf("start server test")

	_, coll, teardown := mongo.MakeTestConnection(t)
	defer teardown()
	// m, err := server.NewMongo(mg, server.MongoParams{DBName: "test", Collection: coll.Name()})
	// require.NoError(t, err)

	os.RemoveAll("/tmp/dkll-test")

	opts := ServerOpts{
		Port:               8080,
		SyslogPort:         15514,
		MongoURL:           getMongoURL(t) + "?db=test&collection=" + coll.Name(),
		EnableMerged:       true,
		FileBackupLocation: "/tmp/dkll-test",
		MongoMaxDocs:       100,
	}
	s := ServerCmd{ServerOpts: opts}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		st := time.Now()
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(time.Second*5, cancel)
		e := s.Run(ctx)
		require.NoError(t, e)
		assert.True(t, time.Since(st).Seconds() >= 5, "should take about 5s")
		wg.Done()
	}()

	time.Sleep(2 * time.Second) // let server start
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
	expMerged := fmt.Sprintf("2017-05-30 15:13:35 -0500 CDT : BigMac."+
		"local/cont1 [63415] - message 123\n%d-05-30 16:49:03 -0500 CDT : BigMac.local/cont2 [63416] - message blah\n",
		time.Now().Year())
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

func getMongoURL(t *testing.T) string {
	mongoURL := os.Getenv("MONGO_TEST")
	if mongoURL == "" {
		mongoURL = "mongodb://mongo:27017"
		t.Logf("no MONGO_TEST in env, defaulted to %s", mongoURL)
	}
	if mongoURL == "skip" {
		t.Skip("skip mongo test")
	}
	return mongoURL
}
