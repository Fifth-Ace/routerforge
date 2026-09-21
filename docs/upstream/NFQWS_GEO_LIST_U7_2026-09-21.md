# RouterForge U7 GeoSite / GeoIP / List Intelligence

Base SHA: `67ff97e0d2e1a30140b01432fecdc29560b18ff5`

Pinned upstream reference:
- `Omn1z/nfqws2-keenetic-strategy-selector@bf4e810ef22ffb6671e97dc411234ba9430909c9`
- `internal/tools/geo/geo.go`
- `internal/tools/geo/lookup.go`
- `internal/app/app_geo.go`
- `internal/app/app_geo_update.go`

RouterForge adaptation:
- upload SHA256-guarded `geosite.dat`, `geoip.dat`, or text assets;
- persistent source/ref provenance metadata;
- list categories and exact counts;
- preview a category with deterministic de-duplication;
- explicitly save/extract a selected category into a safe `.list`;
- backup on list replacement;
- never edit `nfqws2.conf` or restart/reload the service merely because an asset or list changed;
- expose installed nfqws2 autohostlist capability only; RouterForge does not implement a second autohostlist engine.

Limits:
- assets: 24 MiB;
- preview/extract response limit: 20,000 entries per operation;
- protobuf regex domain entries are skipped, matching the reviewed upstream parser.
