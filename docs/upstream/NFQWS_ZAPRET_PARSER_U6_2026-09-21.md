# RouterForge U6 Zapret parser parity

Base SHA: `00c753b11ba14a333c6646d44eeb912f207ea9db`  
Pinned converter: `whxtelxs/nfqws-zapret-converter@c37858b8ffead9377f1e27de756c8f5c46c23090`

U6 moves Zapret source parsing from browser-only heuristics to the NFQWS Manager backend.

Parity covered:
- Windows batch `^` continuation;
- `winws.exe`, `nfqws.exe`, `nfqws` extraction;
- `set VAR=value` expansion with fail-closed unresolved variables;
- `%~dp0bin\`, `%~dp0lists\`, `%BIN%`, `%LISTS%`;
- `%GameFilter% -> 1024-65535`;
- `--wf-tcp` / `--wf-udp` extraction;
- ordered `--new` profile splitting;
- list/blob dependency inventory;
- converter compatibility aliases for common Google/Max QUIC and TLS fake filenames;
- both upstream log formats with strategy/target/test/pass/fail counters.

Safety:
- parsing is read-only;
- multiple process launches are not merged automatically;
- unresolved variables prevent READY;
- log parsing never fabricates strategy arguments;
- Strategy Library import remains a separate explicit action;
- active nfqws2 config/runtime is not mutated by parse.
