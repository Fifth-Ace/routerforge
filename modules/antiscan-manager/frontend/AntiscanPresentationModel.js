// Read-only Antiscan presentation model. Escaped by Svelte text bindings; no DOM access.
function fmtDuration(seconds) {
  if (seconds === undefined || seconds === null || seconds === '') return '—';
  const value = Number(seconds);
  if (!Number.isFinite(value) || value < 0) return '—';
  if (value === 0) return 'без таймаута';
  if (value % 86400 === 0) return `${value / 86400} дн.`;
  if (value % 3600 === 0) return `${value / 3600} ч.`;
  if (value % 60 === 0) return `${value / 60} мин.`;
  return `${value} сек.`;
}


function boolText(value) {
  return value ? 'Включено' : 'Выключено';
}

function fmtBytes(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number < 0) return '—';
  if (number < 1024) return `${number} B`;
  if (number < 1024 * 1024) return `${(number / 1024).toFixed(1)} KiB`;
  if (number < 1024 * 1024 * 1024) return `${(number / 1024 / 1024).toFixed(1)} MiB`;
  return `${(number / 1024 / 1024 / 1024).toFixed(1)} GiB`;
}

function fmtAuditTime(value) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return String(value || '—');
  return parsed.toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'medium' });
}


const exactMessageTranslations = {
  'POST required': 'Для этого действия требуется POST-запрос.',
  'authorized RouterForge Core request required': 'Действие разрешено только через авторизованный RouterForge Core.',
  'another Antiscan mutation is already running': 'Другая операция Antiscan уже выполняется. Дождитесь её завершения.',
  'multiple JSON values are not allowed': 'В запросе передано несколько JSON-значений.',
  'invalid unban request': 'Некорректный запрос на снятие блокировки.',
  'confirm must equal UNBAN': 'Не подтверждено снятие блокировки.',
  'invalid list-entry request': 'Некорректный запрос на изменение пользовательского списка.',
  'confirm must equal ADD': 'Не подтверждено добавление записи.',
  'invalid lifecycle request': 'Некорректная команда управления Antiscan.',
  'lifecycle action must be start, stop, reload or restart': 'Допустимы запуск, остановка, перечитывание конфигурации или штатный перезапуск.',
  'invalid operation request': 'Некорректная сервисная операция Antiscan.',
  'confirm must equal RUN': 'Сервисная операция не подтверждена.',
  'Antiscan must be running before this operation': 'Для этой сервисной операции Antiscan должен быть запущен.',
  'Antiscan stopped during the operation': 'Во время сервисной операции Antiscan неожиданно остановился.',
  'upstream operation returned while an Antiscan reload lock is still present': 'Штатная операция завершилась, но Antiscan всё ещё держит lock-файл.',
  'iptables binary not found': 'Исполняемый файл iptables не найден.',
  'Antiscan rule chain is already present; update_rules was not repeated to avoid duplicate jump rules.': 'Цепочка Antiscan уже на месте; повторный update_rules не запускался, чтобы не создать дублирующие переходы.',
  'ENABLE_IPS_BAN is disabled; there are no candidate sets to process.': 'ENABLE_IPS_BAN выключен — обрабатывать кандидатов сейчас нечего.',
  'READ_NDM_LOCKOUT_IPSETS is disabled; there is no active Keenetic lockout import to refresh.': 'Импорт блокировок Keenetic выключен — обновлять системный список сейчас нечего.',
  'SAVE_IPSETS is disabled; no persistent ipset export was requested.': 'SAVE_IPSETS выключен — сохранять runtime-наборы в файлы сейчас не требуется.',
  'No custom blocking or exclusion list is enabled; there is nothing to reload.': 'Пользовательские блокирующие списки и исключения выключены — перечитывать нечего.',
  'Geo blocking and Geo exclusions are disabled; there is nothing to download or reload.': 'Geo-блокировка и Geo-исключения выключены — обновлять Geo сейчас нечего.',
  'Antiscan was stopped; upstream restart will start it.': 'Antiscan был остановлен; штатная команда restart запустила его.',
  'Antiscan is already running; no lifecycle command was executed.': 'Antiscan уже запущен; дополнительных действий не выполнялось.',
  'Antiscan is already stopped; no lifecycle command was executed.': 'Antiscan уже остановлен; дополнительных действий не выполнялось.',
  'Antiscan must be running before reload': 'Для перечитывания конфигурации Antiscan должен быть запущен.',
  'Antiscan reload is already in progress': 'Antiscan уже перечитывает конфигурацию. Дождитесь завершения.',
  'Antiscan reload is in progress': 'Antiscan сейчас перечитывает конфигурацию. Дождитесь завершения.',
  'Antiscan init script is unavailable or not executable': 'Штатный скрипт запуска Antiscan отсутствует или не исполняется.',
  'Antiscan init script is unavailable': 'Штатный скрипт запуска Antiscan недоступен.',
  'Antiscan is not running': 'Antiscan не запущен.',
  'Antiscan is not detected on this device': 'Antiscan не обнаружен на этом устройстве.',
  'Antiscan runtime marker is absent; live blocking state is not authoritative': 'Маркер работы Antiscan отсутствует; текущее состояние блокировок нельзя считать достоверным.',
  'ipset is unavailable; live membership cannot be checked': 'Команда ipset недоступна; проверить текущее содержимое наборов невозможно.',
  'ipset binary not found': 'Исполняемый файл ipset не найден.',
  'custom whitelist mode is configured but the runtime ipset is absent': 'Включён пользовательский белый список, но его активный набор ipset отсутствует.',
  'geo whitelist mode is configured but the runtime ipset is absent': 'Включён Geo-белый список, но его активный набор ipset отсутствует.',
  'Mobile carrier address pools can legitimately rotate through many addresses in one /24; review candidate retention and subnet threshold before changing policy.': 'Мобильные операторы могут легитимно выдавать много адресов из одной /24. Перед изменением политики проверьте время хранения кандидатов и порог блокировки подсети.',
  'invalid config request': 'Некорректный запрос на изменение конфигурации.',
  'confirm must equal APPLY_CONFIG': 'Не подтверждено применение конфигурации.',
  'ascn.conf changed since it was loaded; refresh before applying': 'ascn.conf изменился после загрузки формы. Обновите данные перед применением.',
  'ascn.conf already matches the requested settings.': 'ascn.conf уже соответствует выбранным настройкам.',
  'IPSETS_DIRECTORY changed. Upstream Antiscan requires a stop/start cycle before the new storage directory is fully active.': 'Изменён IPSETS_DIRECTORY. Для полного перехода на новый каталог нужно остановить и снова запустить Antiscan.',
  'Antiscan is stopped. Settings were stored and will become active on the next start.': 'Antiscan остановлен. Настройки сохранены и вступят в силу при следующем запуске.',
  'PORTS and PORTS_FORWARDED cannot both be empty': 'Поля PORTS и PORTS_FORWARDED не могут быть пустыми одновременно.',
  'HONEYPOT_PORTS is required when honeypot is enabled': 'При включённой ловушке необходимо указать HONEYPOT_PORTS.',
  'RULES_MASK must be a dotted IPv4 mask': 'RULES_MASK должен быть маской IPv4 в точечной записи.',
  'GEOBLOCK_COUNTRIES is required when GEOBLOCK_MODE is enabled': 'При включённом GEOBLOCK_MODE необходимо указать GEOBLOCK_COUNTRIES.',
  'IPSETS_DIRECTORY contains unsupported characters': 'IPSETS_DIRECTORY содержит недопустимые символы.',
  'IPSETS_DIRECTORY is required for persistence or Geo lists': 'Для сохранения наборов или Geo-списков необходимо указать IPSETS_DIRECTORY.',
  'IPSETS_DIRECTORY is required by the selected persistence/Geo settings': 'Выбранные настройки хранения или Geo требуют IPSETS_DIRECTORY.',
  'IPSETS_DIRECTORY must be an absolute path without parent traversal': 'IPSETS_DIRECTORY должен быть абсолютным путём без переходов через ..',
  'IPSETS_DIRECTORY contains an invalid path segment': 'IPSETS_DIRECTORY содержит недопустимый сегмент пути.',
  'IPSETS_DIRECTORY must not be /opt/etc': 'Сам каталог /opt/etc использовать нельзя; вложенные каталоги внутри /opt/etc допустимы upstream Antiscan.',
  'IPSETS_DIRECTORY must be under /opt or /tmp': 'IPSETS_DIRECTORY должен находиться внутри /opt или /tmp.',
  'SAVE_IPSETS is enabled but IPSETS_DIRECTORY is empty': 'SAVE_IPSETS включён, но IPSETS_DIRECTORY не задан.',
  'IPSETS_DIRECTORY is unavailable': 'Каталог IPSETS_DIRECTORY недоступен.',
  'invalid custom-list request': 'Некорректный запрос изменения пользовательского списка.',
  'custom list action must be add, delete, clear or reload': 'Допустимы только добавление, удаление, очистка или перечитывание пользовательского списка.',
  'list must be blacklist, whitelist or exclude': 'Можно выбрать только чёрный список, белый список или исключения.',
  'reload does not accept an entry': 'Для перечитывания списка адрес указывать не нужно.',
  'clear does not accept an entry': 'Для очистки списка адрес указывать не нужно.',
  'empty active custom whitelist reload is blocked to prevent an empty-whitelist lockout': 'Перечитывание пустого активного whitelist заблокировано: пустой whitelist способен перекрыть доступ. Сначала добавьте хотя бы один адрес или отключите whitelist.',
  'Custom list is not configured; there is nothing to reload.': 'Этот пользовательский список сейчас выключен — перечитывать runtime нечего.',
  'Antiscan is stopped; reload was not executed.': 'Antiscan остановлен — перечитывание runtime не выполнялось.',
  'Entry already exists in the custom list.': 'Такая запись уже есть в исходном пользовательском списке.',
  'Entry was already absent from the custom list.': 'Этой записи уже нет в исходном пользовательском списке.',
  'Antiscan is stopped; source file was updated without runtime reload.': 'Исходный файл обновлён, но Antiscan остановлен — runtime не перечитывался.',
  'Custom list is not configured; source file was updated without runtime reload.': 'Исходный файл обновлён. Этот список сейчас выключен, поэтому runtime не перечитывался.',
  'custom list file verification failed; original file was restored': 'Проверка исходного файла не пройдена; предыдущий файл восстановлен.',
  'Entry was already absent from the runtime set.': 'Этой записи уже нет в активном наборе.',
  'Entry already exists in the upstream custom list file.': 'Запись уже есть в пользовательском списке Antiscan.',
  'Entry is stored, but Antiscan is stopped; it will become effective when the matching list mode is loaded.': 'Запись сохранена. Antiscan остановлен, поэтому она начнёт действовать после следующего запуска соответствующего режима списка.',
  'Entry is stored, but USE_CUSTOM_EXCLUDE_LIST is disabled in ascn.conf.': 'Запись сохранена, но USE_CUSTOM_EXCLUDE_LIST сейчас выключен в ascn.conf.',
  'Entry is stored, but CUSTOM_LISTS_BLOCK_MODE is not whitelist.': 'Запись сохранена, но CUSTOM_LISTS_BLOCK_MODE сейчас не работает в режиме whitelist.',
  'unban verification failed: entry is still present': 'Проверка снятия блокировки не пройдена: запись всё ещё присутствует.',
  'unban verification failed; runtime state was restored': 'Снять блокировку не удалось; прежнее состояние восстановлено.',
  'ipset verification unavailable; file/runtime were restored': 'Проверить ipset не удалось; файл и рабочее состояние восстановлены.',
  'custom list verification failed; file/runtime were restored': 'Проверка пользовательского списка не пройдена; файл и рабочее состояние восстановлены.',
  'unban target must be an IPv4 address': 'Для снятия блокировки нужен IPv4-адрес.',
  'ascn_subnets unban target must be an IPv4 /24 prefix': 'Для ascn_subnets нужно указать IPv4-подсеть /24.',
  'single-entry unban is allowed only for ascn_ips, ascn_subnets and ascn_honeypot': 'Точечное снятие блокировки разрешено только для ascn_ips, ascn_subnets и ascn_honeypot.',
  'custom list entry must be an IPv4 address or CIDR prefix': 'Запись списка должна быть IPv4-адресом или CIDR-подсетью.',
  'list must be exclude or whitelist': 'Можно выбрать только список исключений или белый список.',
  'custom list exceeds RouterForge safety limit': 'Пользовательский список превышает безопасный лимит RouterForge.',
  'custom list would exceed RouterForge safety limit': 'После добавления пользовательский список превысит безопасный лимит RouterForge.',
  'custom list is not a regular file': 'Файл пользовательского списка имеет неподдерживаемый тип.',
  'unknown Antiscan ipset': 'Неизвестный набор ipset Antiscan.',
  'invalid IPv4 address': 'Некорректный IPv4-адрес.',
  'history limit must be between 1 and 100': 'Количество событий истории должно быть от 1 до 100.',
  'invalid Antiscan history limit': 'Некорректный лимит истории Antiscan.',
  'invalid RCI token request': 'Некорректный запрос управления RCI-токеном.',
  'RCI token action must be set, check or delete': 'Допустимы только установка, проверка или удаление RCI-токена.',
  'confirm must equal SET_TOKEN': 'Не подтверждена установка RCI-токена.',
  'confirm must equal CHECK_TOKEN': 'Не подтверждена проверка RCI-токена.',
  'confirm must equal DELETE_TOKEN': 'Не подтверждено удаление RCI-токена.',
  'RCI token must not be empty': 'RCI-токен не может быть пустым.',
  'RCI token exceeds RouterForge safety limit': 'RCI-токен превышает безопасный лимит RouterForge.',
  'RCI token must contain only ASCII letters and digits': 'RCI-токен может содержать только латинские буквы и цифры.',
  'RCI token flow is not supported by this firmware': 'Эта прошивка не поддерживает token-flow Antiscan.',
  'RCI token/key pair is incomplete': 'Комплект token/key Antiscan неполный.',
  'RCI token check does not accept a token value': 'Для проверки используется уже сохранённый токен; новое значение передавать не нужно.',
  'RCI token delete does not accept a token value': 'Для удаления значение токена передавать не нужно.',
  'upstream RCI token check failed': 'Сохранённый RCI-токен не прошёл проверку Keenetic.',
  'upstream RCI token set failed; previous encrypted token state was restored': 'Установить RCI-токен не удалось; прежнее зашифрованное состояние восстановлено.',
  'RCI token set verification failed; previous encrypted token state was restored': 'Проверка сохранения RCI-токена не пройдена; прежнее зашифрованное состояние восстановлено.',
  'RCI token validation failed; previous encrypted token state was restored': 'Новый RCI-токен не прошёл проверку; прежнее зашифрованное состояние восстановлено.',
  'upstream RCI token delete failed; encrypted token files were restored': 'Удалить RCI-токен не удалось; зашифрованные token/key файлы восстановлены.',
  'RCI token delete verification failed; encrypted token files were restored': 'Проверка удаления RCI-токена не пройдена; зашифрованные token/key файлы восстановлены.',
  'RCI token/key files are already absent.': 'RCI token/key уже отсутствуют.',
  'Antiscan stopped because this firmware requires RCI authentication after token deletion.': 'После удаления токена Antiscan остановлен: эта прошивка требует RCI-аутентификацию.',
  'invalid scheduler request': 'Некорректный запрос изменения расписания Antiscan.',
  'confirm must equal APPLY_SCHEDULE': 'Не подтверждено применение расписания Antiscan.',
  'ascn_crontab.conf changed since it was loaded; refresh before applying': 'ascn_crontab.conf изменился после загрузки. Обновите расписание перед применением.',
  'ascn_crontab.conf contains lines outside the pinned upstream scheduler contract': 'В ascn_crontab.conf есть строки вне разрешённого upstream-контракта. Автоматическое применение заблокировано.',
  'at least one managed scheduler task must remain enabled': 'Хотя бы одна обычная задача Antiscan должна оставаться включённой.',
  'cron expression must contain exactly five fields': 'Cron-выражение должно состоять ровно из пяти полей.',
  'cron expression contains characters unsupported by upstream': 'Cron-выражение содержит символы, которые upstream Antiscan не принимает.',
  'retry_load_geo is managed automatically by upstream Antiscan and was preserved unchanged.': 'Автоматическая задача retry_load_geo сохранена без изменений.',
  'scheduler verification failed; previous scheduler state was restored': 'Проверка расписания не пройдена; предыдущее состояние восстановлено.',
  'invalid flush request': 'Некорректный запрос штатной очистки Antiscan.',
  'flush target must be candidates, ips, subnets, custom_whitelist, custom_blacklist, custom_exclude, geo, ndm_lockout, honeypot or all': 'Выбрана неподдерживаемая цель очистки Antiscan.',
  'flush confirmation does not match target': 'Подтверждение очистки не соответствует выбранной цели.',
  'Antiscan must be running before flush': 'Для штатной очистки Antiscan должен быть запущен.',
  'flush blocked because active custom whitelist with SAVE_IPSETS=0 could lock out access': 'Очистка активного пользовательского whitelist заблокирована: при SAVE_IPSETS=0 пустой whitelist может перекрыть доступ. Сначала отключите whitelist в настройках.',
  'flush blocked because active Geo whitelist with SAVE_IPSETS=0 could lock out access': 'Очистка активного Geo whitelist заблокирована: при SAVE_IPSETS=0 пустой whitelist может перекрыть доступ. Сначала отключите Geo whitelist в настройках.',
  'Dynamic protection sets can repopulate while Antiscan keeps running.': 'Динамические наборы могут снова наполниться, пока Antiscan продолжает работать.',
  'Keenetic lockout entries can be imported again by read_ndm_ipsets.': 'Блокировки Keenetic могут снова импортироваться задачей read_ndm_ipsets.',
  'This flush clears the upstream source file, not only the runtime ipset.': 'Эта очистка стирает штатный исходный файл списка, а не только runtime ipset.',
  'Custom exclusion entries will be erased from the source file; removed exceptions may expose addresses to blocking rules.': 'Записи пользовательских исключений будут удалены из исходного файла; адреса могут снова попасть под блокирующие правила.',
  'Geo flush removes downloaded Geo files and empties all three Geo runtime sets.': 'Geo-очистка удаляет скачанные Geo-файлы и очищает все три Geo runtime-набора.',
  'The upstream all flush does not clear custom lists or ascn_geo_exclude.': 'Штатный flush без цели не очищает пользовательские списки и ascn_geo_exclude.',
  'Geo files are removed because upstream all flush includes geo blacklist and whitelist sets.': 'Geo-файлы удаляются, потому что штатный общий flush включает Geo blacklist и whitelist.',
  'Active custom whitelist protection will be removed by upstream; repopulate the list or change mode before restart.': 'Upstream снимет активную защиту custom whitelist. До перезапуска заполните список заново или смените режим.',
  'Active Geo whitelist protection will be removed by upstream; reload Geo or change mode before restart.': 'Upstream снимет активную защиту Geo whitelist. До перезапуска загрузите Geo заново или смените режим.',
  'Selected Antiscan sets were already empty; upstream flush completed and verification passed.': 'Выбранные наборы уже были пусты; штатная очистка выполнена и проверена.',
  'upstream Antiscan flush command failed': 'Штатная команда очистки Antiscan завершилась ошибкой.',
  'Antiscan stopped during flush': 'Во время очистки Antiscan неожиданно остановился.',
  'upstream flush returned while an Antiscan reload lock is still present': 'Очистка завершилась, но Antiscan всё ещё держит lock-файл.'
};

