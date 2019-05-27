package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-pkgz/repeater/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestCli(t *testing.T) {

	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}, UpdateInterval: 10 * time.Millisecond}, DisplayParams{Out: &out})
	req := core.Request{}
	r1, err := c.Activate(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "h1:c1 - msg1\nh1:c2 - msg2\nh2:c1 - msg3\nh1:c1 - msg4\nh1:c2 - msg5\nh2:c2 - msg6\n", out.String())
	assert.Equal(t, "5ce8718aef1d7346a5443a6f", r1.LastID)
}

func TestCliWithPidAndTS(t *testing.T) {

	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}}, DisplayParams{Out: &out, ShowPid: true, ShowTs: true})
	_, err := c.Activate(context.Background(), core.Request{})
	require.NoError(t, err)
	exp := "h1:c1 - 2019-05-24 20:54:30 [0] - msg1\nh1:c2 - 2019-05-24 20:54:31 [0] - msg2\n" +
		"h2:c1 - 2019-05-24 20:54:32 [0] - msg3\nh1:c1 - 2019-05-24 20:54:33 [0] - msg4\n" +
		"h1:c2 - 2019-05-24 20:54:34 [0] - msg5\nh2:c2 - 2019-05-24 20:54:35 [0] - msg6\n"
	assert.Equal(t, exp, out.String())
}

func TestCliWithCustomTZ(t *testing.T) {

	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}

	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}}, DisplayParams{Out: &out, TimeZone: tz, ShowTs: true})
	_, err = c.Activate(context.Background(), core.Request{})
	require.NoError(t, err)
	exp := "h1:c1 - 2019-05-24 21:54:30 - msg1\nh1:c2 - 2019-05-24 21:54:31 - msg2\nh2:c1 - 2019-05-24 21:54:32 - msg3\n" +
		"h1:c1 - 2019-05-24 21:54:33 - msg4\nh1:c2 - 2019-05-24 21:54:34 - msg5\nh2:c2 - 2019-05-24 21:54:35 - msg6\n"
	assert.Equal(t, exp, out.String())
}

func TestCliWithGrep(t *testing.T) {
	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}
	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}}, DisplayParams{Out: &out, Grep: []string{"msg5"}})
	_, err := c.Activate(context.Background(), core.Request{})
	require.NoError(t, err)
	assert.Equal(t, "h1:c2 - msg5\n", out.String())
}

func TestCliWithUnGrep(t *testing.T) {
	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}
	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}}, DisplayParams{Out: &out, UnGrep: []string{"msg5"}})
	_, err := c.Activate(context.Background(), core.Request{})
	require.NoError(t, err)
	assert.Equal(t, "h1:c1 - msg1\nh1:c2 - msg2\nh2:c1 - msg3\nh1:c1 - msg4\nh2:c2 - msg6\n", out.String())
}

func TestLastID(t *testing.T) {

	var count int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/last" && r.Method == "GET" {
			if atomic.LoadInt64(&count) > 0 {
				w.WriteHeader(400)
				return
			}
			ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
			rec := core.LogEntry{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1",
				Msg: "msg1", Ts: ts.Add(0 * time.Second)}

			err := json.NewEncoder(w).Encode(&rec)
			require.NoError(t, err)
			atomic.AddInt64(&count, 1)
		}
	}))
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{},
		RepeaterStrategy: &strategy.Once{}}, DisplayParams{Out: &out})
	id, err := c.getLastID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "5ce8718aef1d7346a5443a1f", id)

	// second call will fail due to test setup
	_, err = c.getLastID(context.Background())
	require.NotNil(t, err)
}

func TestCliFindFailedAndRestored(t *testing.T) {
	var count int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if atomic.AddInt64(&count, 1) < 5 { // fail first 5 calls
			w.WriteHeader(400)
			return
		}

		if atomic.AddInt64(&count, 1) > 6 { // stop on call 6
			err := json.NewEncoder(w).Encode([]core.LogEntry{})
			require.NoError(t, err)
			return
		}

		if r.URL.Path == "/v1/find" && r.Method == "POST" {
			ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
			recs := []core.LogEntry{
				{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1", Msg: "msg1", Ts: ts.Add(0 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a2f", Host: "h1", Container: "c2", Msg: "msg2", Ts: ts.Add(1 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a3f", Host: "h2", Container: "c1", Msg: "msg3", Ts: ts.Add(2 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a4f", Host: "h1", Container: "c1", Msg: "msg4", Ts: ts.Add(3 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a5f", Host: "h1", Container: "c2", Msg: "msg5", Ts: ts.Add(4 * time.Second)},
				{ID: "5ce8718aef1d7346a5443a6f", Host: "h2", Container: "c2", Msg: "msg6", Ts: ts.Add(5 * time.Second)},
			}
			err := json.NewEncoder(w).Encode(recs)
			require.NoError(t, err)
		}
	}))
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{},
		RepeaterStrategy: &strategy.FixedDelay{Repeats: 10, Delay: 1 * time.Millisecond}}, DisplayParams{Out: &out})
	_, err := c.Activate(context.Background(), core.Request{})
	require.NoError(t, err)
	assert.Equal(t, "h1:c1 - msg1\nh1:c2 - msg2\nh2:c1 - msg3\nh1:c1 - msg4\nh1:c2 - msg5\nh2:c2 - msg6\n", out.String())
	assert.Equal(t, int64(8), atomic.LoadInt64(&count), "called 5 times by repeater")
}

