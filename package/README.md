# DNS Monitor — manual release package

This directory contains files used for the manual GitHub Release archive.

For normal installation on Keenetic / Netcraze ARM64 routers, the recommended
method is the Entware/opkg repository:

```sh
wget -qO- https://raw.githubusercontent.com/Fifth-Ace/dns-monitor/main/scripts/install-repo.sh | sh
```

## Manual installation

A release archive contains:

```text
dns-monitor-linux-arm64
S90dns-monitor
install.sh
uninstall.sh
LICENSE
```

After extracting the archive on the router:

```sh
./install.sh
```

The installer places:

```text
/opt/bin/dns-monitor
/opt/etc/init.d/S90dns-monitor
/opt/share/licenses/dns-monitor/LICENSE
```

The event log is stored at:

```text
/opt/var/log/dns-monitor.log
```

Web UI:

```text
http://<router-ip>:2233
```

Manual uninstall:

```sh
./uninstall.sh
```

Logs are intentionally preserved during uninstall.

For project documentation, build instructions and API details, see the
repository root [README](../README.md).

DNS Monitor is distributed under the [MIT License](../LICENSE).
