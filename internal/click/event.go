package click

import (
	"net"
	"time"
)

// Event is what the redirect handler builds — raw, unparsed data only, so
// the hot path does zero UA/Geo work.
type Event struct {
	LinkID         string
	TS             time.Time
	RawIP          net.IP
	UserAgent      string
	Referrer       string
	AcceptLanguage string
	QueryString    string
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	UTMTerm        string
	UTMContent     string
}

// Metrics is the subset of internal/metrics the writer reports to; defined
// here (not imported) to avoid a dependency cycle.
type Metrics interface {
	IncClicksWritten(n int64)
	IncClicksDropped(n int64)
	IncClicksSpooled(n int64)
	SetQueueDepth(n int)
	IncDBErrors(n int64)
}

type noopMetrics struct{}

func (noopMetrics) IncClicksWritten(int64) {}
func (noopMetrics) IncClicksDropped(int64) {}
func (noopMetrics) IncClicksSpooled(int64) {}
func (noopMetrics) SetQueueDepth(int)      {}
func (noopMetrics) IncDBErrors(int64)      {}
