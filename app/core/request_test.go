package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequest_String(t *testing.T) {
	r := Request{
		LastID:     "111",
		Hosts:      []string{"h1, h2"},
		Containers: []string{"c1", "c2", "c3"},
		Excludes:   []string{"monit"},
		Limit:      1000,
		FromTS:     time.Date(2019, 5, 25, 2, 57, 45, 0, time.UTC),
		ToTS:       time.Date(2019, 5, 25, 6, 57, 45, 0, time.UTC),
	}
	assert.Equal(t, "hosts=[h1, h2], containers=[c1 c2 c3], excludes=[monit], max=1000, from=2019-05-25T02:57:45Z, to=2019-05-25T06:57:45Z, last-id=111", r.String())
}
