# Диагностика RouterForge

Все команды ниже предполагают Entware shell на роутере.

## 1. Core установлен?

```sh
/opt/bin/opkg status routerforge-core 2>/dev/null
```

Или:

```sh
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

## 2. Core запущен?

```sh
ps w | grep '[r]outerforge'
```

Restart:

```sh
/opt/etc/init.d/S90routerforge restart
```

## 3. Слушается ли порт 2233?

```sh
netstat -lnp 2>/dev/null | grep ':2233'
```

Ожидается RouterForge Core.

## 4. Health

```sh
wget -qO- http://127.0.0.1:2233/api/health
echo
```

Если `curl` удобнее:

```sh
curl -fsS http://127.0.0.1:2233/api/health
```

## 5. Лог Core

```sh
tail -n 100 /opt/var/log/routerforge.log
```

Live:

```sh
tail -f /opt/var/log/routerforge.log
```

## 6. Optional modules

Пакеты:

```sh
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

Unix sockets:

```sh
ls -l /opt/var/run/routerforge-*.sock 2>/dev/null
```

Если конкретный monitoring module установлен, но UI показывает OFFLINE:

1. проверьте его package;
2. проверьте init script;
3. проверьте Unix socket;
4. перезапустите конкретный service, затем Core.

## 7. Marketplace не видит обновление или показывает старую Installed version

Сначала нажмите **«Проверить обновления»**. Кнопка делает force refresh Registry + release-index; автоматическая remote-проверка выполняется примерно раз в час.

Проверьте реальное состояние `opkg`:

```sh
/opt/bin/opkg status routerforge-core 2>/dev/null | grep -E '^(Package|Version|Status):'
echo
/opt/bin/opkg status routerforge-dns 2>/dev/null | grep -E '^(Package|Version|Status):'
echo
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

`opkg status` может содержать одновременно текущую `installed` stanza и старую `not-installed` stanza. Core 0.4.3+ игнорирует `not-installed` tombstones. Если `opkg list-installed` и Marketplace расходятся на Core 0.4.3+, приложите полный `opkg status` нужного package к bug report.

Cache:

```sh
ls -l /opt/var/cache/routerforge/ 2>/dev/null
```

Не удаляйте cache без причины: при временной недоступности GitHub RouterForge может использовать last-good state.

## 8. Проверить release channel

В Marketplace отображается текущий channel.

Beta и Stable используют разные release-index:

```text
routerforge-beta-index.json
routerforge-stable-index.json
```

Если вы случайно смешали beta/stable packages, установите bootstrap целевого канала и затем обновите остальные RouterForge components через Marketplace.

## 9. Auth

Config:

```sh
cat /opt/etc/routerforge/security.json 2>/dev/null
```

При включённой авторизации используется Entware `root`.

Не публикуйте содержимое `/opt/etc/shadow`, cookies или password hashes.

Если UI после изменения auth выглядит устаревшим, выполните hard refresh браузера после проверки, что `/api/health` отвечает.

## 10. DNS / Module ABI

Сначала разделите package state и runtime state:

```sh
echo "=== PACKAGE ==="
/opt/bin/opkg status routerforge-dns 2>/dev/null | grep -E '^(Package|Version|Status):'

echo
echo "=== MANIFEST ==="
cat /opt/share/routerforge/modules/dns/manifest.json 2>/dev/null

echo
echo "=== SOCKET ==="
ls -l /opt/var/run/routerforge-dns.sock 2>/dev/null

echo
echo "=== PROCESS ==="
ps w | grep '[r]outerforge-dns'

echo
echo "=== HEALTH ==="
wget -qO- http://127.0.0.1:2233/api/modules/dns/health
echo

echo
echo "=== LAST STARTS ==="
grep 'START' /opt/var/log/routerforge-dns.log 2>/dev/null | tail -10
```

Во время реального update/restart краткий `503` для module API допустим, пока Unix socket поднимается. На Core 0.4.2+ официальный UI сам повторяет health-check и iframe не должен застревать на сыром JSON.

Если runtime уже `ok:true`, но Marketplace показывает другую Installed version, вернитесь к разделу 7 и сравните `opkg status`/`list-installed`.

Если DNS runtime жив, но discovery/capture сообщает ошибку, проверьте Keenetic:

```sh
ndmc -c 'show dns-proxy'
ndmc -c 'show ip hotspot'
ndmc -c 'show ip policy'
```

Вывод может содержать чувствительные данные — санитизируйте его перед публикацией.

## 11. Не делайте broad upgrade ради RouterForge

Для RouterForge не требуется массово обновлять весь Entware:

```text
opkg upgrade
```

не должен быть первым шагом диагностики.

Используйте Marketplace или точечную установку RouterForge package.

## 12. Что приложить к bug report

Без секретов:

```sh
uname -a
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
netstat -lnp 2>/dev/null | grep ':2233'
tail -n 100 /opt/var/log/routerforge.log
```

Также укажите:

- модель роутера;
- KeeneticOS/NDMS version;
- RouterForge Core version;
- versions проблемных modules;
- Stable или Beta;
- что именно воспроизводит проблему.
