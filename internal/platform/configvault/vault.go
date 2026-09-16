package configvault

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	SchemaVersion           = 1
	defaultMaxArtifactBytes = int64(2 << 20)
	defaultMaxSnapshotBytes = int64(8 << 20)
	defaultMaxStoreBytes    = int64(32 << 20)
	defaultMaxSnapshots     = 32
	manifestReadLimit       = int64(1 << 20)
)

var (
	idPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	snapshotPattern = regexp.MustCompile(`^cfg-[0-9]+-[0-9a-f]{16}$`)
	shaPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Options struct {
	Root             string
	AllowedRoots     []string
	MaxArtifactBytes int64
	MaxSnapshotBytes int64
	MaxStoreBytes    int64
	MaxSnapshots     int
}

type Store struct {
	root             string
	allowedRoots     []string
	maxArtifactBytes int64
	maxSnapshotBytes int64
	maxStoreBytes    int64
	maxSnapshots     int
	now              func() time.Time
}

type ArtifactSpec struct {
	ID   string
	Path string
}

type CaptureRequest struct {
	Component     string
	Reason        string
	TransactionID string
	Artifacts     []ArtifactSpec
}

type ArtifactRecord struct {
	ID         string    `json:"id"`
	SourcePath string    `json:"source_path"`
	SHA256     string    `json:"sha256"`
	Size       int64     `json:"size"`
	Mode       uint32    `json:"mode"`
	ModifiedAt time.Time `json:"modified_at"`
}

type Manifest struct {
	SchemaVersion int              `json:"schema_version"`
	ID            string           `json:"id"`
	Component     string           `json:"component"`
	Reason        string           `json:"reason,omitempty"`
	TransactionID string           `json:"transaction_id,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	Artifacts     []ArtifactRecord `json:"artifacts"`
	TotalBytes    int64            `json:"total_bytes"`
}

type State struct {
	LastWorking string    `json:"last_working,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Retention struct {
	MaxSnapshots  int   `json:"max_snapshots"`
	MaxStoreBytes int64 `json:"max_store_bytes"`
}

func New(options Options) (*Store, error) {
	root := filepath.Clean(strings.TrimSpace(options.Root))
	if root == "." || root == "" || !filepath.IsAbs(root) {
		return nil, errors.New("config vault root must be an absolute path")
	}
	if len(options.AllowedRoots) == 0 {
		return nil, errors.New("config vault requires at least one allowed source root")
	}

	allowed := make([]string, 0, len(options.AllowedRoots))
	for _, raw := range options.AllowedRoots {
		value := filepath.Clean(strings.TrimSpace(raw))
		if value == "." || value == "" || !filepath.IsAbs(value) {
			return nil, fmt.Errorf("allowed root must be absolute: %q", raw)
		}
		allowed = append(allowed, value)
	}
	sort.Strings(allowed)

	store := &Store{
		root:             root,
		allowedRoots:     allowed,
		maxArtifactBytes: positiveOr(options.MaxArtifactBytes, defaultMaxArtifactBytes),
		maxSnapshotBytes: positiveOr(options.MaxSnapshotBytes, defaultMaxSnapshotBytes),
		maxStoreBytes:    positiveOr(options.MaxStoreBytes, defaultMaxStoreBytes),
		maxSnapshots:     options.MaxSnapshots,
		now:              time.Now,
	}
	if store.maxSnapshots <= 0 {
		store.maxSnapshots = defaultMaxSnapshots
	}
	if store.maxSnapshotBytes < store.maxArtifactBytes {
		return nil, errors.New("max snapshot bytes must be >= max artifact bytes")
	}
	if store.maxStoreBytes < store.maxSnapshotBytes {
		return nil, errors.New("max store bytes must be >= max snapshot bytes")
	}
	return store, nil
}

func positiveOr(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) Retention() Retention {
	return Retention{MaxSnapshots: s.maxSnapshots, MaxStoreBytes: s.maxStoreBytes}
}

func (s *Store) Capture(request CaptureRequest) (Manifest, error) {
	if !idPattern.MatchString(request.Component) {
		return Manifest{}, errors.New("invalid component id")
	}
	if len(request.Reason) > 512 {
		return Manifest{}, errors.New("snapshot reason is too long")
	}
	if len(request.TransactionID) > 128 {
		return Manifest{}, errors.New("transaction id is too long")
	}
	if len(request.Artifacts) == 0 {
		return Manifest{}, errors.New("snapshot requires at least one artifact")
	}
	if err := s.ensureLayout(); err != nil {
		return Manifest{}, err
	}

	seen := make(map[string]struct{}, len(request.Artifacts))
	records := make([]ArtifactRecord, 0, len(request.Artifacts))
	var total int64
	for _, spec := range request.Artifacts {
		if !idPattern.MatchString(spec.ID) {
			return Manifest{}, fmt.Errorf("invalid artifact id %q", spec.ID)
		}
		if _, exists := seen[spec.ID]; exists {
			return Manifest{}, fmt.Errorf("duplicate artifact id %q", spec.ID)
		}
		seen[spec.ID] = struct{}{}

		record, content, err := s.readArtifact(spec)
		if err != nil {
			return Manifest{}, fmt.Errorf("artifact %s: %w", spec.ID, err)
		}
		if total > s.maxSnapshotBytes-record.Size {
			return Manifest{}, fmt.Errorf("snapshot exceeds %d bytes", s.maxSnapshotBytes)
		}
		total += record.Size
		if err := s.ensureObject(record.SHA256, content); err != nil {
			return Manifest{}, fmt.Errorf("artifact %s object: %w", spec.ID, err)
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })

	now := s.now().UTC()
	id, err := newSnapshotID(now)
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		ID:            id,
		Component:     request.Component,
		Reason:        strings.TrimSpace(request.Reason),
		TransactionID: strings.TrimSpace(request.TransactionID),
		CreatedAt:     now,
		Artifacts:     records,
		TotalBytes:    total,
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	payload = append(payload, '\n')
	if int64(len(payload)) > manifestReadLimit {
		return Manifest{}, errors.New("snapshot manifest is unexpectedly large")
	}

	manifestPath := s.manifestPath(id)
	if err := safety.WriteFileAtomic(manifestPath, payload, 0600); err != nil {
		return Manifest{}, err
	}
	if err := s.prune(id); err != nil {
		_ = os.Remove(manifestPath)
		_ = s.gcObjects()
		return Manifest{}, err
	}
	return manifest, nil
}

func (s *Store) List() ([]Manifest, error) {
	entries, err := os.ReadDir(s.snapshotsDir())
	if errors.Is(err, os.ErrNotExist) {
		return []Manifest{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]Manifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if !snapshotPattern.MatchString(id) {
			continue
		}
		manifest, err := s.Get(id)
		if err != nil {
			return nil, err
		}
		result = append(result, manifest)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) Get(id string) (Manifest, error) {
	if !snapshotPattern.MatchString(id) {
		return Manifest{}, errors.New("invalid snapshot id")
	}
	path := s.manifestPath(id)
	info, err := os.Lstat(path)
	if err != nil {
		return Manifest{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Manifest{}, errors.New("snapshot manifest must be a regular non-symlink file")
	}
	if info.Size() > manifestReadLimit {
		return Manifest{}, errors.New("snapshot manifest exceeds read limit")
	}

	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, manifestReadLimit+1))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.ID != id || !idPattern.MatchString(manifest.Component) {
		return Manifest{}, errors.New("invalid snapshot manifest")
	}
	return manifest, nil
}

func (s *Store) MarkLastWorking(id string) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	if err := s.ensureLayout(); err != nil {
		return err
	}
	state := State{LastWorking: id, UpdatedAt: s.now().UTC()}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return safety.WriteFileAtomic(s.statePath(), payload, 0600)
}

