package core

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEntry(t *testing.T) {

	year := time.Now().Year()

	tz, err := time.LoadLocation("America/Chicago")
	require.NoError(t, err)

	tbl := []struct {
		inp string
		out LogEntry
		err error
	}{

		{"", LogEntry{}, errors.New("line is too short, line=[]")},
		{"12345", LogEntry{}, errors.New("line is too short, line=[12345]")},
		{"Oct 19 95:29:43 sniper-mgd-1 docker/mongo[888]: 2015-10", LogEntry{},
			errors.New("can't extract time from \"Oct 19 95:29:43 sniper-mgd-1 docker/mongo[888]: 2015-10\": parsing time \"Oct\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"Oct\" as \"2006\"")},

		{
			"Oct 19 15:29:43 mgd-server-1 docker/mongo[888]: 2016-10-19T19:29:43.236Z I NETWORK " +
				" [conn453005] end connection 172.17.42.1:35981 (18 connections now open)",
			LogEntry{Host: "mgd-server-1", Container: "mongo", Pid: 888, Ts: time.Date(time.Now().Year(), 10, 19, 15, 29, 43, 0, tz),
				Msg: "2016-10-19T19:29:43.236Z I NETWORK  [conn453005] end connection 172.17.42.1:35981 (18 connections now open)"},
			nil,
		},

		{
			"May 30 16:49:03 host-dev dhclient: DHCPACK of 10.0.2.54 from 10.0.2.1",
			LogEntry{Host: "host-dev", Container: "syslog", Pid: 0, Ts: time.Date(time.Now().Year(), 5, 30, 16, 49, 3, 0, tz),
				Msg: "DHCPACK of 10.0.2.54 from 10.0.2.1"},
			nil,
		},

		{
			"2017-05-30T16:13:35-04:00 BigMac.local docker/test123[63415]: 2017/05/30 16:13:35 tail-dynamic 0a7aed6",
			LogEntry{Host: "BigMac.local", Container: "test123", Pid: 63415, Ts: time.Date(2017, 5, 30, 15, 13, 35, 0, tz),
				Msg: "2017/05/30 16:13:35 tail-dynamic 0a7aed6"},
			nil,
		},

		{
			`May 30 18:03:27 dev-1 docker[1187]: time="2017-05-30T18:03:27-04:00" level=info msg="Firewalld running: false"`,
			LogEntry{Host: "dev-1", Container: "syslog", Pid: 0, Ts: time.Date(year, 5, 30, 18, 3, 27, 0, tz),
				Msg: `time="2017-05-30T18:03:27-04:00" level=info msg="Firewalld running: false"`},
			nil,
		},

		{
			`May 30 18:03:27 dev-1 docker[1187]: 2017/10/02 04:05:24.509511 [INFO]  logger.go:106: REST umputun/demo GET/exclusions/xyz?psrc=abc - 192.168.1.33 - 200 (157) - 63.183µs  ->28ed7948-5349-4b3f-a5b8-dadc713df3ae`,
			LogEntry{Host: "dev-1", Container: "syslog", Pid: 0, Ts: time.Date(year, 5, 30, 18, 3, 27, 0, tz),
				Msg: `2017/10/02 04:05:24.509511 [INFO]  logger.go:106: REST umputun/demo GET/exclusions/xyz?psrc=abc - 192.168.1.33 - 200 (157) - 63.183µs  ->28ed7948-5349-4b3f-a5b8-dadc713df3ae`},
			nil,
		},
	}

	for n, tt := range tbl {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			logEntry, err := NewEntry(tt.inp, tz)
			if tt.err != nil {
				assert.NotNil(t, err, fmt.Sprintf("expects error in #%d", n))
				assert.EqualError(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
			logEntry.ID = "5927d382b2035078e61816a5"
			tt.out.ID = "5927d382b2035078e61816a5"
			assert.Equal(t, tt.out.Host, logEntry.Host, fmt.Sprintf("mismatch in #%d", n))
			assert.Equal(t, tt.out.Container, logEntry.Container, fmt.Sprintf("mismatch in #%d", n))
			assert.Equal(t, tt.out.Msg, logEntry.Msg, fmt.Sprintf("mismatch in #%d", n))
			assert.Equal(t, tt.out.Pid, logEntry.Pid, fmt.Sprintf("mismatch in #%d", n))
			assert.Equal(t, tt.out.ID, logEntry.ID, fmt.Sprintf("mismatch in #%d", n))
			assert.Equal(t, tt.out.Ts.Format(time.RFC3339), logEntry.Ts.Format(time.RFC3339), fmt.Sprintf("mismatch in #%d", n))
		})
	}
}
