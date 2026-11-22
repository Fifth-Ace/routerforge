# G / Source-only cross-module read lifetime and mutation preservation

Full consolidated repository, built on Stage 1 F (SHA256 `1b25cd6872543fea7dfacb35e454ad6788e81c9b0f0e3f611caeff51279a69cc`). Relevant source:

- `components/core/frontend/src/lib/{http,api,readSessions}.js`
- `modules/monitoring/frontend/{MonitoringModuleApp,EventsPane}.svelte`
- `modules/admin/frontend/{AdminModuleApp,FileManagerPane,FileEditorDrawer,FilePropertiesModal,TerminalPane}.svelte`
- `modules/crowdsec-manager/frontend/{CrowdSecManagerApp.svelte,readSessions.js}`
- `modules/dns/frontend/DNSModuleApp.svelte`
- `modules/dns-router-runtime/frontend/{DnsRuntimeMode,DnsRuntimeDiagnostics}.svelte`
- `modules/dns-router-lite/frontend/{DnsLiteApi.js,DnsLiteControllerLogic.js}`

GET cancellation and stale response protection are managed by the Svelte UI owner; protected writes never use that owner AbortSignal. CrowdSec is standalone and carries its own small reader scope. Browser/build/API/security/Linux checks deliberately not executed in Stage 1. Next: final missing owner/source map. Git/CI/router untouched.