func (s *Store) State() (State, error) {
	info, err := os.Lstat(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return State{}, errors.New("config vault state must be a regular non-symlink file")
	}
	payload, err := os.ReadFile(s.statePath())
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(payload, &state); err != nil {
		return State{}, err
	}
	if state.LastWorking != "" && !snapshotPattern.MatchString(state.LastWorking) {
		return State{}, errors.New("config vault state contains invalid snapshot id")
	}
	return state, nil
}

func (s *Store) readArtifact(spec ArtifactSpec) (ArtifactRecord, []byte, error) {
	path := filepath.Clean(strings.TrimSpace(spec.Path))
	if !filepath.IsAbs(path) {
		return ArtifactRecord{}, nil, errors.New("source path must be absolute")
	}
	if !s.allowed(path) {
		return ArtifactRecord{}, nil, errors.New("source path is outside allowed roots")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ArtifactRecord{}, nil, err
	}
	resolved = filepath.Clean(resolved)
	if resolved != path {
		return ArtifactRecord{}, nil, errors.New("source path must not contain symlinks")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return ArtifactRecord{}, nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ArtifactRecord{}, nil, errors.New("source must be a regular non-symlink file")
	}
	if info.Size() > s.maxArtifactBytes {
		return ArtifactRecord{}, nil, fmt.Errorf("source exceeds %d bytes", s.maxArtifactBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return ArtifactRecord{}, nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, s.maxArtifactBytes+1))
	if err != nil {
		return ArtifactRecord{}, nil, err
	}
	if int64(len(content)) > s.maxArtifactBytes {
		return ArtifactRecord{}, nil, fmt.Errorf("source exceeds %d bytes", s.maxArtifactBytes)
	}
	sum := sha256.Sum256(content)
	checksum := hex.EncodeToString(sum[:])
	return ArtifactRecord{
		ID:         spec.ID,
		SourcePath: path,
		SHA256:     checksum,
		Size:       int64(len(content)),
		Mode:       uint32(info.Mode().Perm()),
		ModifiedAt: info.ModTime().UTC(),
	}, content, nil
}

