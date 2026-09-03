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

## 7. Marketplace не видит обновление

Сначала нажмите **«Проверить обновления»**.

В новой архитектуре эта кнопка делает force refresh Registry + release-index.

Автоматическая remote-проверка — примерно раз в час.

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

## 10. DNS

Если DNS-раздел отсутствует:

```sh
/opt/bin/opkg status routerforge-dns 2>/dev/null
ls -l /opt/etc/routerforge/dns.enabled 2>/dev/null
```

Если DNS-раздел есть, но discovery/capture сообщает ошибку, проверьте:

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
