package main

import (
	"io"
	"os"
	"testing"

	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/mongo/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupLog_MasksMongoPassword(t *testing.T) {
	mongoURL := "mongodb://admin:s3cretpw@mongo:27017/?db=dkll&collection=logs&authSource=admin" //nolint:gosec // fake credentials
	for _, dbg := range []bool{true, false} {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		stdout := os.Stdout
		os.Stdout = w
		setupLog(dbg, mongo.SecretsMongoUrls(mongoURL)...)
		log.Printf("[INFO] make mongo client for %q", mongoURL)
		os.Stdout = stdout
		require.NoError(t, w.Close())
		out, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Contains(t, string(out), "admin:******@mongo:27017")
		assert.NotContains(t, string(out), "s3cretpw")
	}
	setupLog(false)
}
