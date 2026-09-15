#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]

SOURCE = ROOT / "components/core/user_app_sources.go"
WEB = ROOT / "components/core/web_probe.go"
UI = ROOT / "components/core/frontend/src/lib/components/SourceManager.svelte"
TEST_ISOLATION = ROOT / "components/core/user_app_sources_isolation_test.go"
TEST_EXPLICIT = ROOT / "components/core/user_app_sources_explicit_action_test.go"
TEST_BOUNDARY = ROOT / "components/core/user_app_sources_boundary_test.go"

def fail(message: str) -> None:
    raise SystemExit("private-registry-contract: " + message)

def read(path: Path) -> str:
    if not path.is_file():
        fail(f"missing required file: {path.relative_to(ROOT)}")
    return path.read_text(encoding="utf-8")

def require(text: str, marker: str, label: str) -> None:
    if marker not in text:
        fail(f"{label}: missing marker {marker!r}")

def function_block(text: str, name: str) -> str:
    marker = f"func {name}("
    start = text.find(marker)
    if start < 0:
        fail(f"missing function {name}")
    next_func = text.find("\nfunc ", start + len(marker))
    if next_func < 0:
        return text[start:]
    return text[start:next_func]

source = read(SOURCE)
web = read(WEB)
ui = read(UI)
test_isolation = read(TEST_ISOLATION)
test_explicit = read(TEST_EXPLICIT)
test_boundary = read(TEST_BOUNDARY)

# P13C.1 — namespace isolation and conservative trust.
for marker in (
    'func validAppSourceRecordID(id string) bool {',
    'len(id) != 16',
    'strings.HasPrefix(id, "src-")',
    'id != strings.ToLower(id)',
    'item.ID = source.ID + ":" + manifestID',
    'item.ManifestID = manifestID',
    'item.RegistrySource = source.ID',
    'item.Source = "user-source"',
    'Status: "unverified"',
    'Local/private user-added source.',
    'if !validAppSourceRecordID(source.ID) {',
):
    require(source, marker, "P13C.1")

for marker in (
    'TestUserSourceCannotOverridePublicManifestID',
    'TestLocalPrivateSourceGetsExplicitLowerTrustNote',
    'TestMalformedConfiguredSourceCannotEnterCatalog',
):
    require(test_isolation, marker, "P13C.1 tests")

# P13C.2 — preview and add are separate explicit actions bound to SHA-256.
for marker in (
    'appSourceAddConfirm             = "ADD_SOURCE"',
    'Confirm     string `json:"confirm,omitempty"`',
    'Fingerprint string `json:"fingerprint,omitempty"`',
    'func validAppSourceFingerprint(value string) bool {',
    'explicit source add confirmation is required',
    'valid preview fingerprint is required',
    'source changed since preview; preview it again before adding',
):
    require(source, marker, "P13C.2")

add_block = function_block(source, "addAppSource")
confirm_pos = add_block.find("request.Confirm")
fingerprint_pos = add_block.find("request.Fingerprint")
resolve_pos = add_block.find("appSourceResolve(")
persist_pos = min(
    [p for p in (add_block.find("saveAppSourcesConfigUnlocked("),
                 add_block.find("saveAppSourceCache(")) if p >= 0],
    default=-1,
)
if min(confirm_pos, fingerprint_pos, resolve_pos) < 0:
    fail("P13C.2: add gate structure incomplete")
if not (confirm_pos < resolve_pos and fingerprint_pos < resolve_pos):
    fail("P13C.2: explicit confirmation/fingerprint validation must happen before source resolution")
if persist_pos >= 0 and resolve_pos > persist_pos:
    fail("P13C.2: persistence appears before preview-bound re-resolution")

preview_block = function_block(source, "handleAppSourcePreview")
for forbidden in (
    "addAppSource(",
    "saveAppSourcesConfigUnlocked(",
    "saveAppSourceCache(",
    "os.WriteFile(",
    "os.Rename(",
):
    if forbidden in preview_block:
        fail(f"P13C.2: preview contains persistence/mutation primitive {forbidden!r}")
require(preview_block, "http.MethodPost", "P13C.2 preview")
require(preview_block, "sameOriginRequest(r)", "P13C.2 preview")

for marker in (
    "let previewRequest = null;",
    "invalidateSourcePreview()",
    "confirm:'ADD_SOURCE'",
    "fingerprint:preview.fingerprint",
    "onchange={invalidateSourcePreview}",
    "oninput={invalidateSourcePreview}",
):
    require(ui, marker, "P13C.2 UI")

for marker in (
    "TestAppSourceAddRequiresExplicitPreviewFingerprint",
    "TestAppSourcePreviewDoesNotPersistOrEnableSource",
    "TestAppSourceFingerprintValidationIsCanonical",
):
    require(test_explicit, marker, "P13C.2 tests")

# P13C.3 — detection is passive; user/private metadata cannot trigger active web probe.
web_allowed = function_block(web, "catalogWebProbeAllowed")
require(web_allowed, 'strings.HasPrefix(item.RegistrySource, "src-")', "P13C.3 web")
require(web_allowed, "return false", "P13C.3 web")
handle_probe = function_block(web, "handleCatalogWebProbe")
require(handle_probe, "http.MethodPost", "P13C.3 web handler")
require(handle_probe, "sameOriginRequest(r)", "P13C.3 web handler")

for marker in (
    "TestUserSourceWebProbeIsAlwaysExplicitlyBlocked",
    "TestUserSourceDetectionIsReadOnlyAgainstSourceState",
    "TestUserSourceWebMetadataDoesNotGrantLifecycleAuthority",
):
    require(test_boundary, marker, "P13C.3 tests")

# Public IDs must never be silently replaced by a user source.
if re.search(r"\bitem\.ID\s*=\s*manifestID\b", source):
    fail("namespace isolation regression: user source assigns raw manifest ID")
if 'item.ID = source.ID + ":" + manifestID' not in source:
    fail("namespace isolation regression: source namespace assignment missing")

# Local/private source enablement stays opt-in and separate from unsafe install permission.
for marker in (
    "AllowUnverified",
    "AllowLocalSources",
    "appSourceLocalSourcesAllowed()",
    "private source address requires explicit local-source permission",
):
    require(source, marker, "local/private opt-in")

print("PRIVATE_REGISTRY_COMPLETION_GATE=PASS")
print("P13C1_SOURCE_ISOLATION=PASS")
print("P13C2_EXPLICIT_SOURCE_ACTIONS=PASS")
print("P13C3_DETECTION_WEB_BOUNDARIES=PASS")
print("PUBLIC_ID_SILENT_OVERRIDE=BLOCKED")
print("LOCAL_PRIVATE_TRUST=CONSERVATIVE")
print("PREVIEW_PERSISTENCE=NONE")
print("ADD_CONFIRM_AND_SHA256=REQUIRED")
print("USER_SOURCE_DETECTION=PASSIVE_READ_ONLY")
print("USER_SOURCE_WEB_PROBE=BLOCKED")
print("P13C=COMPLETE")
