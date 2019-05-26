package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, req, ds.req)
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

type mockDataService struct {
	req core.Request
}

func (m *mockDataService) Find(req core.Request) ([]core.LogEntry, error) {
	m.req = req
	if req.LastID == "err" {
		return nil, errors.New("the error")
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
	return recs, nil
}

func (m *mockDataService) LastPublished() (entry core.LogEntry, err error) {
	ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
	return core.LogEntry{ID: "5ce8718aef1d7346a5443a6f", Host: "h2", Container: "c2", Msg: "msg6", Ts: ts.Add(5 * time.Second)}, nil
}
