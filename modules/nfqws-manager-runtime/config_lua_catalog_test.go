package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNF1B3LuaCatalogParsesInstalledFunctionDocumentation(t *testing.T) {
	source := `--[[
function bogus(ctx, desync)
]]
-- nfqws1 : "--dpi-desync=fake"
-- standard args : direction, payload, fooling
-- arg : blob=<blob> - fake payload
-- arg : tls_mod=<list> - TLS transformations
function fake(ctx, desync)
  if not desync then return end
end
-- arg : pos=1 - split position
function multisplit(ctx, desync)
end
function helper(ctx, something_else)
end
`
	result := nf1bLuaExtract(source, "zapret-antidpi.lua")
	if len(result) != 2 {
		t.Fatalf("function count=%d want=2: %+v", len(result), result)
	}
	if result[0].Name != "fake" || result[0].File != "zapret-antidpi.lua" || len(result[0].Arguments) != 2 {
		t.Fatalf("fake entry: %+v", result[0])
	}
	if result[0].Legacy == "" || !strings.Contains(result[0].Standard, "payload") {
		t.Fatalf("function documentation missing: %+v", result[0])
	}
	if result[1].Name != "multisplit" || result[1].Arguments[0].Name != "pos" {
		t.Fatalf("multisplit: %+v", result[1])
	}
}

func TestNF1B3LuaCatalogReadOnlyEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/config/lua-catalog", nil)
	resp := httptest.NewRecorder()
	handleNF1BLuaCatalog(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", resp.Code, resp.Body.String())
	}
	var payload nf1bLuaCatalog
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.ReadOnly || payload.Source != "installed_lua" || payload.Functions == nil {
		t.Fatalf("catalog: %+v", payload)
	}
}
