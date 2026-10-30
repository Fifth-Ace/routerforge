package dnsrouter

// DPR2-P8.3: manual-capability bridge to Keenetic-owned local stubby endpoints.
// No DNS Proxy configuration, port 53, routing table or firewall mutation.
// The native:// address is ONLY a selector; it is never used as a dial target.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var nativeInstanceP83 = regexp.MustCompile(`^Policy(0|[1-9][0-9]{0,2})$`)
var nativeHintP83 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$`)

type nativeSelectorP83 struct {
	Instance string
	Domain   string
	Hint     string
}

func parseNativeSelectorP83(u Upstream) (nativeSelectorP83, error) {
	zero := nativeSelectorP83{}
	if u.Transport != "native" || u.ServerName != "" || u.Path != "" || len(u.Fallbacks) != 0 {
		return zero, fmt.Errorf("native upstream %q must have native transport, no TLS fields or legacy fallbacks", u.ID)
	}
	if !strings.HasPrefix(u.Address, "native://") {
		return zero, errors.New("native endpoint selector must start with native://")
	}
	fields := strings.Split(strings.TrimPrefix(u.Address, "native://"), "/")
	if len(fields) != 3 {
		return zero, errors.New("native selector must be native://System-or-PolicyN/zone/upstream-hint")
	}
	selector := nativeSelectorP83{Instance: fields[0], Domain: fields[1], Hint: fields[2]}
	if selector.Instance != "System" && !nativeInstanceP83.MatchString(selector.Instance) {
		return zero, errors.New("native selector requires System or PolicyN")
	}
	if selector.Domain != "." {
		d, err := normalizedDomain(selector.Domain)
		if err != nil || d != selector.Domain || isProtected(d) {
			return zero, errors.New("invalid or protected native DNS zone")
		}
	}
	if !nativeHintP83.MatchString(selector.Hint) {
		return zero, errors.New("invalid native DNS upstream hint")
	}
	return selector, nil
}

// ResolveNativeDestinationP83 verifies the selector against a fresh *complete*
// inventory and returns one bounded loopback endpoint. Never trust a persisted
// native port, an inferred default, or a second ambiguous matching endpoint.
func ResolveNativeDestinationP83(u Upstream, qname string, inventory NativeDNSInventoryP82, listener string) (string, error) {
	selector, err := parseNativeSelectorP83(u)
	if err != nil {
		return "", err
	}
	if !inventory.Available || inventory.ParsedInstances != len(inventory.Instances) {
		return "", errors.New("native DNS inventory unavailable or incomplete")
	}
	normalized, err := normalizedDomain(qname)
	if err != nil || normalized != qname || isProtected(qname) {
		return "", errors.New("invalid or protected DNS question")
	}
	if selector.Domain != "." && qname != selector.Domain && !strings.HasSuffix(qname, "."+selector.Domain) {
		return "", errors.New("DNS question is outside selected native zone")
	}
	listenPort := ""
	if listener != "" {
		_, listenPort, _ = net.SplitHostPort(listener)
	}
	found := ""
	count := 0
	for _, instance := range inventory.Instances {
		if instance.Name != selector.Instance {
			continue
		}
		if !instance.Valid {
			return "", errors.New("selected native policy inventory invalid")
		}
		for _, ep := range instance.Endpoints {
			if ep.Domain != selector.Domain || ep.UpstreamHint != selector.Hint {
				continue
			}
			host, port, e := net.SplitHostPort(ep.Address)
			n, pe := strconv.Atoi(port)
			if e != nil || pe != nil || host != "127.0.0.1" || n < 1025 || n > 65535 || port == listenPort {
				return "", errors.New("unsafe native DNS destination")
			}
			found = ep.Address
			count++
		}
	}
	if count != 1 {
		return "", fmt.Errorf("native selector must resolve exactly once, got %d", count)
	}
	return found, nil
}

type NativeBindingPreviewP83 struct {
	ID       string `json:"id"`
	Selector string `json:"selector"`
	Address  string `json:"address,omitempty"`
	Ready    bool   `json:"ready"`
	Reason   string `json:"reason,omitempty"`
}

type NativeBridgePreviewP83 struct {
	Mode              string                    `json:"mode"`
	ActivationAllowed bool                      `json:"activation_allowed"`
	ChangesApplied    bool                      `json:"changes_applied"`
	EncryptionProven  bool                      `json:"encryption_proven"`
	Bindings          []NativeBindingPreviewP83 `json:"bindings"`
}

func BuildNativeBridgePreviewP83(cfg Config, inventory NativeDNSInventoryP82) NativeBridgePreviewP83 {
	out := NativeBridgePreviewP83{Mode: "read-only-native-bridge-preview", Bindings: []NativeBindingPreviewP83{}}
	listener := ""
	if cfg.Listener != nil {
		listener = cfg.Listener.Listen
	}
	for _, u := range cfg.Upstreams {
		if u.Transport != "native" {
			continue
		}
		item := NativeBindingPreviewP83{ID: u.ID, Selector: u.Address}
		selector, err := parseNativeSelectorP83(u)
		if err != nil {
			item.Reason = err.Error()
		} else {
			probe := "example.org"
			if selector.Domain != "." {
				probe = "p83-probe." + selector.Domain
			}
			address, e := ResolveNativeDestinationP83(u, probe, inventory, listener)
			if e != nil {
				item.Reason = e.Error()
			} else {
				item.Address = address
				item.Ready = true
			}
		}
		out.Bindings = append(out.Bindings, item)
	}
	return out // never grants capability or proves encryption
}

func (f *ShadowForwarder) exchangeNativeP83(ctx context.Context, u Upstream, query []byte) ([]byte, error) {
	if !f.opts.NativeBridge {
		return nil, errors.New("native bridge requires explicit runtime capability")
	}
	q, err := parseDNSQuestion(query)
	if err != nil {
		return nil, err
	}
	dir := f.opts.NativeDNSDir
	if dir == "" {
		dir = "/var"
	}
	address, err := ResolveNativeDestinationP83(u, q.name, ReadOnlyNativeDNSInventoryP82(dir), f.opts.Listen)
	if err != nil {
		return nil, err
	}
	conn, err := f.opts.Dial(ctx, "udp", address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(f.opts.Timeout)); err != nil {
		return nil, err
	}
	return exchangeDNSConn(conn, "udp", query)
}
