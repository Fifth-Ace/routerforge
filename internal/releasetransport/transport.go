// Package releasetransport defines the two approved RouterForge release origins.
package releasetransport

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const GitHubBase = "https://github.com/Fifth-Ace/routerforge/releases/download/"
const LegacyBase = "https://github.com/Fifth-Ace/dns-monitor/releases/download/"
const ForgejoBase = "https://git.fifthace.ru/FifthAce/routerforge/releases/download/"

func safeSegment(s string) bool {
	if s == "" || s == "." || strings.Contains(s, "..") {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._+-", c) {
			return false
		}
	}
	return true
}

// Candidates never accept a host, path or query supplied by a browser or index.
func Candidates(tag, asset string) []string {
	if !strings.HasPrefix(tag, "routerforge-") || !safeSegment(tag) || !safeSegment(asset) {
		return nil
	}
	return []string{GitHubBase + tag + "/" + asset, ForgejoBase + tag + "/" + asset}
}

// GitHubAsset accepts legacy provenance, but canonicalizes the actual transport.
func GitHubAsset(raw string) (tag, asset string, ok bool) {
	for _, base := range []string{GitHubBase, LegacyBase} {
		if tail, found := strings.CutPrefix(raw, base); found {
			a, b, found := strings.Cut(tail, "/")
			if found && len(Candidates(a, b)) == 2 {
				return a, b, true
			}
		}
	}
	return "", "", false
}
func IsMirror(raw string) bool {
	tail, ok := strings.CutPrefix(raw, ForgejoBase)
	if !ok {
		return false
	}
	a, b, ok := strings.Cut(tail, "/")
	return ok && len(Candidates(a, b)) == 2
}

// A mirror cannot introduce a new rolling index without independent freshness
// evidence. A byte-identical previously trusted index is enough to resume.
func MirrorIndexMatches(candidate, trusted []byte) bool {
	return len(trusted) > 0 && bytes.Equal(candidate, trusted)
}

func githubCDN(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && u.Port() == "" && u.Fragment == "" && (u.Host == "release-assets.githubusercontent.com" || u.Host == "objects.githubusercontent.com")
}
func allowedRedirect(to, first *url.URL) bool {
	if to.Scheme != "https" || to.User != nil || to.Port() != "" || to.Fragment != "" {
		return false
	}
	if IsMirror(first.String()) {
		return to.Host == "git.fifthace.ru"
	}
	_, _, ok := GitHubAsset(first.String())
	return ok && githubCDN(to)
}

// Each origin has its own bounded request. Slow DNS/TLS/header failures do not
// consume the entire transaction budget. TLS validation is never disabled.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 8 * time.Second, ResponseHeaderTimeout: 12 * time.Second,
		IdleConnTimeout: 30 * time.Second, DisableCompression: true,
	}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 || len(via) >= 5 || !allowedRedirect(req.URL, via[0].URL) {
			return errors.New("untrusted release redirect")
		}
		return nil
	}}
}