const messageRules = [
  [/^line (\d+): expected KEY="VALUE"$/, (m) => `Строка ${m[1]}: ожидается формат KEY="VALUE".`],
  [/^line (\d+): invalid Antiscan config syntax$/, (m) => `Строка ${m[1]}: некорректный синтаксис конфигурации Antiscan.`],
  [/^line (\d+): unsupported config value$/, (m) => `Строка ${m[1]}: неподдерживаемое значение конфигурации.`],
  [/^line (\d+): unsupported Antiscan config key (.+)$/, (m) => `Строка ${m[1]}: неподдерживаемый параметр Antiscan ${m[2]}.`],
  [/^line (\d+): duplicate Antiscan config key (.+)$/, (m) => `Строка ${m[1]}: параметр ${m[2]} указан повторно.`],
  [/^(.+) must be between (\d+) and (\d+)$/, (m) => `${m[1]} должен быть в диапазоне ${m[2]}–${m[3]}.`],
  [/^(.+) ports must be between 1 and 65535$/, (m) => `Порты ${m[1]} должны быть в диапазоне 1–65535.`],
  [/^(.+) supports at most 8 country codes$/, (m) => `${m[1]} поддерживает не более 8 кодов стран.`],
  [/^(.+) contains invalid country code "(.+)"$/, (m) => `${m[1]} содержит некорректный код страны ${m[2]}.`],
  [/^(.+) does not exist$/, (m) => `Набор ${m[1]} отсутствует.`],
  [/^confirm must equal (ADD|DELETE|CLEAR|RELOAD)$/, (m) => `Для этой операции требуется подтверждение ${m[1]}.`],
  [/^Ignored (\d+) invalid custom-list lines\.$/, (m) => `Пропущено некорректных строк исходного списка: ${m[1]}.`],
  [/^runtime list state could not be verified: (.*)$/, (m) => `Не удалось проверить runtime-состояние списка: ${m[1]}`],
  [/^custom-list verification failed: (.*)$/, (m) => `Проверка пользовательского списка не пройдена: ${m[1]}`],
  [/^upstream (start|stop|reload|restart) failed: (.*)$/, (m) => `Штатная команда Antiscan ${m[1]} завершилась ошибкой: ${m[2]}`],
  [/^upstream (.+) failed: (.*)$/, (m) => `Штатная операция Antiscan ${m[1]} завершилась ошибкой: ${m[2]}`],
  [/^start verification failed: \/tmp\/ascn\.run is absent$/, () => 'Проверка запуска не пройдена: /tmp/ascn.run не появился.'],
  [/^stop verification failed: \/tmp\/ascn\.run is still present$/, () => 'Проверка остановки не пройдена: /tmp/ascn.run всё ещё существует.'],
  [/^reload verification failed: Antiscan stopped during reload$/, () => 'Проверка перечитывания не пройдена: Antiscan остановился во время операции.'],
  [/^restart verification failed: \/tmp\/ascn\.run is absent$/, () => 'Проверка перезапуска не пройдена: /tmp/ascn.run не появился.']
];