func TestCliFindWrongResponse(t *testing.T) {
	var count int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/find" && r.Method == "POST" {
			_, _ = fmt.Fprint(w, "blah")
		}
		atomic.AddInt64(&count, 1)
	}))
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{},
		RepeaterStrategy: &strategy.FixedDelay{Repeats: 10, Delay: 1 * time.Millisecond}}, DisplayParams{Out: &out})
	_, err := c.Activate(context.Background(), core.Request{})
	assert.NotNil(t, err)
	assert.Equal(t, int64(10), atomic.LoadInt64(&count), "called 10 times by repeater")
}

func TestCliFindFollow(t *testing.T) {
	var count int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if atomic.LoadInt64(&count) > 5 {
			require.NoError(t, json.NewEncoder(w).Encode([]core.LogEntry{}))
			return
		}
		if r.URL.Path == "/v1/find" && r.Method == "POST" {
			c := atomic.LoadInt64(&count)
			ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
			recs := []core.LogEntry{
				{
					ID:   fmt.Sprintf("5ce8718aef1d7346a5443a1%d", c),
					Host: "h1", Container: "c1", Msg: fmt.Sprintf("msg%d", c),
					Ts: ts.Add(time.Duration(c) * time.Second),
				},
			}
			require.NoError(t, json.NewEncoder(w).Encode(recs))
		}
		atomic.AddInt64(&count, 1)
	}))
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{},
		RepeaterStrategy: &strategy.FixedDelay{Repeats: 1, Delay: 1 * time.Millisecond}}, DisplayParams{Out: &out, FollowMode: true})
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond*500, cancel)
	_, err := c.Activate(ctx, core.Request{})
	assert.EqualError(t, err, "context canceled")
	assert.Equal(t, "h1:c1 - msg0\nh1:c1 - msg1\nh1:c1 - msg2\nh1:c1 - msg3\nh1:c1 - msg4\nh1:c1 - msg5\n", out.String())
	assert.Equal(t, int64(6), atomic.LoadInt64(&count), "called 6 times by repeater")
}

func TestCliFindTail(t *testing.T) {

	var count int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if atomic.LoadInt64(&count) > 5 {
			require.NoError(t, json.NewEncoder(w).Encode([]core.LogEntry{}))
			return
		}

		if r.URL.Path == "/v1/last" && r.Method == "GET" {
			ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
			rec := core.LogEntry{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1",
				Msg: "msg1", Ts: ts.Add(5 * time.Second)}
			err := json.NewEncoder(w).Encode(&rec)
			require.NoError(t, err)
		}

		if r.URL.Path == "/v1/find" && r.Method == "POST" {
			if atomic.AddInt64(&count, 1) > 1 {
				var recs []core.LogEntry
				err := json.NewEncoder(w).Encode(recs)
				require.NoError(t, err)
				return
			}
			req := core.Request{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, "5ce8718aef1d7346a5443a1f", req.LastID)
			ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
			recs := []core.LogEntry{{ID: "5ce8718aef1d7346a5443a12f", Host: "h1", Container: "c1",
				Msg: "msg1", Ts: ts.Add(15 * time.Second)}}
			require.NoError(t, json.NewEncoder(w).Encode(recs))
		}
	}))
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{},
		RepeaterStrategy: &strategy.FixedDelay{Repeats: 1, Delay: 1 * time.Millisecond}}, DisplayParams{Out: &out, TailMode: true})
	_, err := c.Activate(context.Background(), core.Request{})
	assert.NoError(t, err)
	assert.Equal(t, "h1:c1 - msg1\n", out.String())
}

func TestCliFindTailFailed(t *testing.T) {

	var count int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if atomic.LoadInt64(&count) > 5 {
			require.NoError(t, json.NewEncoder(w).Encode([]core.LogEntry{}))
			return
		}

		if r.URL.Path == "/v1/last" && r.Method == "GET" {
			w.WriteHeader(500)
			return
		}

		if r.URL.Path == "/v1/find" && r.Method == "POST" {
			t.Fatal("should not be called")
		}
	}))
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{},
		RepeaterStrategy: &strategy.FixedDelay{Repeats: 1, Delay: 1 * time.Millisecond}}, DisplayParams{Out: &out, TailMode: true})
	_, err := c.Activate(context.Background(), core.Request{})
	assert.NotNil(t, err)
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
