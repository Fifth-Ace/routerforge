# Исторические тесты DNS Router Runtime до Svelte single-root

В этих 15 файлах проверялся `modules/dns-router-runtime/frontend/app.js`, который являлся исполняемым legacy-контроллером. После миграции Runtime этот контроллер удалён. Сохранены **исходные байты** старых контрактов для ревизии, ни одна проверка не объявлена успешной в обход исполнения.

## Новые исполняемые проверки

- `tests/dns_runtime_svelte5_behavior.test.mjs`: 26 тестов реального Svelte-owned draft/API logic (CAS, dirty, 403/409, import, timeout, cancellation, security).
- `tests/dns_runtime_native_parity.test.mjs`: 18 тестов реальных чистых функций из Runtime modules: Keenetic native catalog/zone, DNS группировка/replace, P88 explicit planning/probe, P92 домены, P91 DNS/TLS verification.
- `p87_native_policy_ui_contract.test.js`: ранее проходившие тесты остаются активными.
- Chromium CDP browser/API/CAS mock gate (отдельный локальный runner): пользователь подтвердил PASS 09.11.2026 на bundle `200685449d5bad304649f3b4702c8b9c67b505a5f3676f49dd7fced5a7d0689a`, ещё не запускался после изменения только тестов/verifiers.
- Go tests/vet/race и experimental IPK: **PENDING/BLOCKED**, не заменяются source-level assertions.

| Бывшая группа | Действующая замена / дополнительный gate |
|---|---|
| P10 grouped, native group catalog/edit | Node P10 planner + браузерный dirty/CAS + Go API/capability |
| P10 e2e, managed/trial/durable | Go `TestDPR2*`, live nonpublishing experimental IPK, rollback |
| P10 Lite Svelte | отдельный Lite модуль L01–L08, ещё НЕ DONE |
| P85/P87/P88 | Native catalog + P88 plan/probe + браузер + Go native bridge |
| P89 | Native catalog + зонирование/отклонение неполного инвентаря |
| P90/P91 | P91 lookup/TLS pure logic + Go certificates/lookup; браузер полных форм PENDING |
| P92 | Parser/import/export unit, браузерный stage/CAS; визуальная parity PENDING |

Этот перенос **не** означает автоматического выполнения всех исторических пользовательских сценариев. `RUNTIME_MODULE_DONE=PENDING` до real component parity, backend Go/race/vet, experimental IPK, upgrade/rollback и разрешённого exact-SHA CI. Source-only gates остаются вспомогательными, их PASS недостаточен.
