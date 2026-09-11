# Диагностика RouterForge

```sh
echo '=== PACKAGES ==='
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
echo '=== PROCESSES ==='
ps w | grep '[r]outerforge'
echo '=== LISTEN ==='
netstat -lnp 2>/dev/null | grep ':2233'
echo '=== SOCKETS ==='
ls -l /opt/var/run/routerforge-*.sock 2>/dev/null
echo '=== CORE HEALTH ==='
wget -qO- http://127.0.0.1:2233/api/health
```

Core — единственный RouterForge LAN listener `:2233`.

## Module health

```sh
wget -qO- http://127.0.0.1:2233/api/modules/dns/health
wget -qO- http://127.0.0.1:2233/api/modules/admin/health
wget -qO- http://127.0.0.1:2233/api/modules/monitoring/health
```

## Monitoring migration

```sh
ROUTERFORGE_MONITORING_EXPECTED_VERSION='0.7.1' \
  sh scripts/verify-monitoring-migration.sh runtime
```

Нормально: один `routerforge-monitoring` process/package и 5 sockets including compatibility names.

## Management / Terminal

```sh
/opt/bin/opkg status routerforge-admin
ps w | grep '[r]outerforge-admin'
ls -l /opt/var/run/routerforge-admin.sock
ndmc -c 'show version'
```

## DNS

```sh
/opt/bin/opkg status routerforge-dns
ps w | grep '[r]outerforge-dns'
ls -l /opt/var/run/routerforge-dns.sock
wget -qO- http://127.0.0.1:2233/api/modules/dns/health
```

## App Center stale state
Нажмите **Проверить обновления**, затем сравните `opkg status`/`list-installed`. Не удаляйте cache без причины.

## Не делайте broad `opkg upgrade`
Для RouterForge используйте App Center или точечный lifecycle.

Bug report: модель, KeeneticOS, architecture, channel, package versions, sanitized logs/health. Не публикуйте credentials, cookies, VPN keys, private names, public IPs или MAC.
