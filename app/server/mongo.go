package server

import (
	"strings"
	"sync"
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
	MongoParams

	lastPublished struct {
		entry core.LogEntry
		sync.Mutex
	}
}

// MongoParams has all inputs (except dial info) needed to initialize mongo store
type MongoParams struct {
	MaxDocs            int
	MaxCollectionSize  int
	Delay              time.Duration
	DBName, Collection string
}

const (
	defMaxDocs           = 100000000               // 100 Millions
	defMaxCollectionSize = 10 * 1024 * 1024 * 1024 // 10G
)

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
func NewMongo(dial mgo.DialInfo, params MongoParams) (res *Mongo, err error) {
	log.Printf("[INFO] make new mongo server with dial=%+v, %+v", dial, params)
	mg, err := mongo.NewServer(dial, mongo.ServerParams{Delay: int(params.Delay.Seconds()), ConsistencyMode: mgo.Monotonic})
	if err != nil {
		return nil, err
	}

	if params.MaxCollectionSize == 0 {
		params.MaxCollectionSize = defMaxCollectionSize
	}
	if params.MaxDocs == 0 {
		params.MaxDocs = defMaxDocs
	}

	res = &Mongo{Connection: mongo.NewConnection(mg, params.DBName, params.Collection), MongoParams: params}
	if err := res.init(params.Collection); err != nil {
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
	if len(records) > 0 {
		m.lastPublished.Lock()
		m.lastPublished.entry = records[len(records)-1]
		m.lastPublished.Unlock()
	}
	return err
}

// LastPublished returns latest published entry
func (m *Mongo) LastPublished() (entry core.LogEntry, err error) {

	cachedLast := func() (e core.LogEntry, ok bool) {
		m.lastPublished.Lock()
		e = m.lastPublished.entry
		m.lastPublished.Unlock()
		return e, e.ID != ""
	}

	if e, ok := cachedLast(); ok {
		return e, nil
	}

	var mentry mongoLogEntry
	err = m.WithCollection(func(coll *mgo.Collection) error {
		return coll.Find(bson.M{}).Sort("-_id").Limit(1).One(&mentry)
	})
	return m.makeLogEntry(mentry), err
}

// Find records matching given request
func (m *Mongo) Find(req core.Request) ([]core.LogEntry, error) {

	// eliminate mongo find if lastPublished ID < req.LastID
	m.lastPublished.Lock()
	lastPublishedCached := m.lastPublished.entry
	m.lastPublished.Unlock()
	if req.LastID != "" && lastPublishedCached.ID != "" && req.LastID >= lastPublishedCached.ID {
		return []core.LogEntry{}, nil
	}

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

// init Collection, make/ensure indexes
func (m *Mongo) init(collection string) error {
	log.Printf("[INFO] create Collection %s", collection)

	indexes := []mgo.Index{
		{Key: []string{"host", "container", "ts"}},
		{Key: []string{"ts", "host", "container"}},
		{Key: []string{"container", "ts"}},
	}

	err := m.WithDB(func(dbase *mgo.Database) error {
		coll := dbase.C(collection)
		e := coll.Create(&mgo.CollectionInfo{ForceIdIndex: true, Capped: true, MaxBytes: m.MaxCollectionSize, MaxDocs: m.MaxDocs})
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

	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
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
