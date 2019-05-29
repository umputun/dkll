package logger

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

// DemoEmitter is a emitter for fake logs, no docker involved
type DemoEmitter struct {
	Duration time.Duration
}

// Logs generates random log messages
func (d *DemoEmitter) Logs(o docker.LogsOptions) error {
	var n int64
	for {
		select {
		case <-o.Context.Done():
			return o.Context.Err()
		case <-time.After(d.Duration):
			var line string
			switch o.Container {
			case "nginx":
				line = nginxDemo[rand.Intn(len(nginxDemo)-1)]
			case "rest":
				line = restDemo[rand.Intn(len(restDemo)-1)]
			case "mongo":
				line = mongoDemo[rand.Intn(len(mongoDemo)-1)]

			}

			if _, err := o.OutputStream.Write([]byte(fmt.Sprintf("%s\n", line))); err != nil {
				log.Printf("[WARN] demo log failed, %v", err)
			}
			n++
		}
	}
}

var nginxDemo = []string{
	`192.168.1.123 - - [29/May/2019:06:51:42 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16E227 (iPhone; U; CPU OS 12_2 like Mac OS X; en_us)`,
	`192.168.1.123 - - [29/May/2019:06:51:43 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16E227 (iPhone; U; CPU OS 12_2 like Mac OS X; en_us)`,
	`192.168.1.123 - - [29/May/2019:06:51:50 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "CastBox/7.67.2-190518054 (Linux;Android 8.0.0) ExoPlayerLib/2.9.1/7.67.2-190518054 (Linux;Android 8.0.0) xoPlayerLib/2.9.1`,
	`192.168.1.123 - - [29/May/2019:06:51:54 +0000] "HEAD / HTTP/1.1" 301 0 "-" "updown.io daemon 2.2`,
	`192.168.1.123 - - [29/May/2019:06:51:54 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "PodcastAddict/v2 - Dalvik/2.1.0 (Linux; U; Android 9; ONEPLUS A5000 Build/PKQ1.180716.001)`,
	`192.168.1.123 - - [29/May/2019:06:52:05 +0000] "GET /rt_podcast646.mp3 HTTP/1.1" 302 70 "-" "Mozilla/5.0 (
Linux; Android 7.0; Redmi Note 4 Build/NRD90M; wv) AppleWebKit/537.36 (KHTML, like Gecko) version/4.0 "Chrome/73.0.3683.90 Mobile Safari/537.36 GSA/9.88.7.21.arm64`,
	`192.168.1.123 - - [29/May/2019:06:52:05 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16E227 (iPhone; U; CPU OS 12_2 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:05 +0000] "GET /rt_podcast640.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16E227 (iPhone; U; CPU OS 12_2 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:05 +0000] "GET /rt_podcast639.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16E227 (iPhone; U; CPU OS 12_2 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:08 +0000] "GET /podcast-archives.rss HTTP/1.1" 301 178 "-" "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)`,
	`192.168.1.123 - - [29/May/2019:06:52:09 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "atc/1.0 watchOS/5.2.1 model/Watch4,2 hwp/t8006 build/16U113 (6; dt:191)`,
	`192.168.1.123 - - [29/May/2019:06:52:10 +0000] "GET /rt_podcast650.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16G5027i (iPhone; U; CPU OS 12_4 like Mac OS X; en_gb)`,
	`192.168.1.123 - - [29/May/2019:06:52:22 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16E227 (iPhone; U; CPU OS 12_2 like Mac OS X; en_us)`,
	`192.168.1.123 - - [29/May/2019:06:52:22 +0000] "GET /rt644post.mp3 HTTP/1.1" 302 66 "-" "AppleCoreMedia/1.0.0.16F203 (iPhone; U; CPU OS 12_3_1 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:22 +0000] "GET /rt644post.mp3 HTTP/1.1" 302 66 "-" "AppleCoreMedia/1.0.0.16F203 (iPhone; U; CPU OS 12_3_1 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:24 +0000] "HEAD / HTTP/1.1" 301 0 "-" "updown.io daemon 2.2`,
	`192.168.1.123 - - [29/May/2019:06:52:24 +0000] "GET /rt644post.mp3 HTTP/1.1" 302 66 "-" "AppleCoreMedia/1.0.0.16F203 (iPhone; U; CPU OS 12_3_1 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:24 +0000] "GET /rt644post.mp3 HTTP/1.1" 302 66 "-" "AppleCoreMedia/1.0.0.16F203 (iPhone; U; CPU OS 12_3_1 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:25 +0000] "GET /rt644post.mp3 HTTP/1.1" 302 66 "-" "AppleCoreMedia/1.0.0.16F203 (iPhone; U; CPU OS 12_3_1 like Mac OS X; ru_ru)`,
	`192.168.1.123 - - [29/May/2019:06:52:27 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "PodcastAddict/v2 - Dalvik/2.1.0 (Linux; U; Android 9; CLT-L09 Build/HUAWEICLT-L09)`,
	`192.168.1.123 - - [29/May/2019:06:52:27 +0000] "HEAD / HTTP/1.1" 301 0 "-" "Monit/5.24.0`,
	`192.168.1.123 - - [29/May/2019:06:52:28 +0000] "GET /podcast-archives.rss HTTP/1.1" 301 178 "-" "-`,
	`192.168.1.123 - - [29/May/2019:06:52:32 +0000] "HEAD /rt_podcast500.mp3 HTTP/1.1" 302 0 "-" "Monit/5.24.0`,
	`192.168.1.123 - - [29/May/2019:06:52:53 +0000] "HEAD / HTTP/1.1" 301 0 "-" "updown.io daemon 2.2`,
	`192.168.1.123 - - [29/May/2019:06:52:59 +0000] "GET /rt_podcast651.mp3 HTTP/1.1" 302 70 "-" "AppleCoreMedia/1.0.0.16E227 (iPhone; U; CPU OS 12_2 like Mac OS 192.168.1.123`,
}

