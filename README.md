# DKLL [![build](https://github.com/umputun/dkll/workflows/build/badge.svg)](https://github.com/umputun/dkll/actions?query=workflow%3Abuild) [![Go Report Card](https://goreportcard.com/badge/github.com/umputun/dkll?12345)](https://goreportcard.com/report/github.com/umputun/dkll)  [![Coverage Status](https://coveralls.io/repos/github/umputun/dkll/badge.svg?branch=master)](https://coveralls.io/github/umputun/dkll?branch=master) [![Docker Automated build](https://img.shields.io/docker/automated/jrottenberg/ffmpeg.svg)](https://hub.docker.com/r/umputun/dkll/)


Logging server, agent and CLI client for dockerized infrastructure. 

- each host runs `dkll agent` container collecting logs from all docker containers on the host.
- the agent can store logs locally, or/and forward them to remote syslog server.
- server (`dkll server`) container installed on another host, acts as syslog server and stores logs.
- server also provides http api to access logs.
- client (`dkll client`) is a binary command-line utility to read/filter/search and follow logs.

_note: `dkll agent` can be used as a standalone container without server and client. In this case it will be a local collector of 
all logs and/or forwarder to any (external or internal) syslog server._
 

## Build from the source

1. clone this repo - `git clone https://github.com/umputun/dkll.git`
2. build the logger - `cd dkll && docker build -t umputun/dkll .`

_alternatively use provided `Makefile`, i.e. `make test lint docker`_


## Server

Server mode runs syslog server collecting records sent by dkll agent (see below). All records parsed, analyzed and stored
in mongodb (capped collection). Optionally, records can be sent to `<host>/<container>.log` files as well as to merged `dkll.log`
 file. All files rotated and compressed automatically.

### Usage

1. copy provided `compose-server.yml` to `docker-compose.yml` 
2. adjust `docker-compose.yml` if needed. 
3. pull containers - `docker-compose pull`
4. start server - `docker-compose up -d`


command line options and env params:

```
dkll [OPTIONS] server [server-OPTIONS]

Application Options:
      --dbg                              show debug info [$DEBUG]

Help Options:
  -h, --help                             Show this help message

[server command options]
      --api-port=                      rest server port (default: 8080) [$API_PORT]
      --syslog-port=                   syslog server port (default: 5514) [$SYSLOG_PORT]
      --mongo=                         mongo URL [$MONGO]
      --mongo-timeout=                 mongo timeout (default: 5s) [$MONGO_TIMEOUT]
      --mongo-size=                    max collection size (default: 10000000000) [$MONGO_SIZE]
      --mongo-docs=                    max docs in collection (default: 50000000) [$MONGO_DOCS]
      --backup=                        backup log files location [$BACK_LOG]
      --merged                         enable merged log file [$BACK_MRG]

    container:
      --limit.container.max-size=      max log size, in megabytes (default: 100) [$MAX_SIZE]
      --limit.container.max-backups=   max number of rotated files (default: 10) [$MAX_BACKUPS]
      --limit.container.max-age=       max age of rotated files, days (default: 30) [$MAX_AGE]

    merged:
      --limit.merged.max-size=         max log size, in megabytes (default: 100) [$MAX_SIZE]
      --limit.merged.max-backups=      max number of rotated files (default: 10) [$MAX_BACKUPS]
      --limit.merged.max-age=          max age of rotated files, days (default: 30) [$MAX_AGE]
```

- mongo URL specify the standard [mongodb connection string](https://docs.mongodb.com/manual/reference/connection-string/) with `db` and `collection` extra parameters, e.g. `mongodb://localhost:27017/admin?db=dkll&collection=logs` 
- if `backup` defined dkll server will make `host/container.log` files in `backup` directory
- `merged` parameter produces a single `dkll.log` file with all received records.

Parameters can be set in `command` directive (see docker-compose.yml) or as environment vars. 

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

- `POST /v1/stream?timeout=10s` - find records for given `Request` and stream it. Terminate stream on `timeout` inactivity.

### Storage

DKLL server uses mongo db to save and access records. It is possible and almost trivial to replace mongo with different 
backend by implementing 2 interfaces ([Publisher](https://github.com/umputun/dkll/blob/master/app/server/forwarder.go#L22) and 
[DataService](https://github.com/umputun/dkll/blob/master/app/server/rest_server.go#L28)) with just 3 functions:

- `Publish(records []core.LogEntry) (err error)`
- `LastPublished() (entry core.LogEntry, err error)`
- `Find(req core.Request) ([]core.LogEntry, error)`

### Security and auth

Both syslog and http don't restrict access. To allow some basic auth the simplest way is to run dkll server
behind [nginx-le](https://github.com/umputun/nginx-le) proxy with basic auth
[configured on nginx level](https://docs.nginx.com/nginx/admin-guide/security-controls/configuring-http-basic-authentication/). 
To limit access to syslog port (514) firewall (internal or external) cab be used.
 
TODO: example
 

## Agent

Agent container runs on each host and collects logs from all containers on the host. The logs sent to remote dkll server and/or 
stored and rotated locally. The agent can intercept logs from containers configured with a logging driver that works with docker
 logs (journald and json-file).  

To deploy agent use provided `compose-agent.yml` 

```
dkll [OPTIONS] agent [agent-OPTIONS]

Application Options:
      --dbg                show debug info [$DEBUG]

Help Options:
  -h, --help               Show this help message

[agent command options]
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
          --demo           demo mode, generates simulated log entries [$DEMO]
          --demo-every=    demo interval (default: 3s) [$DEMO_EVERY]
      
```

- at least one of destinations (`files` or `syslog`) should be allowed
- location of log files can be mapped to host via `volume`, ex: `- ./logs:/srv/logs` (see `compose-agent.yml`)
- both `--exclude` and `--include` flags are optional and mutually exclusive, i.e. if `--exclude` defined `--include` not allowed, and vise versa.

If you use the provided docker image, by default docker agent will run with `UID=1001`. Make sure that the access for docker socket granted for that user. Another way is specifing `APP_UID` environment variable for the agent container with either UID with docker privileges or `0` for running with root privileges.

#### Demo mode

Defining `--demo` or `$DEMO` switches agent to demo mode emitting fake log messages from fake containers. Everything preconfigured in `compose-demo.yml` and can be activated with `MONGO_PASSWD=<any random password> docker-compose -f compose-demo.yml up`.  

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

## Development 

- go v1.11 and above required
- repository uses modules and should be cloned outside of GOPATH
- `dkll server` tests need mongo up and running, i.e. `docker run -d --name=mongo -p 27017:27017 mongo`
 
