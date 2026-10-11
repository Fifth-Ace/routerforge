package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ProtectedService is an explicitly scoped destination, not a firewall rule.
// The wizard does not apply firewall changes or run an Entware daemon.
type ProtectedService struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type EngineWizardRequest struct {
	Engine          string             `json:"engine"`
	Interface       string             `json:"interface"`
	Knock           []int              `json:"knock,omitempty"`
	SequenceSeconds int                `json:"sequence_seconds,omitempty"`
	AccessSeconds   int                `json:"access_seconds"`
	Services        []ProtectedService `json:"services"`
}

func validateWizard(req EngineWizardRequest) error {
	if req.Engine != "knockd" && req.Engine != "fwknopd" {
		return errors.New("unsupported engine")
	}
	if len(req.Services) == 0 || len(req.Services) > 16 {
		return errors.New("service count must be 1..16")
	}
	if req.AccessSeconds < 30 || req.AccessSeconds > 86400 {
		return errors.New("invalid access lifetime")
	}
	if req.Engine == "knockd" {
		if len(req.Knock) != 3 || req.SequenceSeconds < 5 || req.SequenceSeconds > 300 {
			return errors.New("invalid knock sequence or window")
		}
		if len(req.Interface) < 1 || len(req.Interface) > 24 {
			return errors.New("invalid interface")
		}
		for _, c := range req.Interface {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
				return errors.New("invalid interface")
			}
		}
		seen := map[int]bool{}
		for _, p := range req.Knock {
			if p < 1 || p > 65535 || seen[p] {
				return errors.New("knock ports must be distinct")
			}
			seen[p] = true
		}
	} else if len(req.Knock) > 0 {
		return errors.New("SPA cannot have knock ports")
	}
	if req.Engine == "fwknopd" {
		if len(req.Interface) < 1 || len(req.Interface) > 24 {
			return errors.New("invalid SPA interface")
		}
		for _, c := range req.Interface {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
				return errors.New("invalid SPA interface")
			}
		}
	}
	seenSvc := map[string]bool{}
	for _, s := range req.Services {
		if len(s.Name) < 1 || len(s.Name) > 40 {
			return errors.New("invalid service label")
		}
		for _, c := range s.Name {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
				return errors.New("invalid service label")
			}
		}
		addr := net.ParseIP(s.Address)
		if addr == nil || addr.To4() == nil || addr.IsUnspecified() || addr.IsMulticast() || addr.IsLoopback() {
			return errors.New("invalid destination IPv4")
		}
		if s.Port < 1 || s.Port > 65535 {
			return errors.New("invalid service port")
		}
		if req.Engine == "knockd" {
			for _, k := range req.Knock {
				if k == s.Port {
					return errors.New("service overlaps knock port")
				}
			}
		}
		key := addr.String() + ":" + strconv.Itoa(s.Port)
		if seenSvc[key] {
			return errors.New("duplicate destination")
		}
		seenSvc[key] = true
	}
	return nil
}

// GenerateSPAKeys is intentionally separate from any status or GET response.
// Its output must only be transported by explicit, authorized secret-bearing actions.
func generateSPAKeys() (string, string, error) {
	key := make([]byte, 32)
	mac := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", "", fmt.Errorf("encryption key: %w", err)
	}
	if _, err := rand.Read(mac); err != nil {
		return "", "", fmt.Errorf("authentication key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), base64.StdEncoding.EncodeToString(mac), nil
}

func describeWizard(req EngineWizardRequest) (string, error) {
	if err := validateWizard(req); err != nil {
		return "", err
	}
	lines := []string{fmt.Sprintf("engine=%s; services=%d; lifetime=%ds", req.Engine, len(req.Services), req.AccessSeconds)}
	if req.Engine == "knockd" {
		lines = append(lines, fmt.Sprintf("interface=%s; sequence=%d,%d,%d; window=%ds", req.Interface, req.Knock[0], req.Knock[1], req.Knock[2], req.SequenceSeconds))
	}
	for _, s := range req.Services {
		lines = append(lines, s.Name+": "+s.Address+":"+strconv.Itoa(s.Port))
	}
	return strings.Join(lines, "\n"), nil
}
