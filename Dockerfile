FROM umputun/baseimage:buildgo-latest as build

ADD . /build/dkll
WORKDIR /build/dkll

RUN \
    revison=$(/script/git-rev.sh) && \
    echo "revision=${revison}" && \
    go build -mod=vendor -o dkll -ldflags "-X main.revision=$revison -s -w" ./app


FROM umputun/baseimage:app-latest

COPY --from=build /build/dkll/dkll /srv/dkll
RUN chown -R app:app /srv
RUN chmod +x /srv/dkll

EXPOSE 8080 5514
WORKDIR /srv

CMD ["/srv/dkll", "server"]
ENTRYPOINT ["/init.sh"]
