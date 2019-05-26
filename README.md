# DKLL [![Build Status](https://travis-ci.org/umputun/dkll.svg?branch=master)](https://travis-ci.org/umputun/dkll) [![Go Report 
Card](https://goreportcard.com/badge/github.com/umputun/dkll)](https://goreportcard.com/report/github.com/umputun/dkll)  
[![Coverage Status](https://coveralls.io/repos/github/umputun/dkll/badge.svg?branch=master)](https://coveralls
.io/github/umputun/dkll?branch=master)

Logging server and CLI client for dockerized infrastructure. 

## Server

Server mode runs syslog server collecting records sent by [docker-logger collector](https://github.com/umputun/docker-logger). 
All records parsed, analyzed and stored in mongodb (capped collection). Optionally, records can be sent to `<host>/<container>
.log` files as well to merged `dkll.log` file.

### Usage

TODO

### API

TODO

## Client

Command line client accessing dkll server and printing the content.

### Usage

TODO
