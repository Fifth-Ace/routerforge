package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

type foundationNativeRead func(context.Context, string, string) ([]byte, error)

// Only fixed authenticated local Unix socket endpoints; never a user supplied
// URL, external HTTP request, shell command, or module write.
func foundationReadNative(ctx context.Context, module, target string) ([]byte, error) {
	socket, ok := activeModuleSocket(module)
	if !ok {
		return nil, contract.ErrNativeList
	}
	tr := moduleTransport(socket)
	defer tr.CloseIdleConnections()
	client := http.Client{Transport: tr, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix"+target, nil)
	if e != nil {
		return nil, contract.ErrNativeList
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
	b, e := io.ReadAll(io.LimitReader(resp.Body, contract.NativePayloadLimit+1))
	if e != nil || len(b) > contract.NativePayloadLimit {
		return nil, contract.ErrNativeList
	}
	return b, nil
}
func foundationNativeResolver(ctx context.Context, read foundationNativeRead) contract.NativeListResolver {
	var dns []byte
	var dnsErr error
	dnsRead := false
	return func(ref policy.NativeListReference) (contract.NativeList, error) {
		switch ref.Module {
		case "dns":
			if !dnsRead {
				dnsRead = true
				dns, dnsErr = read(ctx, "dns-router", "/v1/config")
			}
			if dnsErr != nil {
				return contract.NativeList{}, contract.ErrNativeList
			}
			return contract.DNSNativeList(dns, ref.Name)
		case "nfqws":
			b, e := read(ctx, "nfqws-manager", "/v1/list?"+url.Values{"name": []string{ref.Name}}.Encode())
			if e != nil {
				return contract.NativeList{}, contract.ErrNativeList
			}
			return contract.NFQWSNativeList(b, ref.Name)
		}
		return contract.NativeList{}, contract.ErrNativeList
	}
}

// A bounded user-initiated check. It returns a candidate pin, never edits the
// catalogue. Stale pins are deliberately not updated by the server.
func registerFoundationNative(mux *http.ServeMux, s *contract.Store, read foundationNativeRead) {
	mux.HandleFunc(foundationPath+"/native-reference", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowedJSON(w, http.MethodGet)
			return
		}
		q := r.URL.Query()
		if len(q) != 1 || len(q["ref"]) != 1 || len(q.Get("ref")) > 253 {
			writeAuthJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
		ref, ok := policy.ParseNativeListReference(q.Get("ref"))
		if !ok {
			writeAuthJSON(w, 400, map[string]string{"error": "INVALID_REFERENCE"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		list, e := foundationNativeResolver(ctx, read)(ref)
		if e != nil {
			writeAuthJSON(w, 409, map[string]string{"error": "NATIVE_LIST_UNAVAILABLE_OR_UNSUPPORTED"})
			return
		}
		status := "unpinned"
		if ref.SHA256 != "" {
			status = "current"
			if ref.SHA256 != list.SHA256 {
				status = "stale"
			}
		}
		writeAuthJSON(w, 200, map[string]any{"schemaVersion": 1, "ref": ref.Key(), "pin": ref.Pinned(list.SHA256), "sha256": list.SHA256, "count": len(list.Values), "status": status, "scope": "domain-membership-only"})
	})
	mux.HandleFunc(foundationPath+"/native-references", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowedJSON(w, http.MethodGet)
			return
		}
		if r.URL.RawQuery != "" {
			writeAuthJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
		st, e := s.Get()
		if e != nil {
			foundationError(w, e)
			return
		}
		refs, e := contract.NativeReferenceIndex(st.Current)
		if e != nil {
			foundationError(w, e)
			return
		}
		writeAuthJSON(w, 200, map[string]any{"schemaVersion": 1, "etag": st.Current.ETag, "references": refs})
	})
}
