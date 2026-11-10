'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {pathToFileURL} = require('node:url');
const root = path.resolve(__dirname, '..', 'modules/dns-router-runtime');
const frontend = path.join(root, 'frontend');
const read = name => fs.readFileSync(path.join(frontend, name), 'utf8');
const load = name => import(pathToFileURL(path.join(frontend, name)).href);

test('P90 real native catalog: @dnsm has an opaque inventory-bound DoH selector', async () => {
  const {entries} = await load('DnsRuntimeNativeCatalog.js');
  const inventory = {available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[
    {address:'127.0.0.1:40506',domain:'.',upstream_hint:'https://dns.dns-ai.ru/dns-query@dnsm',transport_hint:'doh',selector_hint:'sha256-'+'a'.repeat(64)}
  ]}]};
  const [entry] = entries(inventory, 'System');
  assert.equal(entry.usable, true);
  assert.equal(entry.transport, 'doh');
  assert.equal(entry.selector, 'native://System/./sha256-'+'a'.repeat(64));
  assert.match(entry.label, /https:\/\/dns\.dns-ai\.ru\/dns-query ·/);
  assert.doesNotMatch(entry.label, /@dnsm/);
  assert.equal(entry.encryptionProven, false);
});

test('P90 native inventory fails closed for untrusted zones and incomplete evidence', async () => {
  const {entries, acceptsDomains} = await load('DnsRuntimeNativeCatalog.js');
  const inv = {available:true,parsed_instances:1,instances:[{name:'System',valid:true,endpoints:[
    {address:'127.0.0.1:40500',domain:'.',upstream_hint:'1.1.1.1'},
    {address:'127.0.0.1:40501',domain:'ru',upstream_hint:'common.dot.dns.yandex.net'}
  ]}]};
  const [rootDNS,zoned] = entries(inv, 'System');
  assert.equal(rootDNS.usable, true);
  assert.equal(rootDNS.selector, 'native://System/./1.1.1.1');
  assert.equal(acceptsDomains(zoned, ['youtube.com']).ok, false);
  inv.available = false;
  assert.equal(entries(inv, 'System')[0].usable, false);
});

test('P90 Svelte owns the certificate UI; public PEM is staged only after verified TLS', async () => {
  const component = read('DnsRuntimeP91.svelte');
  const logic = read('DnsRuntimeP91Logic.js');
  const api = fs.readFileSync(path.join(root, 'api.go'), 'utf8');
  const cert = fs.readFileSync(path.join(root, 'certificates_p90.go'), 'utf8');
  const policy = fs.readFileSync(path.join(root, 'policy.go'), 'utf8');
  for (const id of ['drP90Transport','drP90Endpoint','drP90IP','drP90Inspect','drP90Store','drP90Status','drP91PEM']) {
    assert.equal((component.match(new RegExp('id="'+id+'"', 'g')) || []).length, 1, id);
  }
  assert.match(component, /createP91Flow/);
  assert.match(component, /disabled=\{busy\|\|!verified\}/);
  assert.match(logic, /result\.certificate\?\.verified/);
  assert.match(logic, /server\.certificate_pem=result\.certificate\.pem/);
  assert.match(api, /\/v1\/certificates\/inspect/);
  assert.match(api, /requireMutation\(w, r, "certificate-inspect"\)/);
  assert.match(policy, /CertificatePEM string/);
  assert.match(policy, /validateSavedCertificateP90/);
  assert.match(cert, /tls\.Client\(conn,/);
  assert.doesNotMatch(cert, /InsecureSkipVerify/);
  assert.doesNotMatch(logic, /\.innerHTML\s*=|fetch\s*\(/);
});

test('P90 native selector must be bound to fresh inventory, not a raw DoH socket URL', () => {
  const inventory = fs.readFileSync(path.join(root, 'native_inventory_p82.go'), 'utf8');
  const bridge = fs.readFileSync(path.join(root, 'native_bridge_p83.go'), 'utf8');
  assert.match(inventory, /SelectorHint = nativeSelectorHintP90\(hint\)/);
  assert.match(inventory, /TransportHint = "doh"/);
  assert.match(bridge, /nativeHintMatchesP90\(selector\.Hint, ep\.UpstreamHint\)/);
});
