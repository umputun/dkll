# DKLL [![Build Status](https://travis-ci.org/umputun/dkll.svg?branch=master)](https://travis-ci.org/umputun/dkll) [![Go Report Card](https://goreportcard.com/badge/github.com/umputun/dkll)](https://goreportcard.com/report/github.com/umputun/dkll)  [![Coverage Status](https://coveralls.io/repos/github/umputun/dkll/badge.svg?branch=master)](https://coveralls.io/github/umputun/dkll?branch=master) [![Docker Automated build](https://img.shields.io/docker/automated/jrottenberg/ffmpeg.svg)](https://hub.docker.com/r/umputun/dkll/)


Logging server, agent and CLI client for dockerized infrastructure. 

## Server

Server mode runs syslog server collecting records sent by [docker-logger collector](https://github.com/umputun/docker-logger). 
All records parsed, analyzed and stored in mongodb (capped collection). Optionally, records can be sent to `<host>/<container>
.log` files as well to merged `dkll.log` file.

### Usage

1. Copy provided `compose-server.yml` to `docker-compose.yml` 
2. Adjust `docker-compose.yml` if needed. 
3. Pull containers - `docker-compose pull`
4. Start server - `docker-compose up -d`


command line options and env params:

```
dkll [OPTIONS] server [server-OPTIONS]

Application Options:
      --dbg                              show debug info [$DEBUG]

Help Options:
  -h, --help                             Show this help message

[server command options]
          --port=                        rest server port (default: 8080) [$PORT]
          --mongo=                       mongo host:port [$MONGO]
          --mongo-passwd=                mongo password [$MONGO_PASSWD]
          --mongo-delay=                 mongo initial delay (default: 0s) [$MONGO_DELAY]
          --mongo-timeout=               mongo timeout (default: 5s) [$MONGO_TIMEOUT]
          --mongo-db=                    mongo database name (default: dkll) [$MONGO_DB]
          --backup=                      backup log files location [$BACK_LOG]
          --merged                       enable merged log file [$BACK_MRG]

    container:
          --limit.container.max-size=    max log size, in megabytes (default: 100) [$MAX_SIZE]
          --limit.container.max-backups= max number of rotated files (default: 10) [$MAX_BACKUPS]
          --limit.container.max-age=     max age of rotated files (default: 30) [$MAX_AGE]

    merged:
          --limit.merged.max-size=       max log size, in megabytes (default: 100) [$MAX_SIZE]
          --limit.merged.max-backups=    max number of rotated files (default: 10) [$MAX_BACKUPS]
          --limit.merged.max-age=        max age of rotated files (default: 30) [$MAX_AGE]
```


### API

Records format (response):

```go
type LogEntry struct {
	ID        string    `json:"id"`         // record ID
	Host      string    `json:"host"`       // host name
	Container string    `json:"container"`  // container
	Pid       int       `json:"pid"`        // process id
	Msg       string    `json:"msg"`        // log message
	Ts        time.Time `json:"ts"`         // reported time 
	CreatedTs time.Time `json:"cts"`        // creation time
}
```

- `GET /v1/last` - get last records `LogEntry`
- `POST /v1/find` - find records for given `Request`

```go
type Request struct {
	LastID     string    `json:"id"`                   // get records after this id
	Limit      int       `json:"max"`                  // max size of response, i.e. number of messages one request can return
	Hosts      []string  `json:"hosts,omitempty"`      // list of hosts, can be exact match or regex in from of /regex/
	Containers []string  `json:"containers,omitempty"` // list of containers, can be regex as well
	Excludes   []string  `json:"excludes,omitempty"`   // list of excluded containers, can be regex
	FromTS     time.Time `json:"from_ts,omitempty"`    
	ToTS       time.Time `json:"to_ts,omitempty"`
}
```

### Storage

DKLL server uses mongo db to save and access records. It is possible and almost trivial to replace mongo with different 
backend by implementing 2 interfaces ([Publisher](https://github.com/umputun/dkll/blob/master/app/server/forwarder.go#L22) and 
[DataService](https://github.com/umputun/dkll/blob/master/app/server/rest_server.go#L28)) with just 3 functions:

- `Publish(records []core.LogEntry) (err error)`
- `LastPublished() (entry core.LogEntry, err error)`
- `Find(req core.Request) ([]core.LogEntry, error)`


## Agent

Agent container runs on each host and collects logs from all containers on the host. The logs sent to remote dkll server and 
stored locally (optional).

To deploy agent use provided `compose-agent.yml` 

```
      -d, --docker=        docker host (default: unix:///var/run/docker.sock) [$DOCKER_HOST]
          --syslog         enable logging to syslog [$LOG_SYSLOG]
          --syslog-host=   syslog host (default: 127.0.0.1:514) [$SYSLOG_HOST]
          --syslog-prefix= syslog prefix (default: docker/) [$SYSLOG_PREFIX]
          --files          enable logging to files [$LOG_FILES]
          --max-size=      size of log triggering rotation (MB) (default: 10) [$MAX_SIZE]
          --max-files=     number of rotated files to retain (default: 5) [$MAX_FILES]
          --max-age=       maximum number of days to retain (default: 30) [$MAX_AGE]
          --mix-err        send error to std output log file [$MIX_ERR]
          --loc=           log files locations (default: logs) [$LOG_FILES_LOC]
      -x, --exclude=       excluded container names [$EXCLUDE]
      -i, --include=       included container names [$INCLUDE]
      -j, --json           wrap message with JSON envelope [$JSON]
```

## Client

Command line client accessing dkll server and printing the content.

### Usage

DKLL client should be used directly as a compiled binary. You can get precompiled 
[release](https://github.com/umputun/dkll/releases) or build it from the source (`make deploy`). 

```
dkll [OPTIONS] client [client-OPTIONS]


Application Options:
      --dbg       show debug info [$DEBUG]

Help Options:
  -h, --help      Show this help message

[client command options]
      -a, --api=  API endpoint (client) [$DKLL_API]
      -c=         show container(s) only
      -h=         show host(s) only
      -x=         exclude container(s)
      -m          show syslog timestamp
      -p          show pid
      -s          show syslog messages
      -f          follow mode
      -t          tail mode
      -n=         show N records
      -g=         grep on entire record
      -G=         un-grep on entire record
          --tail= number of initial records (default: 10)
          --tz=   time zone (default: Local)
```

* containers (-c), hosts (-h) and exclusions (-x) can be repeated multiple times. 
* both containers and hosts support regex inside "/", i.e. `/^something/`

