package server

import (
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/mongo"
	"github.com/pkg/errors"

	"github.com/umputun/dkll/app/core"
)

// Mongo store provides all mongo-related ops
type Mongo struct {
	*mongo.Connection
}

type mongoLogEntry struct {
	ID        bson.ObjectId `bson:"_id,omitempty"`
	Host      string        `bson:"host"`
	Container string        `bson:"container"`
	Pid       int           `bson:"pid"`
	Msg       string        `bson:"msg"`
	Ts        time.Time     `bson:"ts"`
	CreatedTs time.Time     `bson:"cts"`
}

// NewMongo makes Mongo accessor
func NewMongo(dial mgo.DialInfo, delay time.Duration, dbName string, collection string) (res *Mongo, err error) {
	log.Printf("[INFO] make new mongo server with dial=%+v, db=%s, delay=%v", dial, dbName, delay)
	mg, err := mongo.NewServer(dial, mongo.ServerParams{Delay: int(delay.Seconds()), ConsistencyMode: mgo.Monotonic})
	if err != nil {
		return nil, err
	}
	res = &Mongo{Connection: mongo.NewConnection(mg, dbName, collection)}
	if err = res.init(collection); err != nil {
		return nil, err
	}
	return res, nil
}

// Publish inserts buffer to mongo
func (m *Mongo) Publish(records []core.LogEntry) (err error) {
	recs := make([]interface{}, len(records))
	for i, v := range records {
		recs[i] = m.makeMongoEntry(v)
	}
	err = m.WithCollection(func(coll *mgo.Collection) error {
		return coll.Insert(recs...)
	})
	return err
}

// LastPublished returns latest published entry
func (m *Mongo) LastPublished() (entry core.LogEntry, err error) {
	var mentry mongoLogEntry
	err = m.WithCollection(func(coll *mgo.Collection) error {
		return coll.Find(bson.M{}).Sort("-_id").Limit(1).One(&mentry)
	})
	return m.makeLogEntry(mentry), err
}

// Find records matching given request
func (m *Mongo) Find(req core.Request) ([]core.LogEntry, error) {

	query := m.makeQuery(req)

	var mresult []mongoLogEntry
	err := m.WithCollection(func(coll *mgo.Collection) error {
		return coll.Find(query).Sort("+_id").All(&mresult)
	})
	if err != nil {
		return nil, errors.Wrapf(err, "can't get records for %+v", req)
	}

	result := make([]core.LogEntry, len(mresult))
	for i, r := range mresult {
		result[i] = m.makeLogEntry(r)
	}
	log.Printf("[DEBUG] req: %+v, recs=%d", req, len(result))
	return result, nil
}

func (m *Mongo) makeQuery(req core.Request) (b bson.M) {

	fromTS := time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local)
	if !req.FromTS.IsZero() {
		fromTS = req.FromTS
	}

	toTS := time.Date(2100, 1, 1, 0, 0, 0, 0, time.Local)
	if !req.ToTS.IsZero() {
		toTS = req.ToTS
	}
	query := bson.M{"_id": bson.M{"$gt": m.getBid(req.LastID)}, "ts": bson.M{"$gte": fromTS, "$lt": toTS}}

	if req.Containers != nil && len(req.Containers) > 0 {
		query["container"] = bson.M{"$in": m.convertListWithRegex(req.Containers)}
	}

	if req.Hosts != nil && len(req.Hosts) > 0 {
		query["host"] = bson.M{"$in": m.convertListWithRegex(req.Hosts)}
	}

	if req.Excludes != nil && len(req.Excludes) > 0 {
		if val, found := query["container"]; found {
			val.(bson.M)["$nin"] = m.convertListWithRegex(req.Excludes)
		} else {
			query["container"] = bson.M{"$nin": m.convertListWithRegex(req.Excludes)}
		}
	}

	return query
}

func (m *Mongo) convertListWithRegex(elems []string) []interface{} {
	var result []interface{}
	for _, elem := range elems {
		if strings.HasPrefix(elem, "/") && strings.HasSuffix(elem, "/") {
			result = append(result, bson.RegEx{Pattern: elem[1 : len(elem)-1], Options: ""})
		} else {
			result = append(result, elem)
		}
	}
	return result
}

func (m *Mongo) getBid(id string) bson.ObjectId {
	bid := bson.ObjectId("000000000000")
	if id != "0" && bson.IsObjectIdHex(id) {
		bid = bson.ObjectIdHex(id)
	}
	return bid
}

// init collection, make/ensure indexes
func (m *Mongo) init(collection string) error {
	log.Printf("[INFO] create collection %s", collection)

	indexes := []mgo.Index{
		{Key: []string{"host", "container", "ts"}},
		{Key: []string{"ts", "host", "container"}},
		{Key: []string{"container", "ts"}},
	}

	err := m.WithDB(func(dbase *mgo.Database) error {
		coll := dbase.C(collection)
		e := coll.Create(&mgo.CollectionInfo{ForceIdIndex: true, Capped: true, MaxBytes: 50000000000, MaxDocs: 500000000})
		if e != nil {
			return e
		}
		for _, index := range indexes {
			if err := coll.EnsureIndex(index); err != nil {
				log.Printf("[WARN] can't insure index %v, %v", index, err)
			}
		}
		return nil
	})

	return err
}

func (m *Mongo) makeMongoEntry(entry core.LogEntry) mongoLogEntry {
	res := mongoLogEntry{
		ID:        m.getBid(entry.ID),
		Host:      entry.Host,
		Container: entry.Container,
		Msg:       entry.Msg,
		Ts:        entry.Ts,
		CreatedTs: entry.CreatedTs,
		Pid:       entry.Pid,
	}
	if entry.ID == "" {
		res.ID = bson.NewObjectId()
	}
	return res
}

func (m *Mongo) makeLogEntry(entry mongoLogEntry) core.LogEntry {
	return core.LogEntry{
		ID:        entry.ID.Hex(),
		Host:      entry.Host,
		Container: entry.Container,
		Msg:       entry.Msg,
		Ts:        entry.Ts,
		CreatedTs: entry.CreatedTs,
		Pid:       entry.Pid,
	}
}
