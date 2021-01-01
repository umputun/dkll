package server

import (
	"context"
	"testing"
	"time"

	"github.com/go-pkgz/mongo/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/dkll/app/core"
)

func TestMongo_LastPublished(t *testing.T) {

	mg, coll, teardown := mongo.MakeTestConnection(t)
	defer teardown()
	m, err := NewMongo(mg, MongoParams{DBName: "test", Collection: coll.Name()})
	require.NoError(t, err)

	_, err = m.LastPublished()
	assert.NoError(t, err)

	ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
	recs := []core.LogEntry{
		{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1", Msg: "msg1", Ts: ts.Add(0 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a2f", Host: "h1", Container: "c2", Msg: "msg2", Ts: ts.Add(1 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a3f", Host: "h2", Container: "c1", Msg: "msg3", Ts: ts.Add(2 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a4f", Host: "h1", Container: "c1", Msg: "msg4", Ts: ts.Add(3 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a5f", Host: "h1", Container: "c2", Msg: "msg5", Ts: ts.Add(4 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a6f", Host: "h2", Container: "c2", Msg: "msg6", Ts: ts.Add(5 * time.Second)},
	}
	assert.NoError(t, m.Publish(recs))

	rec, err := m.LastPublished()
	assert.NoError(t, err)
	assert.Equal(t, "msg6", rec.Msg, "last record with msg6")
}

func TestMongo_Find(t *testing.T) {
	mg, coll, teardown := mongo.MakeTestConnection(t)
	defer teardown()
	m, err := NewMongo(mg, MongoParams{DBName: "test", Collection: coll.Name()})
	require.NoError(t, err)

	ts := time.Date(2019, 5, 24, 20, 54, 30, 0, time.Local)
	recs := []core.LogEntry{
		{ID: "5ce8718aef1d7346a5443a1f", Host: "h1", Container: "c1", Msg: "msg1", Ts: ts.Add(0 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a2f", Host: "h1", Container: "c2", Msg: "msg2", Ts: ts.Add(1 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a3f", Host: "h2", Container: "c1", Msg: "msg3", Ts: ts.Add(2 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a4f", Host: "h1", Container: "c1", Msg: "msg4", Ts: ts.Add(3 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a5f", Host: "h1", Container: "c2", Msg: "msg5", Ts: ts.Add(4 * time.Second)},
		{ID: "5ce8718aef1d7346a5443a6f", Host: "h2", Container: "c2", Msg: "msg6", Ts: ts.Add(5 * time.Second)},
	}
	assert.NoError(t, m.Publish(recs))

	recs, err = m.Find(core.Request{})
	assert.NoError(t, err)
	assert.Equal(t, 6, len(recs), "no-filter, all records")
	assert.Equal(t, "msg1", recs[0].Msg)
	assert.Equal(t, "msg6", recs[5].Msg)

	recs, err = m.Find(core.Request{Limit: 3})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(recs), "3 last records")
	assert.Equal(t, "msg4", recs[0].Msg)
	assert.Equal(t, "msg5", recs[1].Msg)
	assert.Equal(t, "msg6", recs[2].Msg)

	recs, err = m.Find(core.Request{LastID: "5ce8718aef1d7346a5443a3f"})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(recs), "records after 5ce8718aef1d7346a5443a3f")
	assert.Equal(t, "5ce8718aef1d7346a5443a4f", recs[0].ID, "find with last-id")

	recs, err = m.Find(core.Request{Hosts: []string{"h1"}, Containers: []string{"c1"}})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(recs), "records for host h1 and container c1")
	assert.Equal(t, "h1", recs[0].Host)
	assert.Equal(t, "h1", recs[1].Host)
	assert.Equal(t, "c1", recs[0].Container)
	assert.Equal(t, "c1", recs[1].Container)

	recs, err = m.Find(core.Request{FromTS: ts.Add(1 * time.Second), ToTS: ts.Add(4 * time.Second)})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(recs), "time interval")
	assert.Equal(t, ts.Add(1*time.Second), recs[0].Ts.In(time.Local))
	assert.Equal(t, ts.Add(3*time.Second), recs[2].Ts.In(time.Local))

	recs, err = m.Find(core.Request{Excludes: []string{"c2"}})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(recs), "exclude container c2")
	assert.Equal(t, "c1", recs[0].Container)
	assert.Equal(t, "c1", recs[1].Container)
	assert.Equal(t, "c1", recs[2].Container)

	recs, err = m.Find(core.Request{Excludes: []string{"c2"}, Containers: []string{"/c/"}})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(recs), "exclude container c2")
	assert.Equal(t, "c1", recs[0].Container)
	assert.Equal(t, "c1", recs[1].Container)
	assert.Equal(t, "c1", recs[2].Container)

	recs = []core.LogEntry{
		{ID: "5ce8718aef1d7346a5443b1f", Host: "hh1", Container: "c1", Msg: "msg1", Ts: ts.Add(0 * time.Second)},
		{ID: "5ce8718aef1d7346a5443b2f", Host: "hh22", Container: "c2", Msg: "msg2", Ts: ts.Add(1 * time.Second)},
		{ID: "5ce8718aef1d7346a5443b3f", Host: "hh3456", Container: "c1", Msg: "msg3", Ts: ts.Add(2 * time.Second)},
	}
	assert.NoError(t, m.Publish(recs))
	recs, err = m.Find(core.Request{Hosts: []string{"/hh/"}})
	assert.NoError(t, err, "regex hh hosts")
	assert.Equal(t, 3, len(recs))
	assert.Equal(t, "hh1", recs[0].Host)
	assert.Equal(t, "hh22", recs[1].Host)
	assert.Equal(t, "hh3456", recs[2].Host)
}

func TestMongo_FindEmpty(t *testing.T) {
	mg, coll, teardown := mongo.MakeTestConnection(t)
	defer teardown()
	m, err := NewMongo(mg, MongoParams{DBName: "test", Collection: coll.Name()})
	require.NoError(t, err)

	recs, err := m.Find(core.Request{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(recs), "no records")
	assert.Equal(t, []core.LogEntry{}, recs, "no records with empty slice")
}

func TestMongo_Init(t *testing.T) {
	mg, _, teardown := mongo.MakeTestConnection(t)
	defer teardown()
	require.NoError(t, mg.Database("test").Collection("test_msgs").Drop(context.Background()))

	_, err := NewMongo(mg, MongoParams{DBName: "test", Collection: "test_msgs"})
	require.NoError(t, err)
}
