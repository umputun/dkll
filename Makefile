OS=linux
ARCH=amd64

bin:
	docker build -f Dockerfile.artifacts -t dkll.bin .
	- @docker rm -f dkll.bin 2>/dev/null || exit 0
	docker run -d --name=dkll.bin dkll.bin
	docker cp dkll.bin:/artifacts/dkll.$(OS)-$(ARCH) dkll
	docker rm -f dkll.bin

docker:
	docker build -t umputun/dkll .

deploy:
	docker build -f Dockerfile.artifacts -t dkll.bin .
	- @docker rm -f dkll.bin 2>/dev/null || exit 0
	- @mkdir -p bin
	docker run -d --name=dkll.bin dkll.bin
	docker cp dkll.bin:/artifacts/dkll.linux-amd64.tar.gz bin/dkll.linux-amd64.tar.gz
	docker cp dkll.bin:/artifacts/dkll.linux-arm64.tar.gz bin/dkll.linux-arm64.tar.gz
	docker cp dkll.bin:/artifacts/dkll.darwin-amd64.tar.gz bin/dkll.darwin-amd64.tar.gz
	docker cp dkll.bin:/artifacts/dkll.windows-amd64.zip bin/dkll.windows-amd64.zip
	docker rm -f dkll.bin

.PHONY: bin