# E01 — общий фундамент / Foundation v1

BASE: `b5406aaa5ce7a9982200681b6c577e3edcb0549f`. Master Plan 4.0 / Engineering Protocol 1.0.

## Область реализации

Core получает отдельную страницу «Объекты политик» с каталогом черновиков устройств и групп. Дополнены существующие `internal/platform/policy` и `internal/platform/events`; второго Event Engine нет. Старые API `/api/platform/events` и `/api/platform/alerts` сохраняются, в legacy Event добавляется только поле `id`. Новые данные не меняют действующие DNS/NAT/firewall-правила. В трёх maintenance-only модулях изменений нет.

Единый JSON Schema и синтетические fixtures находятся в `internal/platform/contract`. Go проверяет схему и связи при импорте; JS проверяет тот же набор на Node.js. Python fixture reader демонстрирует чтение метрик rfs на имеющемся runtime. Это пока не production-интеграция Portal/rfs, не rfs API. Схема использует UUID для устройств; старые ID Portal сохраняются через постоянную карту old ID → UUID при будущей миграции P2, без изменения vault-ключей. Конфликт ID или неоднозначное совпадение не разрешаются автоматически. JSON-схема не содержит credentials; в текущем Core draft store ссылки credential/trust и пути кроме `/` отклоняются до реализации vault и отдельных разрешённых профилей. `list-ref` — декларативная ссылка, не разрешённый список; адаптеры потребителей и применение правил — отдельное продолжение F01.

## API (внутренний, через существующий auth middleware Core)

- `GET /api/platform/foundation/schema`: v1 JSON Schema.
- `GET /api/platform/foundation/catalog`: сохранённый каталог и до четырёх предыдущих редакций.
- `POST /api/platform/foundation/preview`: JSON каталога, `If-Match: "<etag>"`, `X-RF-Foundation: 1`. Не пишет на диск. Возвращает добавления/изменения/удаления и ограниченное разворачивание групп.
- `POST /api/platform/foundation/save`: тот же body и headers, плюс `X-RF-Confirm: SAVE_DRAFT` и `X-RF-Preview: <candidate.etag>`. Сверяет ETag и повторно проверяет содержимое. Изменение только при явном сохранении. 409 при конфликте; 400 при невалидной схеме; 403 при cross-origin или отсутствии защитного заголовка.
- `GET /api/platform/foundation/events`: безопасная проекция уже существующего Ring, фильтры component, severity, type, recovery, since/until, limit. Не отдаёт raw context, message, object path, transaction payload. Не экспортирует неизвестные типы и не добавляет адаптеры защитных модулей. `observerId` идентифицирует процесс Core, не постоянный Device ID; device/object/correlation остаются null, пока нет проверенной связи. Текущий Ring volatile: после рестарта предыдущие события не обещаются.

Каталог хранится в отдельном `policy-catalog-v1.json` рядом с существующей конфигурацией Core (`/opt/etc/routerforge`). До явного сохранения файла нет. Сохранение через временный файл в том же каталоге, sync и rename. Лимит каталога 256 KiB, история 4 редакции, не более 100 групп, 1024 элемента на группу, 4096 развёрнутых элементов в одной группе и 8192 суммарно в preview. Идентификатор поколения предотвращает ABA-конфликты; ревизии изменившихся объектов назначаются на сервере. Удаление объекта с существующей ссылкой отвергается при preview/save. Неизвестные/повреждённые данные не перезаписываются автоматически. UI имеет RU/EN, отдельный черновик, preview, явное сохранение, восстановление через новый черновик, экспорт сохранённых метаданных.

## Безопасность и ограничения

Не выполняем сетевые запросы из каталога, не открываем listener, не добавляем токены/пароли, не запускаем сторонние команды. Новый API наследует текущую настройку auth Core; мутации требуют JSON и отдельный action header, origin gate, ETag и подтверждение. Не расширяем существующие scopes. Нет новой зависимости в Go/npm; полный аудит старых лицензий всех продуктов не закрывается одним этим изменением. Аппаратный baseline перенесён на приёмку фактического модуля; Portal main-only остаётся неизменённым до P2. Никаких изменений main или релизов. Фактическая работоспособность на роутере требует согласованной установки позднее.

## Проверки и продолжение

`go test ./internal/platform/{contract,policy,events}` и `go test ./components/core -run '^TestFoundation'`; `node internal/platform/contract/check.mjs`; `python internal/platform/contract/rfs_fixture.py`; сборка Core frontend штатными npm scripts. CI обычный dev push с `[rf-ci-only]`; без release dispatch. Следующий функциональный объём Wave I: adapter-specific read-only ссылки DNS Router/NFQWS/Network Tools, реальные mapping device/object/correlation для активных источников, применение через существующие UI после явного подтверждения, межпродуктовый import/export и миграция Portal перед P2. Не объявлять P1/F01/F02 целиком принятыми до проверки этих интеграций. F03/F04 и production rfs JSON/API относятся к последующим согласованным этапам.
