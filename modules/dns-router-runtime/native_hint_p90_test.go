package dnsrouter

import (
 "strings"
 "testing"
)
func TestP90NativeOpaqueDoH(t *testing.T){
 raw:="https://dns.dns-ai.ru/dns-query@dnsm"
 selector:=nativeSelectorHintP90(raw)
 if !strings.HasPrefix(selector,"sha256-") || len(selector)!=71{t.Fatalf("bad selector %q",selector)}
 if !nativeHintMatchesP90(selector,raw) || nativeHintMatchesP90(selector,raw+"x"){t.Fatal("opaque identity mismatch")}
 if nativeSelectorHintP90("9.9.9.9")!="9.9.9.9"{t.Fatal("legacy selector changed")}
 for _,bad:=range []string{"", "http://host/dns-query", "https://evil.example/x#frag", "https://evil.example/x?q=abc", "https://user:pw@evil.example/x", strings.Repeat("x",121),"https://evil.example/no path\n"} {
  if got:=nativeSelectorHintP90(bad);got!=""{t.Errorf("unsafe hint accepted: %q",bad)}
 }
}
func TestP90NativeHintDuplicateAndPin(t *testing.T){
 raw:="https://dns.dns-ai.ru/dns-query@dnsm"
 ep:=NativeEndpointP82{Address:"127.0.0.1:40506",Domain:".",UpstreamHint:raw,SelectorHint:nativeSelectorHintP90(raw)}
 inv:=NativeDNSInventoryP82{Available:true,ParsedInstances:1,Instances:[]NativeInstanceP82{{Name:"System",Valid:true,Endpoints:[]NativeEndpointP82{ep}}}}
 selector:="native://System/./"+ep.SelectorHint
 if _,err:=ResolveNativeDestinationP83(Upstream{ID:"p90",Transport:"native",Address:selector},"youtube.com",inv,"");err!=nil{t.Fatal(err)}
 if _,err:=ResolveNativeDestinationP83(Upstream{ID:"p90",Transport:"native",Address:selector},"youtube.com",NativeDNSInventoryP82{Available:false,Instances:inv.Instances},"");err==nil{t.Fatal("incomplete accepted")}
 inv.Instances[0].Endpoints=append(inv.Instances[0].Endpoints,NativeEndpointP82{Address:"127.0.0.1:40507",Domain:".",UpstreamHint:raw,SelectorHint:ep.SelectorHint})
 if _,err:=ResolveNativeDestinationP83(Upstream{ID:"p90",Transport:"native",Address:selector},"youtube.com",inv,"");err==nil{t.Fatal("duplicate selected")}
}
