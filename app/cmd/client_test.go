package cmd

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-pkgz/lgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestClient(t *testing.T) {
	ts := prepTestServer(t)
	defer ts.Close()

	c := ClientCmd{ClientOpts{
		API:      ts.URL + "/v1",
		TimeZone: "America/New_York",
		ShowTs:   true,
	}}

	lgr.Out(ioutil.Discard)
	defer lgr.Out(os.Stdout)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second*5, cancel)

	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	e := c.Run(ctx)
	require.NoError(t, e)
	_ = w.Close()
	out, _ := ioutil.ReadAll(r)
	os.Stdout = rescueStdout
	exp := "h1:c1 - 2019-05-24 21:54:30 - msg1\nh1:c2 - 2019-05-24 21:54:31 - msg2\n" +
		"h2:c1 - 2019-05-24 21:54:32 - msg3\nh1:c1 - 2019-05-24 21:54:33 - msg4\n" +
		"h1:c2 - 2019-05-24 21:54:34 - msg5\nh2:c2 - 2019-05-24 21:54:35 - msg6\n"
	assert.Equal(t, exp, string(out))
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
