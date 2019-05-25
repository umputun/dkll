package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyslog(t *testing.T) {
	s := Syslog{}
	ctx, cancel := context.WithCancel(context.Background())
	ch := s.Go(ctx)

	conn, err := net.Dial("tcp", "127.0.0.1:5514")
	require.NoError(t, err)

	time.AfterFunc(time.Millisecond*200, func() {
		assert.NoError(t, err, conn.Close())
		cancel()
	})

	n, err := fmt.Fprintf(conn, "May 30 18:03:27 dev-1 docker[1187]: 2017/10/02 04:05:24.509511 [INFO] message1\n")
	assert.NoError(t, err)
	assert.Equal(t, 79, n)

	n, err = fmt.Fprintf(conn, "May 30 18:03:28 dev-1 docker[1187]: 2017/10/02 04:05:24 [INFO] message2\n")
	assert.NoError(t, err)
	assert.Equal(t, 72, n)

	assert.Equal(t, "May 30 18:03:27 dev-1 docker[1187]: 2017/10/02 04:05:24.509511 [INFO] message1", <-ch)
	assert.Equal(t, "May 30 18:03:28 dev-1 docker[1187]: 2017/10/02 04:05:24 [INFO] message2", <-ch)
	time.Sleep(time.Millisecond * 400)
}