var mongoDemo = []string{
	`2019-05-11T02:04:03.245+0000 I ACCESS   [conn44204] Successfully authenticated as principal root on admin`,
	`2019-05-11T02:04:44.584+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:54266 #45239 (6 connections now open)`,
	`2019-05-11T02:04:44.585+0000 I NETWORK  [conn45239] end connection 172.18.0.9:54266 (5 connections now open)`,
	`2019-05-11T02:05:49.278+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:54442 #45240 (6 connections now open)`,
	`2019-05-11T02:05:49.279+0000 I NETWORK  [conn45240] end connection 172.18.0.9:54442 (5 connections now open)`,
	`2019-05-11T02:06:53.979+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:54688 #45241 (6 connections now open)`,
	`2019-05-11T02:06:53.979+0000 I NETWORK  [conn45241] end connection 172.18.0.9:54688 (5 connections now open)`,
	`2019-05-11T02:07:57.165+0000 I ACCESS   [conn44204] Successfully authenticated as principal root on admin`,
	`2019-05-11T02:07:58.874+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:54874 #45242 (6 connections now open)`,
	`2019-05-11T02:07:58.875+0000 I NETWORK  [conn45242] end connection 172.18.0.9:54874 (5 connections now open)`,
	`2019-05-11T02:09:03.369+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:55068 #45243 (6 connections now open)`,
	`2019-05-11T02:09:03.370+0000 I NETWORK  [conn45243] end connection 172.18.0.9:55068 (5 connections now open)`,
	`2019-05-11T02:09:43.542+0000 I ACCESS   [conn44204] Successfully authenticated as principal root on admin`,
	`2019-05-11T02:10:07.873+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:55256 #45244 (6 connections now open)`,
	`2019-05-11T02:10:07.874+0000 I NETWORK  [conn45244] end connection 172.18.0.9:55256 (5 connections now open)`,
	`2019-05-11T02:11:12.367+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:55462 #45245 (6 connections now open)`,
	`2019-05-11T02:11:12.368+0000 I NETWORK  [conn45245] end connection 172.18.0.9:55462 (5 connections now open)`,
	`2019-05-11T02:12:17.062+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:55952 #45246 (6 connections now open)`,
	`2019-05-11T02:12:17.063+0000 I NETWORK  [conn45246] end connection 172.18.0.9:55952 (5 connections now open)`,
	`2019-05-11T02:13:21.755+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:56146 #45247 (6 connections now open)`,
	`2019-05-11T02:13:21.756+0000 I NETWORK  [conn45247] end connection 172.18.0.9:56146 (5 connections now open)`,
	`2019-05-11T02:14:26.247+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:56420 #45248 (6 connections now open)`,
	`2019-05-11T02:14:26.248+0000 I NETWORK  [conn45248] end connection 172.18.0.9:56420 (5 connections now open)`,
	`2019-05-11T02:15:30.742+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:56790 #45249 (6 connections now open)`,
	`2019-05-11T02:15:30.743+0000 I NETWORK  [conn45249] end connection 172.18.0.9:56790 (5 connections now open)`,
	`2019-05-11T02:16:35.436+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:57008 #45250 (6 connections now open)`,
	`2019-05-11T02:16:35.437+0000 I NETWORK  [conn45250] end connection 172.18.0.9:57008 (5 connections now open)`,
	`2019-05-11T02:17:39.928+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:57228 #45251 (6 connections now open)`,
	`2019-05-11T02:17:39.929+0000 I NETWORK  [conn45251] end connection 172.18.0.9:57228 (5 connections now open)`,
	`2019-05-11T02:17:57.162+0000 I ACCESS   [conn44204] Successfully authenticated as principal root on admin`,
	`2019-05-11T02:18:44.221+0000 I NETWORK  [listener] connection accepted from 172.18.0.9:57510 #45252 (6 connections now open)`,
}

