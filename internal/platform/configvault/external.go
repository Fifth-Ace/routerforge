package configvault

// Authenticated, bounded transport for existing Config Vault manifests. Import
// creates a new local snapshot, never writes any managed configuration file.
import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const ExternalLimit = 14 << 20
const externalMagic = "RFVAULT-ENC-1\n"

var errExternal = errors.New("invalid encrypted Config Vault archive or key")

type externalBundle struct {
	Manifest Manifest `json:"manifest"`
	Payloads [][]byte `json:"payloads"`
}

func externalCipher(key string) (cipher.AEAD, error) {
	if len(key) != 64 {
		return nil, errExternal
	}
	raw, e := hex.DecodeString(key)
	if e != nil || len(raw) != 32 {
		return nil, errExternal
	}
	block, e := aes.NewCipher(raw)
	for i := range raw {
		raw[i] = 0
	}
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}
func (s *Store) externalValidate(b externalBundle) error {
	m := b.Manifest
	if m.SchemaVersion != SchemaVersion || m.Component != "admin" || !snapshotPattern.MatchString(m.ID) || len(m.Version) > 128 || len(m.Artifacts) > 128 || len(m.Artifacts) != len(b.Payloads) {
		return errExternal
	}
	ids, paths := map[string]bool{}, map[string]bool{}
	var total int64
	for i, a := range m.Artifacts {
		data := b.Payloads[i]
		path := a.SourcePath
		if !idPattern.MatchString(a.ID) || ids[a.ID] || paths[path] || !filepath.IsAbs(path) || filepath.Clean(path) != path || !s.allowed(path) || a.Mode > 0777 || a.Size < 0 || a.Size > s.maxArtifactBytes || int64(len(data)) != a.Size || !shaPattern.MatchString(a.SHA256) {
			return errExternal
		}
		for _, root := range s.allowedRoots {
			if path == root {
				return errExternal
			}
		}
		if strings.ContainsAny(path, "\x00\r\n") || len(path) > 1024 {
			return errExternal
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != a.SHA256 {
			return errExternal
		}
		if total > s.maxSnapshotBytes-a.Size {
			return errExternal
		}
		total += a.Size
		ids[a.ID] = true
		paths[path] = true
	}
	if total != m.TotalBytes {
		return errExternal
	}
	return nil
}
func (s *Store) ExportEncrypted(id, key string) ([]byte, error) {
	a, e := externalCipher(key)
	if e != nil {
		return nil, e
	}
	m, e := s.Get(id)
	if e != nil {
		return nil, e
	}
	if len(m.Artifacts) > 128 || m.TotalBytes < 0 || m.TotalBytes > s.maxSnapshotBytes {
		return nil, errExternal
	}
	b := externalBundle{Manifest: m, Payloads: make([][]byte, 0, len(m.Artifacts))}
	var total int64
	for _, x := range m.Artifacts {
		rec, data, e := s.ReadArtifact(id, x.ID)
		if e != nil || rec != x || total > s.maxSnapshotBytes-int64(len(data)) {
			return nil, errExternal
		}
		total += int64(len(data))
		b.Payloads = append(b.Payloads, data)
	}
	if e = s.externalValidate(b); e != nil {
		return nil, e
	}
	plain, e := json.Marshal(b)
	if e != nil || len(plain) > ExternalLimit-len(externalMagic)-a.NonceSize()-a.Overhead() {
		return nil, errExternal
	}
	nonce := make([]byte, a.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	out := append([]byte(externalMagic), nonce...)
	return a.Seal(out, nonce, plain, []byte(externalMagic)), nil
}
func (s *Store) ImportEncrypted(blob []byte, key string) (Manifest, error) {
	var empty Manifest
	a, e := externalCipher(key)
	if e != nil {
		return empty, e
	}
	n := len(externalMagic) + a.NonceSize()
	if len(blob) < n+a.Overhead() || len(blob) > ExternalLimit || !bytes.HasPrefix(blob, []byte(externalMagic)) {
		return empty, errExternal
	}
	plain, e := a.Open(nil, blob[len(externalMagic):n], blob[n:], []byte(externalMagic))
	if e != nil {
		return empty, errExternal
	}
	var b externalBundle
	d := json.NewDecoder(bytes.NewReader(plain))
	d.DisallowUnknownFields()
	if d.Decode(&b) != nil {
		return empty, errExternal
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return empty, errExternal
	}
	if e = s.externalValidate(b); e != nil {
		return empty, e
	}
	// Check capacity before any write. Conservative: include all payload bytes,
	// even deduplicated objects. No retention/GC on import.
	list, e := s.List()
	if e != nil {
		return empty, e
	}
	usage, e := s.storeBytes()
	if e != nil {
		return empty, e
	}
	if len(list) >= s.maxSnapshots || usage > s.maxStoreBytes-manifestReadLimit-b.Manifest.TotalBytes {
		return empty, errors.New("config vault no-prune import limit")
	}
	if e = s.ensureLayout(); e != nil {
		return empty, e
	}
	for _, dir := range []string{s.root, s.objectsDir(), s.snapshotsDir()} {
		info, e := os.Lstat(dir)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return empty, errExternal
		}
	}
	for i, x := range b.Manifest.Artifacts {
		if info, e := os.Lstat(s.objectPath(x.SHA256)); e == nil && (!info.Mode().IsRegular() || info.Size() != x.Size) {
			return empty, errExternal
		} else if e != nil && !errors.Is(e, os.ErrNotExist) {
			return empty, e
		}
		if e = s.ensureObject(x.SHA256, b.Payloads[i]); e != nil {
			return empty, e
		}
	}
	m := b.Manifest
	source := m.ID
	m.CreatedAt = s.now().UTC()
	m.ID, e = newSnapshotID(m.CreatedAt)
	if e != nil {
		return empty, e
	}
	m.Reason = "Imported encrypted snapshot " + source
	m.TransactionID = ""
	payload, e := json.MarshalIndent(m, "", "  ")
	if e != nil || int64(len(payload)+1) > manifestReadLimit {
		return empty, errExternal
	}
	payload = append(payload, '\n')
	if _, e = os.Lstat(s.manifestPath(m.ID)); !errors.Is(e, os.ErrNotExist) {
		return empty, errExternal
	}
	if e = safety.WriteFileAtomic(s.manifestPath(m.ID), payload, 0600); e != nil {
		return empty, e
	}
	return m, nil
}
