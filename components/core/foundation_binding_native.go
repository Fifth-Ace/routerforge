package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

type bindingCall func(context.Context, string, string, string, http.Header, []byte) ([]byte, error)
type bindingTargetView struct {
	Snapshot contract.BindingSnapshot
	Revision string
	Config   map[string]json.RawMessage
	List     map[string]json.RawMessage
}
type bindingWrite struct {
	Module, Method, Path string
	Headers              http.Header
	Body                 []byte
}

// Calls are constructed exclusively by this adapter. No arbitrary URL, path,
// headers or action from the client. Never invoke service/action/reload APIs.
func foundationBindingCall(ctx context.Context, module, method, target string, headers http.Header, body []byte) ([]byte, error) {
	if module != "dns-router" && module != "nfqws-manager" {
		return nil, contract.ErrBinding
	}
	socket, ok := activeModuleSocket(module)
	if !ok {
		return nil, contract.ErrNativeList
	}
	tr := moduleTransport(socket)
	defer tr.CloseIdleConnections()
	client := http.Client{Transport: tr, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, method, "http://unix"+target, bytes.NewReader(body))
	if e != nil {
		return nil, contract.ErrNativeList
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Accept", "application/json")
	resp, e := client.Do(req)
	if e != nil {
		return nil, contract.ErrNativeList
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, contract.ErrNativeList
	}
	// Native save responses may include status/config. Never return these to UI.
	raw, e := io.ReadAll(io.LimitReader(resp.Body, contract.NativePayloadLimit+1))
	if e != nil || len(raw) > contract.NativePayloadLimit {
		return nil, contract.ErrNativeList
	}
	return raw, nil
}
func bindingObject(raw []byte) (map[string]json.RawMessage, error) {
	if _, e := contract.Decode(raw); e != nil {
		return nil, contract.ErrNativeList
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, contract.ErrNativeList
	}
	return m, nil
}
func bindingString(raw json.RawMessage) string { var s string; _ = json.Unmarshal(raw, &s); return s }
func bindingDNSView(raw []byte, ref policy.NativeListReference) (bindingTargetView, error) {
	var out bindingTargetView
	obj, e := bindingObject(raw)
	if e != nil {
		return out, e
	}
	list, e := contract.DNSNativeList(raw, ref.Name)
	if e != nil {
		return out, e
	}
	vals, e := contract.BindingDomains(list.Values)
	if e != nil {
		return out, e
	}
	cfg, e := bindingObject(obj["config"])
	if e != nil {
		return out, e
	}
	var lists []map[string]json.RawMessage
	if json.Unmarshal(cfg["lists"], &lists) != nil {
		return out, contract.ErrNativeList
	}
	for _, l := range lists {
		if bindingString(l["id"]) == ref.Name {
			out.List = l
			break
		}
	}
	if out.List == nil {
		return out, contract.ErrNativeList
	}
	kind := bindingString(out.List["kind"])
	var auto int
	if v, ok := out.List["auto_update_minutes"]; ok && json.Unmarshal(v, &auto) != nil {
		return out, contract.ErrNativeList
	}
	if (kind != "paste" && kind != "file") || auto != 0 || bindingString(out.List["url"]) != "" {
		return out, contract.ErrBinding
	}
	// The native source fingerprint covers routing semantics; add the remaining
	// selected metadata except its automatic updated_at timestamp.
	meta := map[string]json.RawMessage{}
	for k, v := range out.List {
		if k != "updated_at" {
			meta[k] = v
		}
	}
	out.Snapshot = contract.BindingSnapshot{SHA256: contract.BindingDigest(struct {
		Native string
		Meta   map[string]json.RawMessage
	}{list.SHA256, meta}), Values: vals}
	out.Revision = bindingString(obj["revision"])
	out.Config = cfg
	if out.Revision == "" || len(out.Revision) > 128 || strings.ContainsAny(out.Revision, "\r\n\"") {
		return bindingTargetView{}, contract.ErrNativeList
	}
	return out, nil
}
func bindingReadTarget(ctx context.Context, call bindingCall, ref policy.NativeListReference) (bindingTargetView, error) {
	if ref.Module == "dns" {
		raw, e := call(ctx, "dns-router", "GET", "/v1/config", nil, nil)
		if e != nil {
			return bindingTargetView{}, e
		}
		return bindingDNSView(raw, ref)
	}
	raw, e := call(ctx, "nfqws-manager", "GET", "/v1/list?"+url.Values{"name": []string{ref.Name}}.Encode(), nil, nil)
	if e != nil {
		return bindingTargetView{}, e
	}
	list, e := contract.NFQWSNativeList(raw, ref.Name)
	if e != nil {
		return bindingTargetView{}, e
	}
	vals, e := contract.BindingDomains(list.Values)
	if e != nil {
		return bindingTargetView{}, e
	}
	var p struct {
		Content   string `json:"content"`
		Protected *bool  `json:"protected"`
	}
	if json.Unmarshal(raw, &p) != nil || p.Protected == nil || *p.Protected || p.Content != contract.BindingText(vals) || list.SHA256 != contract.BindingTextDigest(vals) {
		return bindingTargetView{}, contract.ErrBinding
	}
	return bindingTargetView{Snapshot: contract.BindingSnapshot{SHA256: list.SHA256, Values: vals}, Revision: list.SHA256}, nil
}

// Compare everything except target rules and that target's updated_at. Never
// accept a native preview that silently rewrites unrelated configuration.
func bindingDNSUnrelated(view bindingTargetView, ref policy.NativeListReference) (string, error) {
	cfg := map[string]json.RawMessage{}
	for k, v := range view.Config {
		if k != "lists" && k != "rules" {
			cfg[k] = v
		}
	}
	var lists, rules []map[string]json.RawMessage
	if json.Unmarshal(view.Config["lists"], &lists) != nil || json.Unmarshal(view.Config["rules"], &rules) != nil {
		return "", contract.ErrBinding
	}
	a, b := []map[string]json.RawMessage{}, []map[string]json.RawMessage{}
	for _, l := range lists {
		if bindingString(l["id"]) != ref.Name {
			a = append(a, l)
		}
	}
	for _, r := range rules {
		if bindingString(r["list_id"]) != ref.Name {
			b = append(b, r)
		}
	}
	return contract.BindingDigest(struct {
		Config       map[string]json.RawMessage
		Lists, Rules []map[string]json.RawMessage
	}{cfg, a, b}), nil
}
func bindingPrepare(ctx context.Context, call bindingCall, ref policy.NativeListReference, before bindingTargetView, values []string) (contract.BindingSnapshot, bindingWrite, error) {
	if reflect.DeepEqual(before.Snapshot.Values, values) {
		return before.Snapshot, bindingWrite{}, nil
	}
	if ref.Module == "nfqws" {
		body, _ := json.Marshal(map[string]string{"name": ref.Name, "content": contract.BindingText(values), "expected_sha256": before.Snapshot.SHA256, "confirm": "NFQWS_LIST_SAVE"})
		return contract.BindingSnapshot{SHA256: contract.BindingTextDigest(values), Values: values}, bindingWrite{"nfqws-manager", "POST", "/v1/list/save", http.Header{"Content-Type": []string{"application/json"}, "X-Routerforge-Module-Authorized": []string{"core-authorized-v1"}}, body}, nil
	}
	body, _ := json.Marshal(map[string]any{"list": before.List, "text": contract.BindingText(values)})
	raw, e := call(ctx, "dns-router", "POST", "/v1/lists/preview", http.Header{"Content-Type": []string{"application/json"}, "X-Routerforge-Action": []string{"lists-preview"}}, body)
	if e != nil {
		return contract.BindingSnapshot{}, bindingWrite{}, e
	}
	obj, e := bindingObject(raw)
	if e != nil || bindingString(obj["base_revision"]) != before.Revision || string(obj["saved"]) != "false" || string(obj["runtime_applied"]) != "false" {
		return contract.BindingSnapshot{}, bindingWrite{}, contract.ErrConflict
	}
	p, e := bindingObject(obj["preview"])
	if e != nil || string(p["invalid"]) != "0" || string(p["duplicates"]) != "0" {
		return contract.BindingSnapshot{}, bindingWrite{}, contract.ErrBinding
	}
	wrapper, _ := json.Marshal(map[string]any{"revision": before.Revision, "config": p["config"]})
	after, e := bindingDNSView(wrapper, ref)
	if e != nil || !reflect.DeepEqual(after.Snapshot.Values, values) {
		return contract.BindingSnapshot{}, bindingWrite{}, contract.ErrBinding
	}
	a, e := bindingDNSUnrelated(before, ref)
	b, e2 := bindingDNSUnrelated(after, ref)
	if e != nil || e2 != nil || a != b {
		return contract.BindingSnapshot{}, bindingWrite{}, contract.ErrBinding
	}
	// Compare selected metadata (excluding the preview timestamp) explicitly.
	clean := func(m map[string]json.RawMessage) string {
		n := map[string]json.RawMessage{}
		for k, v := range m {
			if k != "updated_at" {
				n[k] = v
			}
		}
		return contract.BindingDigest(n)
	}
	if clean(before.List) != clean(after.List) {
		return contract.BindingSnapshot{}, bindingWrite{}, contract.ErrBinding
	}
	return after.Snapshot, bindingWrite{"dns-router", "PUT", "/v1/config", http.Header{"Content-Type": []string{"application/json"}, "X-Routerforge-Action": []string{"save"}, "If-Match": []string{"\"" + before.Revision + "\""}}, p["config"]}, nil
}

// Evidence only: this describes the DNS service revision observed at inspection,
// not packet-level delivery or verified interception.
func bindingDNSRuntime(ctx context.Context, call bindingCall, savedRevision string) string {
	raw, e := call(ctx, "dns-router", "GET", "/v1/service/status", nil, nil)
	if e != nil {
		return "NOT_VERIFIED"
	}
	obj, e := bindingObject(raw)
	if e != nil {
		return "NOT_VERIFIED"
	}
	var active bool
	if json.Unmarshal(obj["active"], &active) != nil {
		return "NOT_VERIFIED"
	}
	if !active {
		return "INACTIVE"
	}
	if bindingString(obj["applied_revision"]) == savedRevision {
		return "SAVED_REVISION_ACTIVE"
	}
	if bindingString(obj["applied_revision"]) != "" {
		return "OTHER_REVISION_ACTIVE"
	}
	return "NOT_VERIFIED"
}
