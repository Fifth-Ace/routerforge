# Contributing

DNS Monitor is focused on Keenetic / Netcraze ARM64 routers running Entware.

## Development workflow

- `main` contains stable public releases.
- `dev` is used for active development.
- Feature branches should normally target `dev`.
- Release changes are merged from `dev` into `main` after testing.

## Before submitting code

Run:

```sh
gofmt -w .
go test ./...
go vet ./...
```

For packaging or installer changes also verify:

```sh
sh -n package/install.sh
sh -n package/uninstall.sh
sh -n package/S90dns-monitor
sh -n scripts/install-repo.sh
sh -n scripts/remove-repo.sh
sh -n scripts/build-opkg.sh
sh -n scripts/build-feed.sh
```

Keep Keenetic-specific parsers defensive: command output can vary between
firmware versions and hardware families.

## Bug reports

Please include:

- router model;
- KeeneticOS/NDMS version;
- CPU architecture;
- DNS Monitor version;
- relevant DNS Monitor log lines;
- sanitized diagnostic output when configuration details are required.

Do not publish credentials, tunnel secrets, private keys, public IP addresses,
MAC addresses, private hostnames or domains that reveal private infrastructure.
