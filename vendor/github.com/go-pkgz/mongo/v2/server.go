package mongo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	driver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Connect to mongo url and return client. Supports expanded url params to pass a set of custom values in the url
func Connect(ctx context.Context, opts *options.ClientOptions, u string, extras ...string) (*driver.Client, map[string]interface{}, error) {
	mongoURL, extMap, err := parseExtMongoURI(u, extras)
	if err != nil {
		return nil, nil, fmt.Errorf("can't parse mongo url: %w", err)
	}

	res, err := driver.Connect(ctx, opts.ApplyURI(mongoURL))
	if err != nil {
		return nil, nil, fmt.Errorf("can't connect to mongo: %w", err)
	}

	if err = res.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("can't ping mongo: %w", err)
	}

	return res, extMap, nil
}

// parseExtMongoURI extracts extra params from extras list and remove them from the url.
// Input example: mongodb://user:password@127.0.0.1:27017/test?ssl=true&ava_db=db1&ava_coll=coll1
func parseExtMongoURI(u string, extras []string) (host string, ex map[string]interface{}, err error) {
	if u == "" {
		return "", nil, errors.New("empty url")
	}
	if len(extras) == 0 {
		return u, nil, nil
	}
	exMap := map[string]interface{}{}

	uu, err := url.Parse(u)
	if err != nil {
		return "", nil, err
	}

	q := uu.Query()
	for _, ex := range extras {
		if val := uu.Query().Get(ex); val != "" {
			exMap[ex] = val
		}
		q.Del(ex)
	}
	uu.RawQuery = q.Encode()
	return uu.String(), exMap, nil
}

// PrepSort prepares sort params for mongo driver and returns bson.D
// Input string provided as [+|-]field1,[+|-]field2,[+|-]field3...
// + means ascending, - means descending. Lack of + or - in the beginning of the field name means ascending sort.
func PrepSort(sort ...string) bson.D {
	res := bson.D{}
	for _, s := range sort {
		if s == "" {
			continue
		}
		s = strings.TrimSpace(s)
		switch s[0] {
		case '-':
			res = append(res, bson.E{Key: s[1:], Value: -1})
		case '+':
			res = append(res, bson.E{Key: s[1:], Value: 1})
		default:
			res = append(res, bson.E{Key: s, Value: 1})
		}
	}
	return res
}

// PrepIndex prepares index params for mongo driver and returns IndexModel
func PrepIndex(keys ...string) driver.IndexModel {
	return driver.IndexModel{Keys: PrepSort(keys...)}
}

// Bind request json body from io.Reader to bson record
func Bind(r io.Reader, v interface{}) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return bson.UnmarshalExtJSON(body, false, v)
}

var reMongoURL = regexp.MustCompile(`mongodb(\+srv)?://([^:]+):([^@]+)@[^/]+/?.*`)

// SecretsMongoUrls retrieves passwords from mongo urls.
// this is needed to pass mongo password to the logging masking function
func SecretsMongoUrls(urls ...string) (res []string) {
	res = []string{}
	mongoPasswd := func(murl string) (string, bool) {
		if !reMongoURL.MatchString(murl) {
			return "", false
		}
		elems := reMongoURL.FindStringSubmatch(murl)
		if len(elems) < 4 {
			return "", false
		}
		return elems[3], true
	}

	for _, u := range urls {
		if secret, ok := mongoPasswd(u); ok {
			res = append(res, secret)
		}
	}
	return res
}
