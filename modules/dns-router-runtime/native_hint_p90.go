package dnsrouter

// P9.0: opaque, deterministic selectors for special-character Keenetic hints.
// A selector is a name, NEVER an address to dial. Every DNS query resolves it
// again against the current native ndnproxy inventory and original DNS zone.
import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
)

var p90HintPlain = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$`)
var p90Opaque = regexp.MustCompile(`^sha256-[a-f0-9]{64}$`)

// nativeSelectorHintP90 returns a stable selector for an unambiguous hint.
// Legacy host/IP hints retain their old selectors for backward compatibility.
func nativeSelectorHintP90(hint string) string {
	if len(hint) == 0 || len(hint) > 120 || strings.IndexFunc(hint, func(r rune) bool { return r < 33 || r > 126 }) >= 0 {
		return ""
	}
	if p90HintPlain.MatchString(hint) && !p90Opaque.MatchString(hint) {
		return hint
	}
	if strings.HasPrefix(hint, "https://") {
		endpoint := hint
		if strings.HasSuffix(endpoint, "@dnsm") {
			endpoint = strings.TrimSuffix(endpoint, "@dnsm")
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || parsed.Opaque != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
			return ""
		}
		// The URL is used ONLY as a native identity. DNS Router never dials this URL.
		sum := sha256.Sum256([]byte(hint))
		return "sha256-" + hex.EncodeToString(sum[:])
	}
	return ""
}

func nativeHintMatchesP90(selector, original string) bool {
	computed := nativeSelectorHintP90(original)
	return computed != "" && computed == selector
}
