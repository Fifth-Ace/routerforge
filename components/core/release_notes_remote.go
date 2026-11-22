package main

// Public RouterForge release notes are data, never executable frontend code.
// A read-only Core proxy prevents browser CORS dependencies, validates hashes
// and persists a bounded offline cache independent of installed IPKs.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	notesCacheDir   = "/opt/var/cache/routerforge/release-notes"
	notesIndexLimit = 128 << 10
	notesTextLimit  = 256 << 10
)

var (
	notesVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-beta\.\d+)?$`)
	notesSHA256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	notesMutex          sync.Mutex
)

type notesAsset struct {
	Asset  string `json:"asset"`
	SHA256 string `json:"sha256"`
}
type notesRelease struct {
	Version string      `json:"version"`
	Title   string      `json:"title"`
	Channel string      `json:"channel,omitempty"`
	Source  string      `json:"source,omitempty"`
	Summary *notesAsset `json:"summary,omitempty"`
	Story   *notesAsset `json:"story,omitempty"`
}
type notesIndex struct {
	SchemaVersion int            `json:"schema_version"`
	Channel       string         `json:"channel"`
	Releases      []notesRelease `json:"releases"`
}

func notesChannel(value string) bool { return value == "beta" || value == "stable" }
func expectedNotesAsset(channel, version, kind string) string {
	return fmt.Sprintf("routerforge-notes-%s-%s-%s.md", channel, version, kind)
}
func validNotesAsset(asset *notesAsset, channel, version, kind string) bool {
	if asset == nil {
		return true
	}
	return asset.Asset == expectedNotesAsset(channel, version, kind) && notesSHA256Pattern.MatchString(asset.SHA256)
}
func parseNotesIndex(data []byte, channel string) (notesIndex, error) {
	var doc notesIndex
	if !notesChannel(channel) || len(data) == 0 || len(data) > notesIndexLimit {
		return doc, errors.New("invalid notes channel or index size")
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	if doc.SchemaVersion != 2 || doc.Channel != channel || len(doc.Releases) > 128 {
		return doc, errors.New("invalid release notes index schema/channel/count")
	}
	seen := map[string]bool{}
	for _, release := range doc.Releases {
		if !notesVersionPattern.MatchString(release.Version) ||
			(channel == "beta") != strings.Contains(release.Version, "-beta.") ||
			len(release.Title) > 240 ||
			(release.Summary == nil && release.Story == nil) ||
			!validNotesAsset(release.Summary, channel, release.Version, "summary") ||
			!validNotesAsset(release.Story, channel, release.Version, "story") ||
			seen[release.Version] {
			return notesIndex{}, errors.New("invalid or duplicate release notes entry")
		}
		seen[release.Version] = true
	}
	return doc, nil
}
func notesIndexFilename(channel string) string {
	return "routerforge-notes-" + channel + "-index.json"
}
func notesFetch(ctx context.Context, channel, asset string, max int64) ([]byte, error) {
	if !notesChannel(channel) || asset == "" || strings.ContainsAny(asset, "/\\") ||
		strings.Contains(asset, "..") || !strings.HasPrefix(asset, "routerforge-notes-"+channel+"-") {
		return nil, errors.New("unsafe notes asset")
	}
	client := &http.Client{
		Timeout: 7 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) >= 8 {
				return errors.New("unsafe notes redirect")
			}
			return nil
		},
	}
	var lastErr error
	for _, repo := range []string{routerForgeCanonicalRepository, routerForgeLegacyRepository} {
		url := fmt.Sprintf("https://github.com/%s/releases/download/routerforge-%s/%s", repo, channel, asset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "RouterForge-release-notes/1")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("notes asset HTTP %d", resp.StatusCode)
			_ = resp.Body.Close()
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, max+1))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if len(data) == 0 || int64(len(data)) > max {
			return nil, errors.New("notes asset exceeds size limit")
		}
		return data, nil
	}
	if lastErr == nil {
		lastErr = errors.New("notes unavailable")
	}
	return nil, lastErr
}
func notesReadCache(asset string, limit int64) ([]byte, os.FileInfo, error) {
	p := filepath.Join(notesCacheDir, asset)
	info, err := os.Stat(p)
	if err != nil {
		return nil, nil, err
	}
	if info.Size() < 1 || info.Size() > limit {
		return nil, nil, errors.New("invalid notes cache size")
	}
	data, err := os.ReadFile(p)
	return data, info, err
}
func notesSaveCache(asset string, data []byte) {
	if err := os.MkdirAll(notesCacheDir, 0755); err != nil {
		return
	}
	_ = safety.WriteFileAtomic(filepath.Join(notesCacheDir, asset), data, 0644)
}
func notesIndexFor(ctx context.Context, channel string) (notesIndex, bool, error) {
	var empty notesIndex
	if !notesChannel(channel) {
		return empty, false, errors.New("invalid notes channel")
	}
	notesMutex.Lock()
	defer notesMutex.Unlock()
	asset := notesIndexFilename(channel)
	cached, info, cacheErr := notesReadCache(asset, notesIndexLimit)
	if cacheErr == nil && time.Since(info.ModTime()) < 15*time.Minute {
		if doc, err := parseNotesIndex(cached, channel); err == nil {
			return doc, false, nil
		}
	}
	data, remoteErr := notesFetch(ctx, channel, asset, notesIndexLimit)
	if remoteErr == nil {
		if doc, err := parseNotesIndex(data, channel); err == nil {
			notesSaveCache(asset, data)
			return doc, true, nil
		} else {
			remoteErr = err
		}
	}
	if cacheErr == nil {
		if doc, err := parseNotesIndex(cached, channel); err == nil {
			return doc, false, nil
		}
	}
	return empty, false, remoteErr
}
func notesTextFor(ctx context.Context, channel, version, kind string) ([]byte, error) {
	if !notesChannel(channel) || !notesVersionPattern.MatchString(version) || (kind != "summary" && kind != "story") {
		return nil, errors.New("invalid notes request")
	}
	index, _, err := notesIndexFor(ctx, channel)
	if err != nil {
		return nil, err
	}
	var item *notesAsset
	for _, entry := range index.Releases {
		if entry.Version == version {
			if kind == "summary" {
				item = entry.Summary
			} else {
				item = entry.Story
			}
			break
		}
	}
	if item == nil {
		return nil, os.ErrNotExist
	}
	valid := func(data []byte) bool {
		if len(data) == 0 || len(data) > notesTextLimit {
			return false
		}
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:]) == item.SHA256
	}
	cached, _, cacheErr := notesReadCache(item.Asset, notesTextLimit)
	if cacheErr == nil && valid(cached) {
		return cached, nil
	}
	data, err := notesFetch(ctx, channel, item.Asset, notesTextLimit)
	if err != nil {
		return nil, err
	}
	if !valid(data) {
		return nil, errors.New("release notes SHA256 mismatch")
	}
	notesSaveCache(item.Asset, data)
	return data, nil
}
func registerReleaseNotesHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/release-notes/catalog", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		result := struct {
			SchemaVersion int            `json:"schema_version"`
			Releases      []notesRelease `json:"releases"`
			Online        bool           `json:"online"`
		}{SchemaVersion: 2, Releases: []notesRelease{}}
		for _, channel := range []string{"stable", "beta"} {
			index, online, err := notesIndexFor(ctx, channel)
			if err != nil {
				continue
			}
			if online {
				result.Online = true
			}
			for _, entry := range index.Releases {
				entry.Channel, entry.Source = channel, "remote"
				result.Releases = append(result.Releases, entry)
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	})
	mux.HandleFunc("/api/release-notes/text", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		if len(q) != 3 || len(q["channel"]) != 1 || len(q["version"]) != 1 || len(q["kind"]) != 1 {
			http.Error(w, "invalid notes request", http.StatusBadRequest)
			return
		}
		text, err := notesTextFor(r.Context(), q.Get("channel"), q.Get("version"), q.Get("kind"))
		if err != nil {
			http.Error(w, "release note unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(text)
	})
}
