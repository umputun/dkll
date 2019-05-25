package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestCli(t *testing.T) {

	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}}, DisplayParams{Out: &out})
	err := c.Activate(core.Request{})
	require.NoError(t, err)

	assert.Equal(t, "h1:c1 - msg1\nh1:c2 - msg2\nh2:c1 - msg3\nh1:c1 - msg4\nh1:c2 - msg5\nh2:c2 - msg6\n", out.String())
}

func TestCliWithPidAndTS(t *testing.T) {

	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}

	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}}, DisplayParams{Out: &out, ShowPid: true, ShowTs: true})
	err := c.Activate(core.Request{})
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
	err = c.Activate(core.Request{})
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
	err := c.Activate(core.Request{})
	require.NoError(t, err)
	assert.Equal(t, "h1:c2 - msg5\n", out.String())
}

func TestCliWithUnGrep(t *testing.T) {
	ts := prepTestServer(t)
	defer ts.Close()

	out := bytes.Buffer{}
	c := NewCLI(APIParams{API: ts.URL + "/v1", Client: &http.Client{}}, DisplayParams{Out: &out, UnGrep: []string{"msg5"}})
	err := c.Activate(core.Request{})
	require.NoError(t, err)
	assert.Equal(t, "h1:c1 - msg1\nh1:c2 - msg2\nh2:c1 - msg3\nh1:c1 - msg4\nh2:c2 - msg6\n", out.String())
}

func prepTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

}
