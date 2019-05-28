package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMultiWriter_Write(t *testing.T) {

	// with ext JSON
	w1, w2 := wrMock{}, wrMock{}
	writer := NewMultiWriterIgnoreErrors(&w1, &w2).WithExtJSON("c1", "g1")
	n, err := writer.Write([]byte("test 123"))
	assert.NoError(t, err)
	assert.Equal(t, 8, n)

	s1, s2 := w1.String(), w1.String()
	assert.Equal(t, s1, s2, "both dest writers have the same data")
	assert.True(t, strings.HasPrefix(w1.String(), `{"msg":"test 123"`))
	t.Log(s1)

	// without ext JSON
	w1, w2 = wrMock{}, wrMock{}
	writer = NewMultiWriterIgnoreErrors(&w1, &w2)
	n, err = writer.Write([]byte("test 123"))
	assert.NoError(t, err)
	assert.Equal(t, 8, n)
	assert.Equal(t, "test 123", w1.String())
	assert.Equal(t, "test 123", w2.String())
}

func TestMultiWriter_extJSON(t *testing.T) {

	writer := NewMultiWriterIgnoreErrors().WithExtJSON("c1", "g1")
	res, err := writer.extJSON([]byte("test msg"))
	assert.NoError(t, err)

	j := jMsg{}
	err = json.Unmarshal(res, &j)
	assert.NoError(t, err)

	assert.Equal(t, "test msg", j.Msg)
	assert.Equal(t, "c1", j.Container)
	assert.Equal(t, "g1", j.Group)

	hname, err := os.Hostname()
	assert.NoError(t, err)
	assert.Equal(t, hname, j.Host)
	assert.True(t, time.Since(j.TS).Seconds() < 1)
}

func TestMultiWriter_Close(t *testing.T) {
	w1, w2 := wrMock{}, wrMock{failOnClose: true}
	writer := NewMultiWriterIgnoreErrors(&w1, &w2)
	assert.EqualError(t, writer.Close(), "1 error occurred:\n\t* close failed\n\n")

	w1, w2 = wrMock{}, wrMock{}
	writer = NewMultiWriterIgnoreErrors(&w1, &w2)
	assert.NoError(t, writer.Close())
}

type wrMock struct {
	bytes.Buffer
	failOnClose bool
}

func (m *wrMock) Close() error {
	if m.failOnClose {
		return errors.New("close failed")
	}
	return nil
}
