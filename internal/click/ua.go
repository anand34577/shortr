// Package click implements the async click-event pipeline: enrichment
// (user-agent parsing, GeoIP, bot detection), batched writes, and a
// disk spool so clicks survive a database outage.
package click

import (
	"strings"

	"github.com/mileusna/useragent"
)

// extraBotSubstrings catches common non-browser / preview-fetcher clients
// that the useragent library's IsBot() doesn't always flag, notably chat-app
// link unfurlers.
var extraBotSubstrings = []string{
	"whatsapp", "telegrambot", "discordbot", "slackbot", "twitterbot",
	"facebookexternalhit", "linkedinbot", "skypeuripreview", "embedly",
	"quora link preview", "outbrain", "pinterest", "vkshare", "w3c_validator",
	"redditbot", "applebot", "bingpreview", "yandex", "curl/", "wget/",
	"python-requests", "go-http-client", "headlesschrome", "bot", "spider", "crawl",
}

type Parsed struct {
	Device         string // desktop | mobile | tablet | bot | other
	OS             string
	OSVersion      string
	Browser        string
	BrowserVersion string
	IsBot          bool
}

func ParseUA(raw string) Parsed {
	if strings.TrimSpace(raw) == "" {
		return Parsed{Device: "other", IsBot: true}
	}
	ua := useragent.Parse(raw)
	p := Parsed{
		OS:             ua.OS,
		OSVersion:      ua.OSVersion,
		Browser:        ua.Name,
		BrowserVersion: ua.Version,
	}
	lower := strings.ToLower(raw)
	isBot := ua.Bot
	if !isBot {
		for _, s := range extraBotSubstrings {
			if strings.Contains(lower, s) {
				isBot = true
				break
			}
		}
	}
	p.IsBot = isBot

	switch {
	case isBot:
		p.Device = "bot"
	case ua.Mobile:
		p.Device = "mobile"
	case ua.Tablet:
		p.Device = "tablet"
	case ua.Desktop:
		p.Device = "desktop"
	default:
		p.Device = "other"
	}
	return p
}

// FirstLangTag extracts the primary language tag from an Accept-Language
// header, e.g. "en-US,en;q=0.9" -> "en-US".
func FirstLangTag(acceptLanguage string) string {
	s := strings.TrimSpace(acceptLanguage)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, ','); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) > 35 {
		s = s[:35]
	}
	return s
}
