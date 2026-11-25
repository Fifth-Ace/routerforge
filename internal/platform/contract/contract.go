// Package contract implements the bounded v1 wire contract. No network or credential I/O.
package contract

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

//go:embed schema.json
var schemaBytes []byte

const MaxDocument = 2 << 20

var root map[string]any
var patterns = map[string]*regexp.Regexp{}

func init() {
	if err := json.Unmarshal(schemaBytes, &root); err != nil {
		panic("invalid embedded contract")
	}
	var scan func(any)
	scan = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if p, ok := x["pattern"].(string); ok {
				patterns[p] = regexp.MustCompile(p)
			}
			for _, z := range x {
				scan(z)
			}
		case []any:
			for _, z := range x {
				scan(z)
			}
		}
	}
	scan(root)
}
func Schema() []byte       { return append([]byte(nil), schemaBytes...) }
func errAt(p string) error { return fmt.Errorf("invalid contract at %s", p) }

// Decode rejects duplicate keys, trailing values, invalid UTF-8 and excessive depth.
func Decode(b []byte) (any, error) {
	if len(b) > MaxDocument || !utf8.Valid(b) {
		return nil, errAt("$")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		if depth > 32 {
			return nil, errAt("depth")
		}
		t, e := d.Token()
		if e != nil {
			return nil, errAt("json")
		}
		switch t {
		case json.Delim('{'):
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, errAt("key")
				}
				s, ok := k.(string)
				if !ok {
					return nil, errAt("key")
				}
				if _, ok := m[s]; ok {
					return nil, errAt("duplicate key")
				}
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				m[s] = v
			}
			if t, e := d.Token(); e != nil || t != json.Delim('}') {
				return nil, errAt("object")
			}
			return m, nil
		case json.Delim('['):
			a := []any{}
			for d.More() {
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			if t, e := d.Token(); e != nil || t != json.Delim(']') {
				return nil, errAt("array")
			}
			return a, nil
		}
		if n, ok := t.(json.Number); ok {
			v, e := n.Float64()
			if e != nil || math.IsInf(v, 0) || math.IsNaN(v) {
				return nil, errAt("number")
			}
			return v, nil
		}
		return t, nil
	}
	v, e := read(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errAt("trailing data")
	}
	return v, nil
}

var keys = map[string]bool{"$schema": true, "$id": true, "$defs": true, "$ref": true, "type": true, "properties": true, "required": true, "additionalProperties": true, "items": true, "minItems": true, "maxItems": true, "minLength": true, "maxLength": true, "pattern": true, "format": true, "minimum": true, "maximum": true, "enum": true, "const": true, "anyOf": true}

func bound(s map[string]any, k string, n float64, min bool) bool {
	v, ok := s[k].(float64)
	return !ok || (min && n >= v) || (!min && n <= v)
}

