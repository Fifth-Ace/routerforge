package main

import (
	"encoding/binary"
	"strings"
	"testing"
)

func v2GeoPBField(num, wire int, payload []byte) []byte {
	key := uint64(num<<3 | wire)
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, key)
	out := append([]byte{}, buf[:n]...)
	if wire == 2 {
		n = binary.PutUvarint(buf, uint64(len(payload)))
		out = append(out, buf[:n]...)
	}
	out = append(out, payload...)
	return out
}

func v2GeoPBVarintField(num int, value uint64) []byte {
	key := uint64(num << 3)
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, key)
	out := append([]byte{}, buf[:n]...)
	n = binary.PutUvarint(buf, value)
	return append(out, buf[:n]...)
}

func TestGeoSiteParserCategoriesAndRegexSkip(t *testing.T) {
	domain := append(v2GeoPBVarintField(1, 2), v2GeoPBField(2, 2, []byte("example.com"))...)
	regex := append(v2GeoPBVarintField(1, 1), v2GeoPBField(2, 2, []byte(".*bad.*"))...)
	entry := append(v2GeoPBField(1, 2, []byte("TEST")), v2GeoPBField(2, 2, domain)...)
	entry = append(entry, v2GeoPBField(2, 2, regex)...)
	data := v2GeoPBField(1, 2, entry)
	got := v2GeoParseSite(data)
	if len(got["test"]) != 1 || got["test"][0] != "example.com" {
		t.Fatalf("parsed=%+v", got)
	}
}

func TestGeoIPParser(t *testing.T) {
	cidr := append(v2GeoPBField(1, 2, []byte{1, 2, 3, 0}), v2GeoPBVarintField(2, 24)...)
	entry := append(v2GeoPBField(1, 2, []byte("RU")), v2GeoPBField(2, 2, cidr)...)
	data := v2GeoPBField(1, 2, entry)
	got := v2GeoParseIP(data)
	if len(got["ru"]) != 1 || got["ru"][0] != "1.2.3.0/24" {
		t.Fatalf("parsed=%+v", got)
	}
}

func TestGeoTextDedupe(t *testing.T) {
	got := v2GeoParseText([]byte("# x\nExample.com\nexample.com\n1.2.3.0/24\n"))
	items := v2GeoDedupe(got["all"])
	if len(items) != 2 {
		t.Fatalf("items=%v", items)
	}
}

func TestGeoPreviewLimit(t *testing.T) {
	items := v2GeoDedupe([]string{"b.example", "a.example", "a.example"})
	if strings.Join(items, ",") != "a.example,b.example" {
		t.Fatalf("items=%v", items)
	}
}

func TestGeoSafeNames(t *testing.T) {
	for _, name := range []string{"geosite.dat", "geoip.dat", "custom.txt"} {
		if !v2GeoSafeName(name) {
			t.Fatalf("rejected %q", name)
		}
	}
	for _, name := range []string{"../geo.dat", "geo.bin", ".hidden.dat"} {
		if v2GeoSafeName(name) {
			t.Fatalf("accepted %q", name)
		}
	}
}