var restDemo = []string{
	`2019/05/27 16:10:36.274 [INFO]  GET - /api/v1/rss/site?site=radiot - 721f34e65c51 - 200 (15872) - 207.497µs`,
	`2019/05/27 16:10:37.208 [INFO]  GET - /api/v1/rss/site?site=radiot - 2c65059c4c90 - 200 (15872) - 338.754µs`,
	`2019/05/27 16:10:53.983 [INFO]  POST - /api/v1/counts?site=rtnews - 6e8c499551fb - 200 (50763) - 1.
093672ms - ["https://news.radio-t.com/post/glavnyi-navyk-razrabotchika-kotoryi-sdelaet-vash-kod-luchshe","https://news.radio-t.com/post/github-sponsors-novyi-sposob-vnesti-svoi-vklad-v-open-source","https://news.radio-t.com/post/tekhnicheskii-dolg","https://news.radio-t.com/post/github-sponsors-now-lets-users-back-open-source-projects","https://news.radio-t.com/post/apple-introduces-first-8-core-macbook-pro-the-fastest-mac-notebook-ever","https://news.radio-t.com/post/bliki-technicaldebt","https://news.radio-t.com/post/chem-bystree-vy-zabudete-oop-tem-luchshe-dlia-vas-i-vashikh-programm","https://news.radio-t.com/post/opennews-vzlom-diskussionnoi-ploshchadki-stack-overflow","https://news.radio-t.com/post/spotify-s-first-hardware-is-a-voice-controlled-device-for-your-car","https://news.radio-t.com/post/the-new-motorola-razr-looks-amazing-in-this-render","https://news.radio-t.com/post/why-whatsapp-will-never-be-secure,"https://news.radio-t.com/post/falsehoods-programmers-believe-about-unix-time","https://news.radio-t.com...`,
	`2019/05/27 16:10:55.045 [INFO]  POST - /api/v1/counts?site=radiot - d40c6ca5b373 - 200 (3) - 75.952µs - []`,
	`2019/05/27 16:10:55.055 [INFO]  GET - /api/v1/config?site=radiot - d40c6ca5b373 - 200 (498) - 69.365µs`,
	`2019/05/27 16:10:55.241 [INFO]  GET - /api/v1/find?site=radiot&url=https://radio-t.com/p/2008/10/26/podcast-109/&sort=-active&format=tree - d40c6ca5b373 - 200 (23087) - 188.182µs`,
	`2019/05/27 16:11:06.284 [INFO]  GET - /api/v1/rss/site?site=radiot - 721f34e65c51 - 200 (15872) - 862.73µs`,
	`2019/05/27 16:11:36.293 [INFO]  GET - /api/v1/rss/site?site=radiot - 721f34e65c51 - 200 (15872) - 213.72µs`,
	`2019/05/27 16:11:54.201 [INFO]  POST - /api/v1/counts?site=rtnews - 6e8c499551fb - 200 (50763) - 1.197306ms - ["https://news.radio-t.com/post/glavnyi-navyk-razrabotchika-kotoryi-sdelaet-vash-kod-luchshe","https://news.radio-t.com/post/github-sponsors-novyi-sposob-vnesti-svoi-vklad-v-open-source","https://news.radio-t.com/post/tekhnicheskii-dolg","https://news.radio-t.com/post/github-sponsors-now-lets-users-back-open-source-projects","https://news.radio-t.com/post/apple-introduces-first-8-core-macbook-pro-the-fastest-mac-notebook-ever","https://news.radio-t.com/post/bliki-technicaldebt","https://news.radio-t.com/post/chem-bystree-vy-zabudete-oop-tem-luchshe-dlia-vas-i-vashikh-programm","https://news.radio-t.com/post/opennews-vzlom-diskussionnoi-ploshchadki-stack-overflow","https://news.radio-t.com/post/spotify-s-first-hardware-is-a-voice-controlled-device-for-your-car","https://news.radio-t.com/post/the-new-motorola-razr-looks-amazing-in-this-render","https://news.radio-t.com/post/why-whatsapp-will-never-be-secure","https://news.radio-t.com/post/falsehoods-programmers-believe-about-unix-time","https://news.radio-t.com...`,
	`2019/05/27 16:12:06.299 [INFO]  GET - /api/v1/rss/site?site=radiot - 721f34e65c51 - 200 (15872) - 844.052µs`,
	`2019/05/27 16:12:36.307 [INFO]  GET - /api/v1/rss/site?site=radiot - 721f34e65c51 - 200 (15872) - 227.176µs`,
	`2019/05/27 16:12:54.424 [INFO]  POST - /api/v1/counts?site=rtnews - 6e8c499551fb - 200 (50763) - 977.
091µs - ["https://news.radio-t.com/post/glavnyi-navyk-razrabotchika-kotoryi-sdelaet-vash-kod-luchshe",
"https://news.radio-t.com/post/github-sponsors-novyi-sposob-vnesti-svoi-vklad-v-open-source","https://news.radio-t.com/post/tekhnicheskii-dolg","https://news.radio-t.com/post/github-sponsors-now-lets-users-back-open-source-projects","https://news.radio-t.com/post/apple-introduces-first-8-core-macbook-pro-the-fastest-mac-notebook-ever","https://news.radio-t.com/post/bliki-technicaldebt","https://news.radio-t.com/post/chem-bystree-vy-zabudete-oop-tem-luchshe-dlia-vas-i-vashikh-programm","https://news.radio-t.com/post/opennews-vzlom-diskussionnoi-ploshchadki-stack-overflow","https://news.radio-t.com/post/spotify-s-first-hardware-is-a-voice-controlled-device-for-your-car","https://news.radio-t.com/post/the-new-motorola-razr-looks-amazing-in-this-render","https://news.radio-t.com/post/why-whatsapp-will-never-be-secure","https://news.radio-t.com/post/falsehoods-programmers-believe-about-unix-time","https://news.radio-t.com...`,
	`2019/05/27 16:13:06.314 [INFO]  GET - /api/v1/rss/site?site=radiot - 721f34e65c51 - 200 (15872) - 812.592µs`,
	`2019/05/27 16:13:32.620 [INFO]  POST - /api/v1/counts?site=radiot - a7d40bba62fb - 200 (2051) - 117.91µs - ["https://radio-t.com/p/2019/05/25/podcast-651/","https://radio-t.com/p/2019/05/21/prep-651/","https://radio-t.com/p/2019/05/18/podcast-650/","https://radio-t.com/p/2019/05/14/prep-650/","https://radio-t.com/p/2019/05/11/podcast-649/","https://radio-t.com/p/2019/05/07/prep-649/","https://radio-t.com/p/2019/05/04/podcast-648/","https://radio-t.com/p/2019/04/30/prep-648/","https://radio-t.com/p/2019/04/27/podcast-647/","https://radio-t.com/p/2019/04/23/prep-647/","https://radio-t.com/p/2019/04/20/podcast-646/","https://radio-t.com/p/2019/04/16/prep-646/","https://radio-t.com/p/2019/04/13/podcast-645/","https://radio-t.com/p/2019/04/09/prep-645/","https://radio-t.com/p/2019/04/06/podcast-644/"]`,
	`2019/05/27 16:13:36.322 [INFO]  GET - /api/v1/rss/site?site=radiot - 721f34e65c51 - 200 (15872) - 251.704µs`,
}
