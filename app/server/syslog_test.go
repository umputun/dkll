package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyslog(t *testing.T) {
	s := Syslog{Port: 15510}
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.Go(ctx)
	require.NoError(t, err)
	conn, err := net.Dial("tcp", "127.0.0.1:15510")
	require.NoError(t, err)

	mu := sync.Mutex{}
	time.AfterFunc(time.Millisecond*200, func() {
		mu.Lock()
		assert.NoError(t, err, conn.Close())
		mu.Unlock()
		cancel()
	})

	mu.Lock()
	n, err := fmt.Fprintf(conn, "May 30 18:03:27 dev-1 docker[1187]: 2017/10/02 04:05:24.509511 [INFO] message1\n")
	assert.NoError(t, err)
	assert.Equal(t, 79, n)

	n, err = fmt.Fprintf(conn, "May 30 18:03:28 dev-1 docker[1187]: 2017/10/02 04:05:24 [INFO] message2\n")
	assert.NoError(t, err)
	assert.Equal(t, 72, n)

	assert.Equal(t, "May 30 18:03:27 dev-1 docker[1187]: 2017/10/02 04:05:24.509511 [INFO] message1", <-ch)
	assert.Equal(t, "May 30 18:03:28 dev-1 docker[1187]: 2017/10/02 04:05:24 [INFO] message2", <-ch)
	mu.Unlock()

	time.Sleep(time.Millisecond * 400)
}

func TestSyslog_Failed(t *testing.T) {
	s := Syslog{Port: 95510}
	_, err := s.Go(context.Background())
	assert.NotNil(t, err)
	t.Log(err)
}

func TestSyslog_CancelWhileReceiving(t *testing.T) {
	// regression: shutdown closed the output channel during a send and stopped draining the handler channel
	for i := range 10 {
		t.Run(fmt.Sprintf("round %d", i), func(t *testing.T) {
			s := Syslog{Port: 15600 + i}
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			ch, err := s.Go(ctx)
			require.NoError(t, err)
			conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", s.Port))
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			go func() {
				for ctx.Err() == nil {
					if _, err := fmt.Fprintf(conn, "May 30 18:03:27 dev-1 docker[1187]: message %d\n", i); err != nil {
						return
					}
				}
			}()

			select {
			case _, ok := <-ch:
				require.True(t, ok, "syslog channel closed before any traffic")
			case <-time.After(5 * time.Second):
				t.Fatal("no syslog traffic before cancel")
			}
			cancel()

			timeout := time.After(5 * time.Second)
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-timeout:
					t.Fatal("syslog channel not closed after cancel")
				}
			}
		})
	}
}
