package mongo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	driver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MakeTestConnection connects to MONGO_TEST url or "mongo" host (in no env) and returns new connection.
// collection name randomized on each call
func MakeTestConnection(t *testing.T) (mg *driver.Client, coll *driver.Collection, teardown func()) {
	collName := fmt.Sprintf("test_%d", time.Now().Nanosecond())
	return MakeTestConnectionWithColl(t, collName)
}

// MakeTestConnectionWithColl connects to MONGO_TEST url or "mongo" host (in no env) and returns new connection.
// collection name passed in as cname param
func MakeTestConnectionWithColl(t *testing.T, cname string) (mg *driver.Client, coll *driver.Collection, teardown func()) {
	mongoURL := getMongoURL(t)
	log.Print("[DEBUG] connect to mongo test instance")
	opts := options.ClientOptions{}
	opts.SetAppName("test")
	opts.SetConnectTimeout(time.Second)
	mg, err := driver.Connect(context.Background(), options.Client().ApplyURI(mongoURL))
	require.NoError(t, err, "failed to make mongo client")
	coll = mg.Database("test").Collection(cname)
	teardown = func() {
		require.NoError(t, coll.Drop(context.Background()))
		assert.NoError(t, mg.Disconnect(context.Background()))
	}

	_ = coll.Drop(context.Background())
	return mg, coll, teardown
}

func getMongoURL(t *testing.T) string {
	mongoURL := os.Getenv("MONGO_TEST")
	if mongoURL == "" {
		mongoURL = "mongodb://mongo:27017"
		t.Logf("no MONGO_TEST in env, defaulted to %s", mongoURL)
	}
	if mongoURL == "skip" {
		t.Skip("skip mongo test")
	}
	return mongoURL
}