var utc = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d{1,9})?Z$`)

func check(s map[string]any, v any, p string) error {
	for k := range s {
		if !keys[k] {
			return errAt("schema keyword")
		}
	}
	if ref, ok := s["$ref"].(string); ok {
		n := strings.TrimPrefix(ref, "#/$defs/")
		d, ok := root["$defs"].(map[string]any)[n].(map[string]any)
		if !ok {
			return errAt("schema ref")
		}
		return check(d, v, p)
	}
	if a, ok := s["anyOf"].([]any); ok {
		for _, x := range a {
			if check(x.(map[string]any), v, p) == nil {
				return nil
			}
		}
		return errAt(p)
	}
	// enum/const in this schema contain scalar values only.
	equal := func(a, b any) bool {
		switch a := a.(type) {
		case string:
			x, ok := b.(string)
			return ok && x == a
		case float64:
			x, ok := b.(float64)
			return ok && x == a
		case bool:
			x, ok := b.(bool)
			return ok && x == a
		case nil:
			return b == nil
		}
		return false
	}
	if a, ok := s["const"]; ok && !equal(a, v) {
		return errAt(p)
	}
	if a, ok := s["enum"].([]any); ok {
		found := false
		for _, x := range a {
			if equal(x, v) {
				found = true
			}
		}
		if !found {
			return errAt(p)
		}
	}
	switch s["type"] {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return errAt(p)
		}
		props := s["properties"].(map[string]any)
		if req, ok := s["required"].([]any); ok {
			for _, k := range req {
				if _, ok := m[k.(string)]; !ok {
					return errAt(p + "." + k.(string))
				}
			}
		}
		for k, x := range m {
			q, ok := props[k].(map[string]any)
			if !ok {
				return errAt(p + ".unknown")
			}
			if e := check(q, x, p+"."+k); e != nil {
				return e
			}
		}
	case "array":
		a, ok := v.([]any)
		if !ok || !bound(s, "minItems", float64(len(a)), true) || !bound(s, "maxItems", float64(len(a)), false) {
			return errAt(p)
		}
		for _, x := range a {
			if e := check(s["items"].(map[string]any), x, p+"[]"); e != nil {
				return e
			}
		}
	case "string":
		x, ok := v.(string)
		if !ok || !bound(s, "minLength", float64(utf8.RuneCountInString(x)), true) || !bound(s, "maxLength", float64(utf8.RuneCountInString(x)), false) {
			return errAt(p)
		}
		if pat, ok := s["pattern"].(string); ok {
			if !patterns[pat].MatchString(x) {
				return errAt(p)
			}
		}
		if s["format"] == "date-time" {
			if _, e := time.Parse(time.RFC3339Nano, x); !utc.MatchString(x) || e != nil {
				return errAt(p)
			}
		}
	case "integer", "number":
		n, ok := v.(float64)
		if !ok || !bound(s, "minimum", n, true) || !bound(s, "maximum", n, false) || (s["type"] == "integer" && (n != math.Trunc(n) || math.Abs(n) > 9007199254740991)) {
			return errAt(p)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return errAt(p)
		}
	case "null":
		if v != nil {
			return errAt(p)
		}
	case nil:
	default:
		return errAt("schema type")
	}
	return nil
}
func obj(v any) map[string]any { return v.(map[string]any) }
func arr(v any) []any          { return v.([]any) }
func str(v any) string         { return v.(string) }
func unique(a []any, key string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, v := range a {
		k := str(obj(v)[key])
		if out[k] {
			return nil, errAt("duplicate " + key)
		}
		out[k] = true
	}
	return out, nil
}
func def(n string) map[string]any { return root["$defs"].(map[string]any)[n].(map[string]any) }

// Catalog validates a standalone catalogue. Public path/ref fields are not credentials.
func ValidateCatalog(b []byte) error {
	v, e := Decode(b)
	if e != nil {
		return e
	}
	if e = check(def("catalog"), v, "catalog"); e != nil {
		return e
	}
	return catalogSem(obj(v))
}
func catalogSem(c map[string]any) error {
	d, e := unique(arr(c["devices"]), "id")
	if e != nil {
		return e
	}
	co, e := unique(arr(c["connections"]), "id")
	if e != nil {
		return e
	}
	eps := []any{}
	for _, v := range arr(c["connections"]) {
		x := obj(v)
		if !d[str(x["deviceId"])] {
			return errAt("connection device")
		}
		for _, v := range arr(x["endpoints"]) {
			ep := obj(v)
			eps = append(eps, v)
			scheme := str(ep["scheme"])
			if !policy.ValidHost(str(ep["host"])) {
				return errAt("host")
			}
			_, port := ep["port"]
			if port == (scheme == "icmp") {
				return errAt("port")
			}
			proto := str(x["protocol"])
			if proto == "sftp" {
				proto = "ssh"
			}
			if scheme != proto || (x["transport"] == "reverse-proxy" && scheme != "http" && scheme != "https") {
				return errAt("protocol")
			}
			if p, ok := ep["publicPath"].(string); ok {
				if (scheme != "http" && scheme != "https") || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "\\ \t\r\n") || strings.IndexFunc(p, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
					return errAt("path")
				}
			}
		}
	}
	if _, e = unique(eps, "id"); e != nil {
		return e
	}
	if _, e = unique(arr(c["checkPolicies"]), "connectionId"); e != nil {
		return e
	}
	for _, v := range arr(c["checkPolicies"]) {
		if !co[str(obj(v)["connectionId"])] {
			return errAt("check connection")
		}
	}
	b, _ := json.Marshal(c["policyObjects"])
	var groups []policy.Group
	if e = json.Unmarshal(b, &groups); e != nil {
		return errAt("policy objects")
	}
	return policy.ValidateGroups(groups, d)
}
func ValidateDocument(b []byte) error {
	v, e := Decode(b)
	if e != nil {
		return e
	}
	if e = check(root, v, "$"); e != nil {
		return e
	}
	f := obj(v)
	c := obj(f["catalog"])
	if e = catalogSem(c); e != nil {
		return e
	}
	d, _ := unique(arr(c["devices"]), "id")
	co := map[string]map[string]any{}
	for _, v := range arr(c["connections"]) {
		x := obj(v)
		co[str(x["id"])] = x
	}
	if _, e = unique(arr(f["snapshots"]), "snapshotId"); e != nil {
		return e
	}
	if _, e = unique(arr(f["events"]), "id"); e != nil {
		return e
	}
	for _, v := range arr(f["snapshots"]) {
		x := obj(v)
		y := co[str(x["connectionId"])]
		if y == nil || x["deviceId"] != y["deviceId"] || x["adapterId"] != y["adapterId"] || ((x["latencyMs"] == nil) != (x["latencyKind"] == nil)) {
			return errAt("snapshot binding")
		}
		m := obj(x["metrics"])
		pairs := []any{m["memory"], m["swap"]}
		if m["filesystems"] != nil {
			pairs = append(pairs, arr(m["filesystems"])...)
		}
		for _, v := range pairs {
			if v != nil {
				p := obj(v)
				if p["usedBytes"].(float64) > p["totalBytes"].(float64) {
					return errAt("bytes")
				}
			}
		}
	}
	for _, v := range arr(f["events"]) {
		x := obj(v)
		if x["deviceId"] != nil && !d[str(x["deviceId"])] {
			return errAt("event device")
		}
	}
	return nil
}
