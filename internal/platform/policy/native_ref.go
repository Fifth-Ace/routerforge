package policy

import (
	"regexp"
	"strings"
)

// NativeListReference denotes domain membership only, never a native rule action.
// A pinned value is immutable until the catalogue is explicitly edited and saved.
type NativeListReference struct{ Module, Name, SHA256 string }

var nativeDNSName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]{0,39}$`)
var nativeNFName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]{0,74}\.list$`)
var nativeSHA = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ParseNativeListReference(value string) (NativeListReference, bool) {
	var r NativeListReference
	parts := strings.Split(value, "@")
	if len(parts) > 2 {
		return r, false
	}
	if len(parts) == 2 {
		if !nativeSHA.MatchString(parts[1]) {
			return r, false
		}
		r.SHA256 = parts[1]
	}
	module, name, ok := strings.Cut(parts[0], ".")
	if !ok || strings.Contains(name, "..") {
		return r, false
	}
	switch module {
	case "dns":
		if !nativeDNSName.MatchString(name) {
			return r, false
		}
	case "nfqws":
		if !nativeNFName.MatchString(name) {
			return r, false
		}
	default:
		return r, false
	}
	r.Module = module
	r.Name = name
	return r, true
}
func (r NativeListReference) Key() string              { return r.Module + "." + r.Name }
func (r NativeListReference) Pinned(sha string) string { return r.Key() + "@" + sha }

// Preserve old unqualified references as unresolved drafts, but never accept a
// malformed pin or ambiguous native namespace as a valid new reference.
func validListReference(s string) bool {
	if strings.Contains(s, "@") {
		_, ok := ParseNativeListReference(s)
		return ok
	}
	return groupID.MatchString(s)
}
