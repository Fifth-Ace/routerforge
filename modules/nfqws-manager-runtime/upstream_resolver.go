package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	nfqwsMenuRepo       = "rndnaame/nfqws-menu"
	nfqwsMenuBranch     = "main"
	upstreamFreshTTL    = 15 * time.Minute
	upstreamBodyMax     = 2 << 20
	upstreamCacheFormat = 1
)

var (
	nfqwsMenuAPIBase   = "https://api.github.com"
	nfqwsMenuGitBase   = "https://github.com"
	nfqwsMenuCachePath = "/tmp/routerforge-nfqws-manager-nfqws-menu-ref.json"
	upstreamNow        = time.Now
	upstreamHTTPClient = &http.Client{Timeout: 8 * time.Second}
	nfqwsMenuCacheMu   sync.Mutex
	gitMainRefPattern  = regexp.MustCompile(`(?m)([0-9a-f]{40}) refs/heads/main(?:\x00|\n|$)`)
)

type upstreamCacheRecord struct {
	Format    int       `json:"format"`
	SHA       string    `json:"sha"`
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetched_at"`
}

type upstreamResolution struct {
	Repo               string `json:"repo"`
	Branch             string `json:"branch"`
	SHA                string `json:"sha,omitempty"`
	State              string `json:"state"`
	Source             string `json:"source,omitempty"`
	Cached             bool   `json:"cached"`
	Stale              bool   `json:"stale"`
	FetchedAt          string `json:"fetched_at,omitempty"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	Error              string `json:"error,omitempty"`
	RateLimitRemaining *int   `json:"rate_limit_remaining,omitempty"`
	RateLimitReset     string `json:"rate_limit_reset,omitempty"`
}

type upstreamHTTPError struct {
	Status         int
	Message        string
	RateLimited    bool
	Remaining      *int
	RateLimitReset string
}

func (e *upstreamHTTPError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

func registerUpstreamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/upstream/nfqws-menu", getOnly(handleNfqwsMenuUpstream))
}

func handleNfqwsMenuUpstream(w http.ResponseWriter, _ *http.Request) {
	resolution := resolveNfqwsMenuRef()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, resolution)
}

func resolveNfqwsMenuRef() upstreamResolution {
	nfqwsMenuCacheMu.Lock()
	defer nfqwsMenuCacheMu.Unlock()

	now := upstreamNow().UTC()
	cache := readUpstreamCache()
	if validCommitSHA(cache.SHA) && now.Before(cache.FetchedAt.Add(upstreamFreshTTL)) {
		return resolutionFromCache(cache, now, false, "")
	}

	apiSHA, apiMeta, apiErr := fetchNfqwsMenuAPIRef()
	if apiErr == nil && validCommitSHA(apiSHA) {
		record := upstreamCacheRecord{Format: upstreamCacheFormat, SHA: apiSHA, Source: "github-api", FetchedAt: now}
		writeUpstreamCache(record)
		result := resolutionFromCache(record, now, false, "")
		result.RateLimitRemaining = apiMeta.Remaining
		result.RateLimitReset = apiMeta.RateLimitReset
		return result
	}

	gitSHA, gitErr := fetchNfqwsMenuGitRef()
	if gitErr == nil && validCommitSHA(gitSHA) {
		record := upstreamCacheRecord{Format: upstreamCacheFormat, SHA: gitSHA, Source: "git-smart-http", FetchedAt: now}
		writeUpstreamCache(record)
		result := resolutionFromCache(record, now, false, "")
		if apiMeta != nil {
			result.RateLimitRemaining = apiMeta.Remaining
			result.RateLimitReset = apiMeta.RateLimitReset
		}
		return result
	}

	combined := joinUpstreamErrors(apiErr, gitErr)
	if validCommitSHA(cache.SHA) {
		result := resolutionFromCache(cache, now, true, combined)
		if apiMeta != nil {
			result.RateLimitRemaining = apiMeta.Remaining
			result.RateLimitReset = apiMeta.RateLimitReset
		}
		return result
	}

	state := "offline"
	if isRateLimited(apiErr) {
		state = "rate_limited"
	}
	result := upstreamResolution{
		Repo: nfqwsMenuRepo, Branch: nfqwsMenuBranch,
		State: state, Error: combined,
	}
	if apiMeta != nil {
		result.RateLimitRemaining = apiMeta.Remaining
		result.RateLimitReset = apiMeta.RateLimitReset
	}
	return result
}

func resolutionFromCache(cache upstreamCacheRecord, now time.Time, stale bool, errText string) upstreamResolution {
	state := "synced"
	source := cache.Source
	if source == "" {
		source = "cache"
	}
	if stale {
		state = "stale_cache"
		source = "cache:" + source
	}
	return upstreamResolution{
		Repo: nfqwsMenuRepo, Branch: nfqwsMenuBranch, SHA: cache.SHA,
		State: state, Source: source, Cached: source == "cache" || strings.HasPrefix(source, "cache:"),
		Stale: stale, FetchedAt: cache.FetchedAt.UTC().Format(time.RFC3339),
		ExpiresAt: cache.FetchedAt.Add(upstreamFreshTTL).UTC().Format(time.RFC3339),
		Error:     errText,
	}
}

type upstreamAPIMeta struct {
	Remaining      *int
	RateLimitReset string
}

func fetchNfqwsMenuAPIRef() (string, *upstreamAPIMeta, error) {
	url := strings.TrimRight(nfqwsMenuAPIBase, "/") + "/repos/" + nfqwsMenuRepo + "/commits/" + nfqwsMenuBranch
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RouterForge-nfqws-manager/"+version)

	resp, err := upstreamHTTPClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	meta := &upstreamAPIMeta{
		Remaining:      parseOptionalInt(resp.Header.Get("X-RateLimit-Remaining")),
		RateLimitReset: parseRateLimitReset(resp.Header.Get("X-RateLimit-Reset")),
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", meta, &upstreamHTTPError{
			Status: resp.StatusCode, Message: fmt.Sprintf("GitHub API HTTP %d", resp.StatusCode),
			RateLimited: resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests,
			Remaining:   meta.Remaining, RateLimitReset: meta.RateLimitReset,
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamBodyMax+1))
	if err != nil {
		return "", meta, err
	}
	if len(body) > upstreamBodyMax {
		return "", meta, errors.New("GitHub API response exceeds safety limit")
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", meta, fmt.Errorf("decode GitHub API response: %w", err)
	}
	payload.SHA = strings.ToLower(strings.TrimSpace(payload.SHA))
	if !validCommitSHA(payload.SHA) {
		return "", meta, errors.New("GitHub API returned invalid commit SHA")
	}
	return payload.SHA, meta, nil
}

func fetchNfqwsMenuGitRef() (string, error) {
	url := strings.TrimRight(nfqwsMenuGitBase, "/") + "/" + nfqwsMenuRepo + ".git/info/refs?service=git-upload-pack"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/x-git-upload-pack-advertisement")
	req.Header.Set("User-Agent", "git/2.39.0 RouterForge-nfqws-manager/"+version)

	resp, err := upstreamHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Git smart HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamBodyMax+1))
	if err != nil {
		return "", err
	}
	if len(body) > upstreamBodyMax {
		return "", errors.New("Git smart HTTP response exceeds safety limit")
	}
	match := gitMainRefPattern.FindSubmatch(body)
	if len(match) != 2 {
		return "", errors.New("Git smart HTTP main ref not found")
	}
	sha := strings.ToLower(string(match[1]))
	if !validCommitSHA(sha) {
		return "", errors.New("Git smart HTTP returned invalid commit SHA")
	}
	return sha, nil
}

func validCommitSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func isRateLimited(err error) bool {
	var httpErr *upstreamHTTPError
	return errors.As(err, &httpErr) && httpErr.RateLimited
}

func joinUpstreamErrors(apiErr, gitErr error) string {
	parts := []string{}
	if apiErr != nil {
		parts = append(parts, apiErr.Error())
	}
	if gitErr != nil {
		parts = append(parts, gitErr.Error())
	}
	return strings.Join(parts, "; ")
}

func parseOptionalInt(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &number
}

func parseRateLimitReset(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func readUpstreamCache() upstreamCacheRecord {
	data, err := os.ReadFile(nfqwsMenuCachePath)
	if err != nil {
		return upstreamCacheRecord{}
	}
	var record upstreamCacheRecord
	if json.Unmarshal(data, &record) != nil || record.Format != upstreamCacheFormat || !validCommitSHA(record.SHA) || record.FetchedAt.IsZero() {
		return upstreamCacheRecord{}
	}
	record.SHA = strings.ToLower(record.SHA)
	return record
}

func writeUpstreamCache(record upstreamCacheRecord) {
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	_ = safety.WriteFileAtomic(nfqwsMenuCachePath, append(data, '\n'), 0600)
}
