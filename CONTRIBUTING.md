# Contributing

DNS Monitor is currently focused on Keenetic / Netcraze ARM64 routers running Entware.

Before opening a bug report, please include:

- router model;
- KeeneticOS/NDMS version;
- CPU architecture;
- DNS Monitor version;
- relevant DNS Monitor log lines;
- sanitized output where configuration details are required.

Please remove public IP addresses, MAC addresses, device names, domains that reveal private infrastructure, credentials and tunnel secrets before posting diagnostics publicly.

For code changes:

```sh
go test ./...
go vet ./...
```

Keep router-specific parsing defensive: Keenetic output can vary between firmware versions and hardware families.
