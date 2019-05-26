# DKLL [![Build Status](https://travis-ci.org/umputun/dkll.svg?branch=master)](https://travis-ci.org/umputun/dkll) [![Go Report Card](https://goreportcard.com/badge/github.com/umputun/dkll)](https://goreportcard.com/report/github.com/umputun/dkll) [![Coverage Status](https://coveralls.io/repos/github/umputun/dkll/badge.svg?branch=master)](https://coveralls.io/github/umputun/dkll?branch=master)


Logging server and CLI client for dockerized infrastructure. 

## Server

Server mode runs syslog server collecting records sent by [docker-logger collector](https://github.com/umputun/docker-logger). 
All records parsed, analyzed and stored in mongodb (capped collection). Optionally, records can be sent to `<host>/<container>
.log` files as well to merged `dkll.log` file.

### Usage

**dkll [OPTIONS] server [server-OPTIONS]**

```
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

TODO

### API

TODO

## Client

Command line client accessing dkll server and printing the content.

### Usage

**dkll [OPTIONS] client [client-OPTIONS]**

```
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
TODO
