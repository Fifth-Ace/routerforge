# Shared Transaction Evidence — P16F

P16F вводит единый формат доказательств для опасных mutation flow RouterForge.

Цель — перестать возвращать только `success/error` и одинаково описывать, что именно успело произойти:

`precheck → snapshot → validated → applied → verified → committed`

и аварийные состояния:

`rolled-back | ambiguous | failed`

## Общий transaction manifest

`internal/platform/transaction` теперь хранит:

- transaction ID;
- component;
- текущее state;
- время начала и обновления;
- связанные artifacts;
- bounded evidence trail;
- итоговую ошибку.

Каждая evidence-запись содержит:

- `time`;
- `stage`;
- `status`: `info`, `passed`, `failed`, `recovered`;
- короткое message;
- необязательные строковые details.

История ограничена 64 записями. При переполнении сохраняется начальный `precheck` и самые новые события.

## Ошибки с evidence

`transaction.Wrap` сохраняет исходную Go error chain и прикладывает immutable-копию manifest.

Поэтому `errors.Is` продолжает работать для доменных ошибок, а HTTP/API слой может через `transaction.Extract` вернуть пользователю тот же transaction report.

## Config Vault restore

P16C restore теперь использует общий constructor/evidence API.

Ответ restore уже содержит `transaction`, поэтому в нём дополнительно видны:

- target validation;
- safety snapshot;
- content validation;
- atomic apply;
- SHA-256 readback;
- rollback verification;
- terminal state.

`rolled-back` означает подтверждённое восстановление safety snapshot.
`ambiguous` означает, что RouterForge не смог доказать rollback.

## DNS native mutation

Native DNS create/update/delete/enable/disable теперь получают общий transaction manifest.

Evidence фиксирует:

- mutation envelope validation;
- native RCI snapshot;
- concurrent-change check;
- native apply;
- exact RCI readback;
- RouterForge metadata write, если он был нужен;
- Unix `/v1/health` runtime probe;
- commit либо rollback/ambiguous.

Успешный DNS mutation response содержит `transaction`.

Если post-write операция падает, DNS API возвращает ошибку вместе с `transaction`, при этом исходный тип ошибки сохраняется и HTTP status mapping не ломается.

## Границы P16F

P16F не превращает transaction evidence в долговременный Incident Timeline. Manifest живёт в ответе конкретной операции.

Постоянное хранение, корреляция нескольких компонентов и пользовательская временная шкала относятся к будущему P21 Incident Timeline / Health & Alerts.

Metadata-only операции над уже disabled DNS resolver, которые не вызывают native RCI mutation, пока не объявляются частью native transaction evidence.

## Следующий этап

После P16F фундамент P16 можно считать завершённым:

- Config Vault;
- safe restore;
- automatic pre-change snapshots;
- component validator/runtime probe;
- единый transaction evidence contract.

Следующий основной этап: **P17 Network Doctor + Route Inspector** с использованием готовых safety/probe/evidence primitives.
