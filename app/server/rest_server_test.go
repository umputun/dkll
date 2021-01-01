package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestRest_Run(t *testing.T) {
	ds := &mockDataService{}
	srv := RestServer{DataService: ds, Port: 10080}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond*100, cancel)
	err := srv.Run(ctx)
	assert.EqualError(t, err, "http: Server closed")
}

func TestRest_findCtrl(t *testing.T) {
	ds := &mockDataService{}
	srv := RestServer{DataService: ds}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	buff := bytes.Buffer{}
	req := core.Request{Hosts: []string{"xyz"}}
	err := json.NewEncoder(&buff).Encode(req)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/v1/find", "application/json", &buff)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, req, ds.getReq())
	var recs []core.LogEntry
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&recs))
	assert.Equal(t, 6, len(recs))
	assert.Equal(t, "5ce8718aef1d7346a5443a1f", recs[0].ID)
	assert.Equal(t, "5ce8718aef1d7346a5443a6f", recs[5].ID)
}

func TestRest_findCtrlFailed(t *testing.T) {
	ds := &mockDataService{}
	srv := RestServer{DataService: ds}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	buff := bytes.Buffer{}
	req := core.Request{LastID: "err"} // trigger error
	err := json.NewEncoder(&buff).Encode(req)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/v1/find", "application/json", &buff)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	resp, err = http.Post(ts.URL+"/v1/find", "application/json", nil)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestRest_lastCtrl(t *testing.T) {
	ds := &mockDataService{}
	srv := RestServer{DataService: ds}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	rec := core.LogEntry{}
	resp, err := http.Get(ts.URL + "/v1/last")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rec))
	assert.Equal(t, "5ce8718aef1d7346a5443a6f", rec.ID)
}

func TestRest_streamCtrl(t *testing.T) {
	ds := &mockDataService{maxRepeats: 10}
	srv := RestServer{DataService: ds, StreamDuration: 10 * time.Millisecond}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	buff := bytes.Buffer{}
	req := core.Request{Hosts: []string{"xyz"}}
	err := json.NewEncoder(&buff).Encode(req)
	require.NoError(t, err)

	st := time.Now()

	resp, err := http.Post(ts.URL+"/v1/stream?timeout=200ms", "application/json", &buff)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, req, ds.getReq())

	data, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, time.Since(st) >= time.Millisecond*600, "4 responses slowed by 100 each and + 200ms timeout")
	t.Logf("since=%v", time.Since(st))
	t.Log(string(data))
	recs := strings.Split(string(data), "\n")
	assert.Equal(t, 6*9+1, len(recs), "got 9 chunks")
	assert.Equal(t, `{"id":"5ce8718aef1d7346a5443a1f","host":"h1","container":"c1","pid":0,"msg":"msg1","ts":"2019-05-24T20:54:30-05:00","cts":"0001-01-01T00:00:00Z"}`,
		recs[0])
	assert.Equal(t, `{"id":"5ce8718aef1d7346a5443a6f","host":"h2","container":"c2","pid":0,"msg":"msg6","ts":"2019-05-24T21:03:35-05:00","cts":"0001-01-01T00:00:00Z"}`,
		recs[53])

}

type mockDataService struct {
	req struct {
		sync.Mutex
		v core.Request
	}
	maxRepeats int32
	repeats    int32
}

func (m *mockDataService) Find(req core.Request) ([]core.LogEntry, error) {
	m.req.Lock()
	m.req.v = req
	m.req.Unlock()

	if req.LastID == "err" {
		return nil, errors.New("the error")
	}

	if m.maxRepeats > 0 && atomic.AddInt32(&m.repeats, 1) > m.maxRepeats {
		return []core.LogEntry{}, nil
	}

	if atomic.LoadInt32(&m.repeats) == 5 { // slow empty response on req #5
		time.Sleep(100 * time.Millisecond)
		return []core.LogEntry{}, nil
	}

	if atomic.LoadInt32(&m.repeats) > 5 { // slow down responses
		time.Sleep(100 * time.Millisecond)
	}

	ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
	tsOffset := time.Minute * time.Duration(atomic.LoadInt32(&m.repeats)-1)
	recs := []core.LogEntry{
		{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1", Msg: "msg1", TS: ts.Add(0*time.Second + tsOffset)},
		{ID: "5ce8718aef1d7346a5443a2f", Host: "h1", Container: "c2", Msg: "msg2", TS: ts.Add(1*time.Second + tsOffset)},
		{ID: "5ce8718aef1d7346a5443a3f", Host: "h2", Container: "c1", Msg: "msg3", TS: ts.Add(2*time.Second + tsOffset)},
		{ID: "5ce8718aef1d7346a5443a4f", Host: "h1", Container: "c1", Msg: "msg4", TS: ts.Add(3*time.Second + tsOffset)},
		{ID: "5ce8718aef1d7346a5443a5f", Host: "h1", Container: "c2", Msg: "msg5", TS: ts.Add(4*time.Second + tsOffset)},
		{ID: "5ce8718aef1d7346a5443a6f", Host: "h2", Container: "c2", Msg: "msg6", TS: ts.Add(5*time.Second + tsOffset)},
	}
	return recs, nil
}

func (m *mockDataService) LastPublished() (entry core.LogEntry, err error) {
	ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
	return core.LogEntry{ID: "5ce8718aef1d7346a5443a6f", Host: "h2", Container: "c2", Msg: "msg6", TS: ts.Add(5 * time.Second)}, nil
}

func (m *mockDataService) getReq() core.Request {
	m.req.Lock()
	defer m.req.Unlock()
	return m.req.v

}
