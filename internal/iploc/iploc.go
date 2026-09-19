// Package iploc looks up IP geolocation/ASN data from an optional,
// self-configured HTTP service (GET <baseURL>/<ip> -> JSON). It is entirely
// opt-in — with no base URL configured, Lookup is never called — and never
// points at any specific third party; the admin supplies their own service.
package iploc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Result mirrors the response shape of the configured lookup service.
type Result struct {
	IP              string  `json:"ip"`
	Country         string  `json:"country"`
	CountryCode     string  `json:"countryCode"`
	City            string  `json:"city"`
	Subdivision     string  `json:"subdivision"`
	SubdivisionCode string  `json:"subdivisionCode"`
	Postal          string  `json:"postal"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Timezone        string  `json:"timezone"`
	ASN             int     `json:"asn"`
	ASNOrganization string  `json:"asnOrganization"`
}

var client = &http.Client{Timeout: 5 * time.Second}

// Lookup calls GET <baseURL>/<ip> and decodes the JSON response. baseURL
// must be a non-empty http(s) URL; ip must be a valid IPv4/IPv6 address.
func Lookup(ctx context.Context, baseURL, ip string) (*Result, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("iploc: no base URL configured")
	}
	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("iploc: invalid IP address %q", ip)
	}
	url := strings.TrimRight(baseURL, "/") + "/" + ip
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("iploc: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("iploc: unexpected status %d", resp.StatusCode)
	}
	var out Result
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("iploc: decoding response: %w", err)
	}
	return &out, nil
}
