# Mongo  [![Build Status](https://github.com/go-pkgz/mongo/workflows/build/badge.svg)](https://github.com/go-pkgz/mongo/actions) [![Go Report Card](https://goreportcard.com/badge/github.com/go-pkgz/mongo)](https://goreportcard.com/report/github.com/go-pkgz/mongo) [![Coverage Status](https://coveralls.io/repos/github/go-pkgz/mongo/badge.svg?branch=master)](https://coveralls.io/github/go-pkgz/mongo?branch=master)

Provides helpers on top of [mongo-go-driver](https://github.com/mongodb/mongo-go-driver)

## Version 2, for the official mongo-go-driver 

_note: version 1 (mgo based) is deprecated_

- Install and update - `go get -u github.com/go-pkgz/mongo/v2`

- `Connect` - Connects with mongo url and return mongo client. Supports extra url params to pass a set of custom values in
 the url, for example `"mongodb://127.0.0.1:27017/test?debug=true&foo=bar`. `Connect` returns `mongo.Client` as well as the map
  with all extra key/values. After connect call it also tries to ping the mongo server.

```golang
    opts := options.Client().SetAppName("test-app")
    m, params err := Connect(ctx, opts, "mongodb://127.0.0.1:27017/test?debug=true&name=abcd", "debug", "name")
    if err != nil {
        panic("can't make mongo server")
    }
    log.Printf("%+v", params) // prints {"debug":true, "name":"abcd"} 
```  

- `BufferedWriter` implements buffered writer to mongo. Write method caching internally till it reached buffer size. Flush methods can be called manually at any time. 
  - `WithCollection` sets collection name to write to
  - `WithAutoFlush` sets auto flush duration
  
- `PrepSort` - prepares sort object `bson.D` from strings like `"a,-b"`
- `PrepIndex` - prepares index object `driver.IndexModel` from strings like `"a,-b"`

### Testing

- `mongo.MakeTestConnection` creates `mongo.Client` and `mongo.Collection` for url defined in env `MONGO_TEST`. If not defined`mongodb://mongo:27017` used. By default it will use random connection with prefix `test_` in `test` DB.
- `MakeTestConnectionWithColl` creates `mongo.Client` and `mongo.Collection` with a custom collection name.

Both methods return `mongo.Client` and `mongo.Collection` which can be used for testing. It also returns `teardown` func to clean mongo after the test. 
