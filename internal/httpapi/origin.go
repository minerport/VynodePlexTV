package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

// No forwarded headers are trusted. A proxy can use an explicit public origin.
func allowedOrigin(r *http.Request, configured string) bool {
	origin := r.Header.Get("Origin")
	if configured != "" && configured != "auto" {
		return origin == configured
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return u.Scheme == scheme && strings.EqualFold(u.Host, r.Host)
}
