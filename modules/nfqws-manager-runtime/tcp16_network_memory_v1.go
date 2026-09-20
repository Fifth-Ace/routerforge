package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	v2TCP16MemoryVersion = 1
	v2TCP16MemoryMax     = 256
)

var (
	v2TCP16MemoryPath = v2TargetMemoryRoot + "/tcp16-network-memory.json"
	v2TCP16MemoryNow  = time.Now
)

type v2TCP16NetworkEntry struct {
	NetworkKey        string `json:"network_key"`
	CIDR              string `json:"cidr"`
	ASN               int    `json:"asn,omitempty"`
	Provider          string `json:"provider,omitempty"`
	DestinationIPv4   string `json:"destination_ipv4"`
	LastTarget        string `json:"last_target"`
	CutoffSuspected   bool   `json:"cutoff_16k_suspected"`
	SuspectedCount    int    `json:"suspected_count"`
	ClearCount        int    `json:"clear_count"`
	WorkingSNI        string `json:"working_sni,omitempty"`
	WorkingSNISource  string `json:"working_sni_source,omitempty"`
	LastObserved      string `json:"last_observed"`
	LastWorkingSNI    string `json:"last_working_sni,omitempty"`
	Confidence        string `json:"confidence"`
	ObservationSource string `json:"observation_source"`
}

type v2TCP16MemoryDocument struct {
	Version int                   `json:"version"`
	Entries []v2TCP16NetworkEntry `json:"entries"`
}

type v2TCP16MemoryResponse struct {
	OK         bool                     `json:"ok"`
	Version    int                      `json:"version"`
	ReadOnly   bool                     `json:"read_only"`
	Count      int                      `json:"count"`
	Suspected  int                      `json:"suspected"`
	Clear      int                      `json:"clear"`
	WorkingSNI int                      `json:"working_sni"`
	Entries    []v2TCP16NetworkEntry    `json:"entries"`
	References []v2TCP16MemoryReference `json:"references"`
}

type v2TCP16MemoryReference struct {
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Note       string `json:"note"`
}

func registerTCP16NetworkMemoryV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/tcp16-memory", getOnly(handleV2TCP16NetworkMemory))
}

func v2TCP16NetworkIdentity(ipText string) (string, string, error) {
	ip := net.ParseIP(strings.TrimSpace(ipText))
	if ip == nil {
		return "", "", errors.New("invalid IPv4 address")
	}
	v4 := ip.To4()
	if v4 == nil {
		return "", "", errors.New("IPv4 address required")
	}
	network := net.IPv4(v4[0], v4[1], v4[2], 0)
	cidr := network.String() + "/24"
	return "cidr:" + cidr, cidr, nil
}

func v2TCP16Confidence(entry v2TCP16NetworkEntry) string {
	if strings.TrimSpace(entry.WorkingSNI) != "" {
		if entry.SuspectedCount >= 2 {
			return "VERIFIED_SNI"
		}
		return "SNI_KNOWN"
	}
	if entry.SuspectedCount >= 3 {
		return "STRONG"
	}
	if entry.SuspectedCount >= 1 {
		return "OBSERVED"
	}
	if entry.ClearCount >= 2 {
		return "CLEAR"
	}
	return "NEW"
}

func readV2TCP16Memory() (v2TCP16MemoryDocument, error) {
	doc := v2TCP16MemoryDocument{Version: v2TCP16MemoryVersion, Entries: []v2TCP16NetworkEntry{}}
	data, err := os.ReadFile(v2TCP16MemoryPath)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}
	if len(data) > 1<<20 {
		return doc, errors.New("tcp16 network memory exceeds safety limit")
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, errors.New("decode tcp16 network memory: " + err.Error())
	}
	if doc.Version != v2TCP16MemoryVersion {
		return doc, errors.New("unsupported tcp16 network memory version")
	}
	if len(doc.Entries) > v2TCP16MemoryMax {
		return doc, errors.New("tcp16 network memory exceeds entry limit")
	}
	return doc, nil
}

func writeV2TCP16Memory(doc v2TCP16MemoryDocument) error {
	if err := ensureV2TargetMemoryRoot(); err != nil {
		return err
	}
	if len(doc.Entries) > v2TCP16MemoryMax {
		return errors.New("tcp16 network memory entry limit exceeded")
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return safety.WriteFileAtomic(v2TCP16MemoryPath, data, 0600)
}

func v2RecordTCP16Observation(target, destinationIPv4 string, metrics v2HTTPMetrics) error {
	key, cidr, err := v2TCP16NetworkIdentity(destinationIPv4)
	if err != nil {
		return err
	}
	doc, err := readV2TCP16Memory()
	if err != nil {
		return err
	}
	now := v2TCP16MemoryNow().UTC().Format(time.RFC3339)
	index := -1
	for i := range doc.Entries {
		if doc.Entries[i].NetworkKey == key {
			index = i
			break
		}
	}
	entry := v2TCP16NetworkEntry{
		NetworkKey: key, CIDR: cidr, DestinationIPv4: destinationIPv4,
		ObservationSource: "routerforge-detect",
	}
	if index >= 0 {
		entry = doc.Entries[index]
	}
	entry.NetworkKey = key
	entry.CIDR = cidr
	entry.DestinationIPv4 = destinationIPv4
	entry.LastTarget = strings.ToLower(strings.TrimSpace(target))
	entry.LastObserved = now
	entry.ObservationSource = "routerforge-detect"
	entry.CutoffSuspected = metrics.Cutoff16KSuspected
	if metrics.Cutoff16KSuspected {
		entry.SuspectedCount++
	} else if metrics.ResponseComplete || metrics.ProgressProven {
		entry.ClearCount++
	}
	entry.Confidence = v2TCP16Confidence(entry)

	if index >= 0 {
		doc.Entries[index] = entry
	} else {
		doc.Entries = append(doc.Entries, entry)
	}
	sort.SliceStable(doc.Entries, func(i, j int) bool {
		return doc.Entries[i].LastObserved > doc.Entries[j].LastObserved
	})
	if len(doc.Entries) > v2TCP16MemoryMax {
		doc.Entries = doc.Entries[:v2TCP16MemoryMax]
	}
	return writeV2TCP16Memory(doc)
}

func handleV2TCP16NetworkMemory(w http.ResponseWriter, _ *http.Request) {
	doc, err := readV2TCP16Memory()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	suspected, clear, working := 0, 0, 0
	for i := range doc.Entries {
		doc.Entries[i].Confidence = v2TCP16Confidence(doc.Entries[i])
		if doc.Entries[i].CutoffSuspected {
			suspected++
		} else {
			clear++
		}
		if strings.TrimSpace(doc.Entries[i].WorkingSNI) != "" {
			working++
		}
	}
	writeJSON(w, http.StatusOK, v2TCP16MemoryResponse{
		OK: true, Version: doc.Version, ReadOnly: true,
		Count: len(doc.Entries), Suspected: suspected, Clear: clear, WorkingSNI: working,
		Entries: doc.Entries,
		References: []v2TCP16MemoryReference{{
			Repository: "necronicle/z2k",
			Ref:        "fca1ed5a452f2554b3dfa1ab18571cee7c505174",
			Note:       "network-to-SNI memory concept reviewed; RouterForge storage and safety contract are native",
		}},
	})
}
