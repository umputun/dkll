package server

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mdrv "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/umputun/dkll/app/core"
)

// Mongo store provides all mongo-related ops
type Mongo struct {
	*mdrv.Client
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
	defaultLimit         = 1000
)

type mongoLogEntry struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Host      string             `bson:"host"`
	Container string             `bson:"container"`
	Pid       int                `bson:"pid"`
	Msg       string             `bson:"msg"`
	Ts        time.Time          `bson:"ts"`
}

// NewMongo makes Mongo accessor
func NewMongo(client *mdrv.Client, params MongoParams) (res *Mongo, err error) {
	log.Printf("[INFO] make new mongo server with %+v", params)

	if params.MaxCollectionSize == 0 {
		params.MaxCollectionSize = defMaxCollectionSize
	}
	if params.MaxDocs == 0 {
		params.MaxDocs = defMaxDocs
	}

	res = &Mongo{Client: client, MongoParams: params}
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

	coll := m.Database(m.MongoParams.DBName).Collection(m.MongoParams.Collection)
	res, err := coll.InsertMany(context.TODO(), recs)
	if err != nil {
		return errors.Wrapf(err, "publish %d records", len(records))
	}

	if len(res.InsertedIDs) > 0 {
		m.lastPublished.Lock()
		m.lastPublished.entry = records[len(records)-1]
		m.lastPublished.Unlock()
	}
	return nil
}

// LastPublished returns latest published entry
func (m *Mongo) LastPublished() (entry core.LogEntry, err error) {

	cachedLast := func() (entry core.LogEntry, ok bool) {
		m.lastPublished.Lock()
		entry = m.lastPublished.entry
		m.lastPublished.Unlock()
		return entry, entry.ID != ""
	}

	if lastEntry, ok := cachedLast(); ok {
		return lastEntry, nil
	}

	var mentry mongoLogEntry
	coll := m.Database(m.MongoParams.DBName).Collection(m.MongoParams.Collection)
	res := coll.FindOne(context.TODO(), bson.M{}, options.FindOne().SetSort(bson.D{{"_id", -1}}))
	if err := res.Decode(&mentry); err != nil {
		return core.LogEntry{}, nil
	}
	return m.makeLogEntry(mentry), nil
}

// Find records matching given request
func (m *Mongo) Find(req core.Request) ([]core.LogEntry, error) {

	if req.Limit == 0 {
		req.Limit = defaultLimit
	}
	req = m.sanitizeReq(req)
	// eliminate mongo find if lastPublished ID < req.LastID
	m.lastPublished.Lock()
	lastPublishedCached := m.lastPublished.entry
	m.lastPublished.Unlock()
	if req.LastID != "" && lastPublishedCached.ID != "" && req.LastID >= lastPublishedCached.ID {
		return []core.LogEntry{}, nil
	}

	query := m.makeQuery(req)

	var mresult []mongoLogEntry
	coll := m.Database(m.MongoParams.DBName).Collection(m.MongoParams.Collection)

	sortOpt := bson.D{{"_id", 1}}
	if req.LastID == "" || req.LastID == "0" {
		sortOpt = bson.D{{"_id", -1}}
	}
	cursor, e := coll.Find(context.TODO(), query, options.Find().SetLimit(int64(req.Limit)).SetSort(sortOpt))
	if e != nil {
		return nil, errors.Wrapf(e, "can't get records for %+v", req)
	}
	if e = cursor.All(context.TODO(), &mresult); e != nil {
		return nil, errors.Wrapf(e, "can't decode records for %+v", req)
	}

	if req.LastID == "" || req.LastID == "0" {
		sort.Slice(mresult, func(i, j int) bool { return mresult[i].ID.String() < mresult[j].ID.String() })
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
			result = append(result, primitive.Regex{Pattern: elem[1 : len(elem)-1], Options: ""})
		} else {
			result = append(result, elem)
		}
	}
	return result
}

func (m *Mongo) getBid(id string) primitive.ObjectID {

	if id == "0" || id == "" {
		res, _ := primitive.ObjectIDFromHex("000000000000")
		return res
	}

	bid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		res, _ := primitive.ObjectIDFromHex("000000000000")
		return res
	}
	return bid
}

// init Collection, make/ensure indexes
func (m *Mongo) init(collection string) error {
	log.Printf("[INFO] create Collection %s", collection)

	indexes := []mdrv.IndexModel{
		{Keys: bson.D{{"host", 1}, {"container", 1}, {"ts", 1}}},
		{Keys: bson.D{{"ts", 1}, {"host", 1}, {"container", 1}}},
		{Keys: bson.D{{"container", 1}, {"ts", 1}}},
	}

	err := m.Client.Database(m.MongoParams.DBName).CreateCollection(context.Background(), m.MongoParams.Collection,
		options.CreateCollection().SetCapped(true).SetSizeInBytes(int64(m.MaxCollectionSize)).
			SetMaxDocuments(int64(m.MaxDocs)))

	if err != nil {
		return errors.Wrapf(err, "initilize collection %s with %+v", collection, m.MongoParams)
	}

	coll := m.Database(m.MongoParams.DBName).Collection(m.MongoParams.Collection)
	if _, err := coll.Indexes().CreateMany(context.TODO(), indexes); err != nil {
		return errors.Wrap(err, "create indexes")
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
		Pid:       entry.Pid,
	}
	if entry.ID == "" {
		res.ID = primitive.NewObjectID()
	}
	return res
}

func (m *Mongo) makeLogEntry(entry mongoLogEntry) core.LogEntry {
	r := core.LogEntry{
		ID:        entry.ID.Hex(),
		Host:      entry.Host,
		Container: entry.Container,
		Msg:       entry.Msg,
		Ts:        entry.Ts,
		Pid:       entry.Pid,
	}
	r.CreatedTs = entry.ID.Timestamp()
	return r
}

func (m *Mongo) sanitizeReq(request core.Request) core.Request {
	if request.Limit > 1000 {
		request.Limit = 1000
	}
	return request
}