function localizeMessage(value) {
  const text = String(value ?? '').trim();
  if (!text) return '';
  if (exactMessageTranslations[text]) return exactMessageTranslations[text];
  for (const [pattern, formatter] of messageRules) {
    const match = text.match(pattern);
    if (match) return formatter(match);
  }
  if (/[А-Яа-яЁё]/.test(text)) return text;
  return `Техническая причина: ${text}`;
}

function localizeMessages(values) {
  return (values || []).map(localizeMessage).filter(Boolean);
}

const reasonLabels = {
  'custom-exclude': 'Пользовательское исключение',
  'geo-exclude': 'Geo-исключение',
  'custom-blacklist': 'Пользовательский чёрный список',
  'custom-whitelist': 'Пользовательский белый список',
  'custom-whitelist-miss': 'Нет в пользовательском белом списке',
  'geo-blacklist': 'Geo-чёрный список',
  'geo-whitelist': 'Geo-белый список',
  'geo-whitelist-miss': 'Нет в Geo-белом списке',
  'ndm-lockout': 'Блокировка Keenetic',
  honeypot: 'Ловушка',
  'distributed-subnet': 'Блокировка подсети /24',
  'direct-ip': 'Прямая блокировка IP',
  candidate: 'Кандидат /24',
  'candidate-only': 'Кандидат /24',
  'no-active-set-match': 'Совпадений нет',
  unknown: 'Причина не определена',
  'invalid-ip': 'Некорректный IPv4-адрес'
};

