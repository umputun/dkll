# DKLL - Logging Server & Client

Logging server (with REST) and CLI client for dockerized infrastructure

## Run a server

It builds and deploy complete image and compose to run it anywhere.

## Run a client

```
Application Options:
      --server          runs syslog forwarder and rest, activates server (default: false)
      --mongo=          mongo host:port (server only) [$DKLL_MONGO]
      --mongo-password= mongo pssword (server only) [$MONGO_PASSWD]
      --mongo-delay=    mongo initial delay (server only) (default: 0) [$MONGO_DELAY]
      --mongo-db=       mongo database name (server only) (default: dklogger) [$MONGO_DB]
      --syslog=         syslog locaion (server only) (default: /var/log/messages)
  -a, --api=            API endpoint (client) [$DKLL_API]
  -l, --local=          local file (client) [$DKLL_LOCAL]
  -c=                   show container(s) only
  -h=                   show host(s) only
  -x=                   exclude container(s)
  -t                    show syslog timestamp (default: false)
  -p                    show pid (default: false)
  -f                    follow mode (default: false)
      --dbg             show debug info
      --help            show help (default: false)
```
      
