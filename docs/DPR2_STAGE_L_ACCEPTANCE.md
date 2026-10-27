# RouterForge DNS Router — DPR2-L (Managed listener and domain-zone plan)

**Target:** `feature/dns-router-r7-stage-l` only, experimental version `0.14.0~dev.dpr2l`. **Base:** exact K commit `43618cae466868e6aae174982cca7cddb57bf8ea`, CI Run `40593766838` PASS.

## Hardware grounding

- DPR2-I DSTest ARM64: 16 live DNS/TLS cases and 200 query benchmark PASS, live RSS about 12.5 MiB.
- DPR2-J: ndnproxy UDP/TCP domain route via temporary relay PASS, exact DNS inventory rollback PASS.
- DPR2-K: native private LAN listener on 192.168.10.1:55358 with exact source-IP ACL, direct ndnproxy UDP/TCP, exact rollback PASS, no relay. GitHub Stage K exact CI RUN `40593766838` SUCCESS; no stable/main/beta/dev publishing.

## Scope of DPR2-L

1. Optional `listener` object in the existing CAS-managed `config.json` with `mode: loopback|lan`, canonical high-port `listen`, and exact private/loopback IPv4 `allowed_clients` in LAN mode; absence means unmodified legacy behavior.
2. ConfigStore.Save/Compile rejects wildcard, public, IPv6 LAN, port 53, duplicate or non-private ACL source IPs, while preserving existing policy validation.
3. Shadow lifecycle Start/Apply resolves the saved listener with `--shadow-lan-experimental` as an **additional runtime capability gate**. Saving settings does not start anything. Apply retains previous runtime snapshot on replacement failure.
4. `GET /v1/keenetic/domain-plan` returns an **in-memory preview** of domain-scoped ndmc add/remove strings for suffix rules only; groups subdomains under parent zones, warns on exact/subdomain-only unmatched rules; no writes or ndmc execution. It carries the exact configuration revision. Max 32 zones for a Ready plan.
5. Native UI adds a listener editor plus preview. Mutations use existing `PUT /v1/config` with `If-Match` CAS; shadow lifecycle remains manual. No new services or auto-start.
6. Explicit negative tests for invalid listener/ACL and missing LAN permission, CAS, preview-only API and no implicit fallback. Go race/vet, frontend syntax, three experimental IPK architectures verified.

## IMPORTANT: NOT INCLUDED in L

- `ndmc` operations, persistent DNS interception, service autostart, router installation, stable/beta release, or production rollout.
- Exact transactional DNS Proxy Apply/Rollback with independent crash watchdog: **next isolated DSTest stage**, after this software CI passes.
- An active GUI run on a router: this stage only ships the code to the feature branch, not a hardware release.

## Example saved JSON portion

```json
{
  "listener": {
    "mode": "lan",
    "listen": "192.168.10.1:55358",
    "allowed_clients": ["192.168.10.1", "127.0.0.1"]
  }
}
```

The object above is **partial** and must be combined with full schema_version/upstreams/rules. `--serve --shadow-lan-experimental` enables the *possibility* of a manual LAN Start, without starting the listener on daemon initialization.
