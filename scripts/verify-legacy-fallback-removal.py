#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / "components" / "core" / "catalog.go"
APP_CENTER_ENTWARE = ROOT / "components" / "core" / "app_center_entware.go"
REGISTRY_GO = ROOT / "components" / "core" / "routerforge_registry.go"
SUBMISSIONS = ROOT / "marketplace" / "submissions"
REGISTRY = ROOT / "marketplace" / "registry" / "index.json"
EMBEDDED = ROOT / "components" / "core" / "embedded" / "marketplace-index.json"
BUILDER = ROOT / "marketplace" / "build_registry.py"

RETIRED_LEGACY_IDS = {
    "adguardhome-keenetic",
    "awg-manager",
    "bypass-keenetic",
    "chur-keenetic",
    "hydraroute-neo",
    "keen-pbr",
    "keenetic-entware-extras",
    "keenetic-sing-box-ui",
    "kvas",
    "nfqws",
    "nfqws-web",
    "nfqws2",
    "skeen",
    "traffic-via-vpn",
    "xkeen",
    "xkeen-ui",
}

RETIRED_FUNCTIONS = {
    "integrationCatalog",
    "adGuardHomeIntegration",
    "awgManagerIntegration",
    "bypassKeeneticIntegration",
    "churKeeneticIntegration",
    "entwareExtrasIntegration",
    "hydraRouteIntegration",
    "keenPBRIntegration",
    "keeneticSingBoxUIIntegration",
    "kvasIntegration",
    "nfqws2Integration",
    "nfqwsIntegration",
    "nfqwsWebIntegration",
    "skeenIntegration",
    "trafficViaVPNIntegration",
    "xkeenIntegration",
    "xkeenUIIntegration",
}

def fail(message):
    raise SystemExit("legacy-fallback-removal: " + message)

def load_builder():
    spec = importlib.util.spec_from_file_location("routerforge_marketplace_builder", BUILDER)
    if spec is None or spec.loader is None:
        fail("cannot import marketplace builder")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

def main():
    catalog = CATALOG.read_text(encoding="utf-8")
    app_center_entware = APP_CENTER_ENTWARE.read_text(encoding="utf-8")
    registry_go = REGISTRY_GO.read_text(encoding="utf-8")
    builder = load_builder()

    public_doc = json.loads(REGISTRY.read_text(encoding="utf-8"))
    embedded_doc = json.loads(EMBEDDED.read_text(encoding="utf-8"))
    if public_doc != embedded_doc:
        fail("public and embedded registries differ")

    manifests = {}
    for path in sorted(SUBMISSIONS.glob("*.json")):
        obj = json.loads(path.read_text(encoding="utf-8"))
        builder.validate_manifest(obj, path)
        item_id = obj["id"]
        if item_id in manifests:
            fail("duplicate manifest ID: " + item_id)
        manifests[item_id] = (path, obj)

    entries = {}
    for entry in public_doc.get("entries", []):
        item_id = entry.get("id")
        if item_id in entries:
            fail("duplicate registry ID: " + str(item_id))
        entries[item_id] = entry

    missing_manifest = sorted(RETIRED_LEGACY_IDS - set(manifests))
    missing_registry = sorted(RETIRED_LEGACY_IDS - set(entries))
    if missing_manifest:
        fail("retired IDs without manifests: " + ", ".join(missing_manifest))
    if missing_registry:
        fail("retired IDs without registry entries: " + ", ".join(missing_registry))

    for item_id in sorted(RETIRED_LEGACY_IDS):
        path, manifest = manifests[item_id]
        entry = entries[item_id]
        expected_source = f"marketplace/submissions/{item_id}.json"
        digest = builder.digest_manifest(manifest)

        if entry.get("kind") != "integration":
            fail(f"{item_id}: registry kind is not integration")
        if entry.get("manifest_id") != item_id:
            fail(f"{item_id}: registry manifest_id mismatch")
        if entry.get("manifest_source") != expected_source:
            fail(f"{item_id}: manifest_source mismatch")
        if entry.get("manifest_sha256") != digest:
            fail(f"{item_id}: manifest digest mismatch")

        trust = (entry.get("trust") or {}).get("status", "")
        if trust == "unverified":
            for action in ("install", "update", "remove"):
                plan = manifest.get(action) or {}
                if plan and plan.get("preview_only") is not True:
                    fail(f"{item_id}: unverified {action} plan is not preview_only")

    # Legacy Go seed constructors must stay retired.
    for name in sorted(RETIRED_FUNCTIONS):
        if f"func {name}(" in catalog:
            fail("retired Go fallback function remains: " + name)

    if "integrations := bundledRegistryIntegrations()" not in catalog:
        fail("buildCatalog is not seeded from the bundled manifest registry")
    if "for _, item := range bundledRegistryIntegrations() {" not in app_center_entware:
        fail("App Center package inventory is not seeded from the bundled manifest registry")
    if "func bundledRegistryIntegrations() []catalogItem {" not in catalog:
        fail("bundled manifest integration loader missing")
    for marker in (
        "doc, err := parseRouterForgeRegistry(bundledRouterForgeRegistry)",
        'item.Kind != "integration"',
        "integrations = append(integrations, item)",
    ):
        if marker not in catalog:
            fail("bundled integration loader changed: missing " + marker)

    # Runtime remote/cache overlay must still support a registry-only integration
    # that has no pre-existing catalog item.
    for marker in (
        "if target := findCatalogItem(snapshot, incoming.ID, incoming.Kind); target != nil {",
        "if incoming.Builtin {",
        "snapshot.Integrations = append(snapshot.Integrations, incoming)",
    ):
        if marker not in registry_go:
            fail("manifest-only runtime append path changed: missing " + marker)

    print("LEGACY_FALLBACK_REMOVAL_GATE=PASS")
    print("RETIRED_LEGACY_IDS=16")
    print("LEGACY_GO_FALLBACK_REMOVED=PASS")
    print("MANIFEST_COVERAGE=PASS")
    print("REGISTRY_COVERAGE=PASS")
    print("MANIFEST_DIGEST_COVERAGE=PASS")
    print("UNVERIFIED_LIFECYCLE_NARROWING=PASS")
    print("BUNDLED_MANIFEST_BOOTSTRAP=PASS")
    print("MANIFEST_ONLY_RUNTIME_APPEND_PATH=PASS")
    print("MANIFEST_SOURCE_OF_TRUTH=PASS")
    print("P13B5_REMOVE_LEGACY_GO_FALLBACK=PASS")
    print("P13B=COMPLETE")

if __name__ == "__main__":
    main()
