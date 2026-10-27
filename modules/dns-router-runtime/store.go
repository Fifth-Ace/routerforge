package dnsrouter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const MaxConfigBytes int64 = 4 << 20

var ErrStale = errors.New("configuration revision changed; refresh before saving")

type ConfigStore struct {
	path string
	mu   sync.Mutex
}

func NewConfigStore(path string) *ConfigStore { return &ConfigStore{path: path} }

func EmptyConfig() Config {
	return Config{SchemaVersion: SchemaVersion, Upstreams: []Upstream{}, Rules: []Rule{}}
}

func DecodeConfig(reader io.Reader) (Config, error) {
	var cfg Config
	dec := json.NewDecoder(io.LimitReader(reader, MaxConfigBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return cfg, errors.New("multiple JSON objects are not permitted")
		}
		return cfg, fmt.Errorf("trailing JSON: %w", err)
	}
	if _, err := Compile(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func revision(cfg Config) string {
	data, _ := json.Marshal(cfg)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (s *ConfigStore) loadLocked() (Config, string, error) {
	if s.path == "" {
		return Config{}, "", errors.New("config path is empty")
	}
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := EmptyConfig()
		return cfg, revision(cfg), nil
	}
	if err != nil {
		return Config{}, "", err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxConfigBytes {
		return Config{}, "", errors.New("configuration must be a regular file up to 4 MiB")
	}
	f, err := os.Open(s.path)
	if err != nil {
		return Config{}, "", err
	}
	defer f.Close()
	cfg, err := DecodeConfig(f)
	if err != nil {
		return Config{}, "", err
	}
	return cfg, revision(cfg), nil
}

func (s *ConfigStore) Load() (Config, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *ConfigStore) Save(cfg Config, expected string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(expected) == 0 {
		return "", errors.New("If-Match revision required")
	}
	_, current, err := s.loadLocked()
	if err != nil {
		return "", err
	}
	if current != expected {
		return "", ErrStale
	}
	if _, err := Compile(cfg); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if len(data) > int(MaxConfigBytes) {
		return "", errors.New("configuration exceeds 4 MiB")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".dns-router-stage-*")
	if err != nil {
		return "", err
	}
	pending := f.Name()
	defer os.Remove(pending)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(pending, s.path); err != nil {
		return "", err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return revision(cfg), nil
}

// ExportContent returns an immutable snapshot; it never modifies the store.
func ExportContent(cfg Config) []byte {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return append(bytes.TrimRight(data, "\n"), '\n')
}
