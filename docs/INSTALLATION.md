# Установка и обновление RouterForge

## Требования

- Keenetic / Netcraze с Entware, установленным в `/opt`.
- Доступ к терминалу с правами root и соединение с интернетом.
- `opkg`, `sha256sum` и `curl` или `wget`.

Актуальные версии Stable и Beta отображаются в [главном README](../README.md) и на [странице выпусков](https://github.com/Fifth-Ace/routerforge/releases). Компоненты могут иметь независимые версии.

## Установка Stable

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

## Установка Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Bootstrap устанавливает Core. Необязательные модули выбираются в Центре приложений RouterForge. Адрес интерфейса: `http://<ip-роутера>:2233`.

## Проверка установки

```sh
wget -qO- http://127.0.0.1:2233/api/health
echo
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

## Обновление

1. Убедитесь, что выбран нужный канал — Stable или Beta.
2. Запустите bootstrap целевого канала.
3. В Центре приложений проверьте обновления установленных модулей.
4. Примените доступные обновления; не удаляйте файлы пакетов вручную.

Stable и Beta используют разные индексы выпусков. При переходе к каналу с более старой версией пакета обычное обновление не выполняет автоматический downgrade. Перед изменением канала сохраните конфигурацию и проверьте совместимость.

## Поддерживаемые архитектуры

| Архитектура | Статус |
| --- | --- |
| `aarch64-3.10` | Основная, проверенная на оборудовании |
| `mipsel-3.4` | Experimental |
| `mips-3.4` | Experimental |

Bootstrap выполняет проверки совместимости; несовместимые условия блокируют установку. Подробности — в [матрице архитектур](ARCHITECTURES.md).

## Безопасность

Для функций, изменяющих правила firewall или удалённый доступ, обеспечьте независимый резервный доступ к роутеру. Состояние установленной службы само по себе не доказывает работоспособность защиты. Подробные проверки и ограничения описаны в интерфейсах соответствующих модулей.

## Управление Core

```sh
/opt/etc/init.d/S90routerforge restart
tail -f /opt/var/log/routerforge.log
```

## Удаление

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/remove-repo.sh | sh
```
