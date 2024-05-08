FROM umputun/baseimage:buildgo-latest as build

ADD . /build/dkll
WORKDIR /build/dkll

RUN \
    revision=$(/script/git-rev.sh) && \
    echo "revision=${revision}" && \
    go build -o dkll -ldflags "-X main.revision=$revision -s -w" ./app


FROM umputun/baseimage:app-latest

# enables automatic changelog generation by tools like Dependabot
LABEL org.opencontainers.image.source="https://github.com/umputun/dkill"

COPY --from=build /build/dkll/dkll /srv/dkll

RUN chown -R app:app /srv
RUN chmod +x /srv/dkll

EXPOSE 8080 5514
WORKDIR /srv

CMD ["/srv/dkll", "server"]