function reasonLabel(value) {
  return reasonLabels[value] || value || 'Причина не определена';
}

function evidenceSummary(item) {
  return {
    'custom-exclude': 'Адрес найден в пользовательских исключениях. Эти правила проверяются раньше блокирующих.',
    'geo-exclude': 'Адрес попал в Geo-исключение и не должен блокироваться последующими правилами.',
    'custom-blacklist': 'Адрес найден в активном пользовательском чёрном списке.',
    'custom-whitelist': 'Адрес присутствует в активном пользовательском белом списке.',
    'custom-whitelist-miss': 'Адрес отсутствует в активном пользовательском белом списке.',
    'geo-blacklist': 'Адрес относится к подсети из активного Geo-чёрного списка.',
    'geo-whitelist': 'Адрес относится к разрешённой подсети активного Geo-белого списка.',
    'geo-whitelist-miss': 'Адрес отсутствует в разрешённых подсетях активного Geo-белого списка.',
    'ndm-lockout': 'Адрес импортирован из политики блокировок Keenetic.',
    honeypot: 'Адрес находится в наборе блокировок ловушки Antiscan.',
    'distributed-subnet': 'Адрес входит в /24, заблокированную после накопления разных IP-кандидатов.',
    'direct-ip': 'Адрес находится в прямом наборе блокировок. Antiscan не сохраняет, какой именно механизм был точным триггером.',
    candidate: 'Адрес является кандидатом для анализа /24, но сам этот набор его не блокирует.'
  }[item?.kind] || localizeMessage(item?.summary || '');
}


export { fmtDuration, boolText, fmtBytes, fmtAuditTime, localizeMessage, localizeMessages, reasonLabel, evidenceSummary };