func (s *Store) allowed(path string) bool {
	for _, root := range s.allowedRoots {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
			return true
		}
	}
	return false
}

func (s *Store) ensureObject(checksum string, content []byte) error {
	if !shaPattern.MatchString(checksum) {
		return errors.New("invalid object checksum")
	}
	path := s.objectPath(checksum)
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("existing object is not a regular file")
		}
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(existing)
		if hex.EncodeToString(sum[:]) != checksum {
			return errors.New("existing object checksum mismatch")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return safety.WriteFileAtomic(path, content, 0600)
}

func (s *Store) ensureLayout() error {
	for _, dir := range []string{s.root, s.objectsDir(), s.snapshotsDir()} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) prune(preserveID string) error {
	for {
		manifests, err := s.List()
		if err != nil {
			return err
		}
		usage, err := s.storeBytes()
		if err != nil {
			return err
		}
		if len(manifests) <= s.maxSnapshots && usage <= s.maxStoreBytes {
			return s.gcObjects()
		}

		state, err := s.State()
		if err != nil {
			return err
		}
		victim := ""
		for i := len(manifests) - 1; i >= 0; i-- {
			id := manifests[i].ID
			if id != preserveID && id != state.LastWorking {
				victim = id
				break
			}
		}
		if victim == "" {
			return errors.New("config vault retention cannot be satisfied without deleting a protected snapshot")
		}
		if err := os.Remove(s.manifestPath(victim)); err != nil {
			return err
		}
		if err := s.gcObjects(); err != nil {
			return err
		}
	}
}

func (s *Store) gcObjects() error {
	manifests, err := s.List()
	if err != nil {
		return err
	}
	referenced := make(map[string]struct{})
	for _, manifest := range manifests {
		for _, artifact := range manifest.Artifacts {
			if shaPattern.MatchString(artifact.SHA256) {
				referenced[artifact.SHA256] = struct{}{}
			}
		}
	}

	entries, err := os.ReadDir(s.objectsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !shaPattern.MatchString(entry.Name()) {
			continue
		}
		if _, ok := referenced[entry.Name()]; ok {
			continue
		}
		path := s.objectPath(entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) storeBytes() (int64, error) {
	var total int64
	err := filepath.WalkDir(s.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return total, err
}

func newSnapshotID(now time.Time) (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("cfg-%d-%s", now.UTC().UnixNano(), hex.EncodeToString(suffix[:])), nil
}

func (s *Store) objectsDir() string   { return filepath.Join(s.root, "objects") }
func (s *Store) snapshotsDir() string { return filepath.Join(s.root, "snapshots") }
func (s *Store) objectPath(sum string) string {
	return filepath.Join(s.objectsDir(), sum)
}
func (s *Store) manifestPath(id string) string {
	return filepath.Join(s.snapshotsDir(), id+".json")
}
func (s *Store) statePath() string { return filepath.Join(s.root, "state.json") }

// ReadArtifact returns one snapshot artifact and verifies the backing object
// before exposing its bytes to a restore consumer.
func (s *Store) ReadArtifact(snapshotID, artifactID string) (ArtifactRecord, []byte, error) {
	if !idPattern.MatchString(artifactID) {
		return ArtifactRecord{}, nil, errors.New("invalid artifact id")
	}
	manifest, err := s.Get(snapshotID)
	if err != nil {
		return ArtifactRecord{}, nil, err
	}

	var record ArtifactRecord
	found := false
	for _, candidate := range manifest.Artifacts {
		if candidate.ID == artifactID {
			record = candidate
			found = true
			break
		}
	}
	if !found {
		return ArtifactRecord{}, nil, os.ErrNotExist
	}
	if !shaPattern.MatchString(record.SHA256) {
		return ArtifactRecord{}, nil, errors.New("artifact contains invalid object checksum")
	}
	if record.Size < 0 || record.Size > s.maxArtifactBytes {
		return ArtifactRecord{}, nil, errors.New("artifact size is outside configured limits")
	}

	path := s.objectPath(record.SHA256)
	info, err := os.Lstat(path)
	if err != nil {
		return ArtifactRecord{}, nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ArtifactRecord{}, nil, errors.New("artifact object must be a regular non-symlink file")
	}
	if info.Size() != record.Size {
		return ArtifactRecord{}, nil, errors.New("artifact object size mismatch")
	}

	file, err := os.Open(path)
	if err != nil {
		return ArtifactRecord{}, nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, s.maxArtifactBytes+1))
	if err != nil {
		return ArtifactRecord{}, nil, err
	}
	if int64(len(content)) != record.Size {
		return ArtifactRecord{}, nil, errors.New("artifact object read size mismatch")
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != record.SHA256 {
		return ArtifactRecord{}, nil, errors.New("artifact object checksum mismatch")
	}
	return record, content, nil
}
