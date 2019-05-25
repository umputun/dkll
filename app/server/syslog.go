package server

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/go-pkgz/lgr"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

// Syslog server on TCP & UDP 5514. Should be mapped to 514 in compose
type Syslog struct {
	server *syslog.Server
}

// Go starts syslog server and returns channel of received lines
func (s *Syslog) Go(ctx context.Context) <-chan string {
	log.Print("[INFO] activate syslog server")
	outCh := make(chan string, 10000) // messages chanel
	inCh := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(inCh)
	s.server = syslog.NewServer()
	s.server.SetFormat(&origFormatter{})
	s.server.SetHandler(handler)
	if err := s.server.ListenUDP("0.0.0.0:5514"); err != nil { // we run under regular user, can't access 514
		log.Fatalf("[ERROR] can't listen to udp, %v", err)
	}
	if err := s.server.ListenTCP("0.0.0.0:5514"); err != nil {
		log.Fatalf("[ERROR] can't listen to tcp, %v", err)
	}
	if err := s.server.Boot(); err != nil {
		log.Fatalf("[ERROR] failed to activate syslog, %v", err)
	}

	go func(inCh syslog.LogPartsChannel) {
		for {
			select {
			case <-ctx.Done():
				return
			case parts := <-inCh:
				outCh <- fmt.Sprintf("%s", parts["msg"])
			}
		}
	}(inCh)

	go func() {
		<-ctx.Done()
		if err := s.server.Kill(); err != nil {
			log.Printf("[ERROR] failed to kill syslog server, %v", err)
		}
		s.server.Wait()
		close(inCh)
		close(outCh)
		log.Print("[WARN] syslog server terminated")
	}()

	return outCh
}

type origFormatter struct{}

// GetParser returns parse-nothing
func (f *origFormatter) GetParser(line []byte) format.LogParser {
	return &origParser{line: line}
}

// GetSplitFunc no split at all
func (f *origFormatter) GetSplitFunc() bufio.SplitFunc { return nil }

type origParser struct {
	line []byte
}

func (p *origParser) Parse() error {
	return nil
}

func (p *origParser) Dump() format.LogParts {
	s := string(p.line)
	// line may start with <id> we don't need it
	if strings.HasPrefix(s, "<") && strings.Contains(s, ">") {
		if pos := strings.Index(s, ">"); pos >= 0 {
			s = s[pos+1:]
		}
	}
	return format.LogParts{"msg": s}
}

func (p *origParser) Location(*time.Location) {

}
