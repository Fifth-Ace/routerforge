const $ = (id) => document.getElementById(id);

const routerForgeParams = new URLSearchParams(window.location.search);
let routerForgeResizeObserver = null;
let routerForgeHeightFrame = 0;
let routerForgeHostObserver = null;

const routerForgeHostTokens = [
  '--rf-bg',
  '--rf-surface',
  '--rf-surface-2',
  '--rf-hover',
  '--rf-text',
  '--rf-muted',
  '--rf-border',
  '--rf-border-strong',
  '--rf-accent',
  '--rf-accent-soft',
  '--rf-accent-hover',
  '--rf-accent-border',
  '--rf-radius-panel',
  '--rf-radius-control',
  '--rf-radius-card',
  '--rf-panel-head-h',
  '--rf-control-gap',
  '--ui-body',
  '--ui-small',
  '--ui-xs',
  '--ui-micro',
  '--ui-panel-title',
  '--rf-type-metric'
];

function syncRouterForgeHostTheme() {
  if (window.parent === window) return false;
  try {
    const root = document.documentElement;
    const hostRoot = window.parent.document.documentElement;
    const hostStyle = window.parent.getComputedStyle(hostRoot);
    routerForgeHostTokens.forEach((token) => {
      const value = hostStyle.getPropertyValue(token).trim();
      if (value) root.style.setProperty(token, value);
    });
    ['theme', 'density', 'radius', 'uiScale'].forEach((key) => {
      const value = hostRoot.dataset[key];
      if (value) root.dataset[key] = value;
    });
    return true;
  } catch (_) {
    return false;
  }
}

const routerForgeThemes = {
  forge: {
    background: '#0b0d10', text: '#f5f7fa', surface: '#12151a', surface2: '#171b21',
    hover: '#1d2229', muted: '#8d98a4', border: '#29313a', borderStrong: '#36414d'
  },
  midnight: {
    background: '#08111b', text: '#edf5ff', surface: '#0f1824', surface2: '#152131',
    hover: '#1b2a3c', muted: '#8ca0b5', border: '#25384a', borderStrong: '#34516a'
  },
  graphite: {
    background: '#101113', text: '#f1f2f4', surface: '#17191c', surface2: '#1d2024',
    hover: '#25292e', muted: '#9299a2', border: '#30353c', borderStrong: '#414850'
  }
};

function routerForgeHex(value, fallback) {
  const normalized = String(value || '').trim().toLowerCase();
  return /^#[0-9a-f]{6}$/i.test(normalized) ? normalized : fallback;
}

function routerForgeMix(hex1, hex2, t) {
  const a = routerForgeHex(hex1, '#000000').slice(1);
  const b = routerForgeHex(hex2, '#ffffff').slice(1);
  const part = (value) => parseInt(value, 16);
  return '#' + [0, 2, 4].map((i) =>
    Math.round(
      part(a.slice(i, i + 2)) * (1 - t) +
      part(b.slice(i, i + 2)) * t
    ).toString(16).padStart(2, '0')
  ).join('');
}

function routerForgeLuminance(hex) {
  const value = routerForgeHex(hex, '#000000').slice(1);
  return [0, 2, 4].map((i) => parseInt(value.slice(i, i + 2), 16))
    .reduce((sum, channel, index) => sum + channel * [.2126, .7152, .0722][index], 0) / 255;
}

function routerForgeCustomTheme(background, text) {
  const light = routerForgeLuminance(background) > .55;
  return {
    background,
    text,
    surface: routerForgeMix(background, text, light ? .045 : .055),
    surface2: routerForgeMix(background, text, light ? .09 : .105),
    hover: routerForgeMix(background, text, light ? .13 : .15),
    muted: routerForgeMix(text, background, .48),
    border: routerForgeMix(background, text, light ? .18 : .19),
    borderStrong: routerForgeMix(background, text, light ? .27 : .29)
  };
}

function applyRouterForgeFrameSettings() {
  const root = document.documentElement;
  const theme = routerForgeParams.get('theme') || 'forge';
  const background = routerForgeHex(routerForgeParams.get('background'), '#0b0d10');
  const text = routerForgeHex(routerForgeParams.get('text'), '#f5f7fa');
  const accent = routerForgeHex(routerForgeParams.get('accent'), '#38bdf8');
  const palette = theme === 'custom'
    ? routerForgeCustomTheme(background, text)
    : (routerForgeThemes[theme] || routerForgeThemes.forge);

  root.dataset.theme = theme;
  root.style.setProperty('--rf-bg', palette.background);
  root.style.setProperty('--rf-surface', palette.surface);
  root.style.setProperty('--rf-surface-2', palette.surface2);
  root.style.setProperty('--rf-hover', palette.hover);
  root.style.setProperty('--rf-text', palette.text);
  root.style.setProperty('--rf-muted', palette.muted);
  root.style.setProperty('--rf-border', palette.border);
  root.style.setProperty('--rf-border-strong', palette.borderStrong);
  root.style.setProperty('--rf-accent', accent);

  const density = routerForgeParams.get('density');
  if (density === 'compact' || density === 'normal' || density === 'comfortable') {
    root.dataset.density = density;
  }

  const radius = routerForgeParams.get('radius');
  if (radius === 'sharp' || radius === 'default' || radius === 'soft') {
    root.dataset.radius = radius;
    root.style.setProperty('--rf-radius', radius === 'sharp' ? '2px' : radius === 'soft' ? '12px' : '8px');
  }

  syncRouterForgeHostTheme();
}

function reportRouterForgeModuleHeight() {
  if (window.parent === window) return;
  if (routerForgeHeightFrame) cancelAnimationFrame(routerForgeHeightFrame);
  routerForgeHeightFrame = requestAnimationFrame(() => {
    const workspace = document.querySelector('.as-page');
    if (!workspace) return;
    const height = Math.ceil(Math.max(workspace.scrollHeight, workspace.getBoundingClientRect().height, 360) + 2);
    window.parent.postMessage({
      type: 'routerforge-module-height',
      moduleId: 'antiscan-manager',
      height
    }, window.location.origin);
  });
}

function startRouterForgeBridge() {
  applyRouterForgeFrameSettings();
  reportRouterForgeModuleHeight();

  if (window.parent !== window && 'MutationObserver' in window) {
    try {
      const hostRoot = window.parent.document.documentElement;
      routerForgeHostObserver = new MutationObserver(() => syncRouterForgeHostTheme());
      routerForgeHostObserver.observe(hostRoot, {
        attributes: true,
        attributeFilter: ['style', 'class', 'data-theme', 'data-density', 'data-radius', 'data-ui-scale']
      });
    } catch (_) {
      routerForgeHostObserver = null;
    }
  }

  const workspace = document.querySelector('.as-page');
  if (workspace && 'ResizeObserver' in window) {
    routerForgeResizeObserver = new ResizeObserver(reportRouterForgeModuleHeight);
    routerForgeResizeObserver.observe(workspace);
  }
  window.addEventListener('resize', reportRouterForgeModuleHeight);
}

const setDescriptions = {
  ascn_candidates: 'Кандидаты для распределённой /24-защиты',
  ascn_ips: 'Прямые IP-блокировки',
  ascn_subnets: 'Заблокированные подсети /24',
  ascn_custom_exclude: 'Пользовательские исключения',
  ascn_custom_blacklist: 'Пользовательский чёрный список',
  ascn_custom_whitelist: 'Пользовательский белый список',
  ascn_geo_blacklist: 'Geo-чёрный список',
  ascn_geo_whitelist: 'Geo-белый список',
  ascn_geo_exclude: 'Geo-исключения',
  ascn_ndm_lockout: 'Блокировки Keenetic',
  ascn_honeypot: 'Ловушка'
};

const browserState = {
  blocked: null,
  lists: null,
  geo: null,
  customSource: null
};

let snapshot = null;
let lifecycleBusy = false;
let operationBusy = false;
let customListBusy = false;
let configBusy = false;
let configDirty = false;
let configBaseline = '';
let configBaseSHA = '';
let historyLoaded = false;
let diagnosticsLoaded = false;
let compatibilityLoaded = false;
let rciTokenLoaded = false;
let rciTokenBusy = false;
let schedulerLoaded = false;
let schedulerBusy = false;
let schedulerSnapshot = null;
let schedulerBaseline = '';
let schedulerDirty = false;
let flushPreviewLoaded = false;
let flushBusy = false;
let flushPreview = null;

function api(path) {
  return fetch(`../${path}`, {
    cache: 'no-store',
    credentials: 'same-origin',
    headers: { Accept: 'application/json' }
  }).then(async (response) => {
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(payload.error || `${path}: HTTP ${response.status}`);
      error.payload = payload;
      throw error;
    }
    return payload;
  });
}

function mutate(path, body) {
  return fetch(`../${path}`, {
    method: 'POST',
    cache: 'no-store',
    credentials: 'same-origin',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  }).then(async (response) => {
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(payload.error || `${path}: HTTP ${response.status}`);
      error.payload = payload;
      throw error;
    }
    return payload;
  });
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>'"]/g, (char) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
  })[char]);
}

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

function setByName(name) {
  return (snapshot?.ipsets || []).find((item) => item.name === name);
}

function countFor(name) {
  const item = setByName(name);
  if (!item?.exists || !item?.count_known) return '—';
  return String(item.count ?? 0);
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
  'IPSETS_DIRECTORY must not be inside /opt/etc': 'IPSETS_DIRECTORY нельзя размещать внутри /opt/etc.',
  'IPSETS_DIRECTORY must resolve to an existing directory': 'IPSETS_DIRECTORY должен указывать на существующий каталог.',
  'SAVE_IPSETS is enabled but IPSETS_DIRECTORY is empty': 'SAVE_IPSETS включён, но IPSETS_DIRECTORY не задан.',
  'IPSETS_DIRECTORY is unavailable': 'Каталог IPSETS_DIRECTORY недоступен.',
  'invalid custom-list request': 'Некорректный запрос изменения пользовательского списка.',
  'custom list action must be add, delete, clear or reload': 'Допустимы только добавление, удаление, очистка или перечитывание пользовательского списка.',
  'list must be blacklist, whitelist or exclude': 'Можно выбрать только чёрный список, белый список или исключения.',
  'reload does not accept an entry': 'Для перечитывания списка адрес указывать не нужно.',
  'clear does not accept an entry': 'Для очистки списка адрес указывать не нужно.',
  'configured custom list cannot be emptied; disable it in ascn.conf before deleting the final entry': 'Нельзя удалить последнюю запись включённого списка. Сначала отключите этот список в ascn.conf.',
  'configured custom list cannot be cleared; disable it in ascn.conf before clearing': 'Нельзя очистить включённый список. Сначала отключите его в ascn.conf.',
  'custom list reload requires at least one valid entry': 'Активный список нельзя перечитать без единой корректной записи.',
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

function setOperationalExplanation(name) {
  return {
    ascn_candidates: 'Кандидат сам по себе не блокирует IP; несколько адресов одной /24 могут позднее привести к блокировке всей подсети.',
    ascn_ips: 'Прямая блокировка IP. Antiscan не сохраняет точную причину срабатывания внутри этого набора.',
    ascn_subnets: 'Блокируется вся /24 после накопления разных IP-кандидатов. Это особенно важно для мобильных и динамических пулов.',
    ascn_honeypot: 'IP заблокирован после обращения к одному из портов ловушки HONEYPOT_PORTS.',
    ascn_ndm_lockout: 'Запись импортирована из политики блокировок Keenetic; прямое снятие такой блокировки здесь намеренно отключено.',
    ascn_custom_exclude: 'Пользовательское исключение имеет приоритет перед блокирующими правилами.',
    ascn_custom_blacklist: 'Активный пользовательский чёрный список при CUSTOM_LISTS_BLOCK_MODE=blacklist.',
    ascn_custom_whitelist: 'В режиме whitelist отсутствие адреса в этом наборе означает блокировку.',
    ascn_geo_blacklist: 'Подсети стран из активного Geo-чёрного списка.',
    ascn_geo_whitelist: 'В режиме Geo whitelist отсутствие подсети в разрешённом наборе означает блокировку.',
    ascn_geo_exclude: 'Geo-исключение имеет приоритет перед блокирующими правилами.'
  }[name] || 'Запись активного набора Antiscan.';
}

function reasonExplanation(result) {
  const descriptions = {
    'custom-exclude': 'IP найден в пользовательских исключениях. Antiscan пропускает такой адрес до проверки блокирующих правил.',
    'geo-exclude': 'IP попал в Geo-исключение и не должен блокироваться последующими правилами.',
    'custom-blacklist': 'IP найден в активном пользовательском чёрном списке.',
    'custom-whitelist-miss': 'Включён режим белого списка, но IP отсутствует среди разрешённых адресов.',
    'geo-blacklist': 'IP относится к подсети страны из активного Geo-чёрного списка.',
    'geo-whitelist-miss': 'Включён Geo-белый список, но IP не относится к разрешённым подсетям.',
    'ndm-lockout': 'IP импортирован из политики блокировок Keenetic.',
    honeypot: 'IP находится в наборе блокировок ловушки после обращения к порту-приманке.',
    'distributed-subnet': 'Заблокирована вся /24: Antiscan накопил заданное количество разных IP-кандидатов из одной подсети.',
    'direct-ip': 'IP находится в прямом наборе блокировок. Antiscan не сохраняет, какой именно механизм был точным триггером.',
    'candidate-only': 'IP пока только кандидат для анализа /24 и этим набором сам по себе не блокируется.',
    'no-active-set-match': 'Совпадений с активными блокирующими наборами не найдено.'
  };
  return descriptions[result?.reason] || '';
}

function configNumber(id) {
  const raw = $(id).value.trim();
  return raw === '' ? 0 : Number(raw);
}

function currentConfigFormPayload() {
  return {
    isp_interfaces: $('cfgIspInterfaces').value.trim(),
    ports: $('cfgPorts').value.trim(),
    forwarded_ports: $('cfgForwardedPorts').value.trim(),
    enable_honeypot: $('cfgEnableHoneypot').checked,
    honeypot_ports: $('cfgHoneypotPorts').value.trim(),
    honeypot_bantime_seconds: configNumber('cfgHoneypotBanTime'),
    enable_ips_ban: $('cfgEnableIpsBan').checked,
    rules_mask: $('cfgRulesMask').value.trim(),
    recent_connections_time_seconds: configNumber('cfgRecentTime'),
    recent_connections_hitcount: configNumber('cfgRecentHitcount'),
    recent_connections_limit: configNumber('cfgRecentLimit'),
    recent_connections_bantime_seconds: configNumber('cfgRecentBanTime'),
    different_ip_candidates_storage_seconds: configNumber('cfgCandidateStorage'),
    different_ip_threshold: configNumber('cfgDifferentThreshold'),
    subnets_bantime_seconds: configNumber('cfgSubnetBanTime'),
    ipsets_directory: $('cfgIPSetsDirectory').value.trim(),
    save_ipsets: $('cfgSaveIPSets').checked,
    save_on_exit: $('cfgSaveOnExit').checked,
    use_custom_exclude_list: $('cfgUseCustomExclude').checked,
    custom_lists_block_mode: $('cfgCustomMode').value,
    geoblock_mode: $('cfgGeoMode').value,
    geoblock_countries: $('cfgGeoCountries').value.trim(),
    geo_exclude_countries: $('cfgGeoExcludeCountries').value.trim(),
    read_ndm_lockout_ipsets: $('cfgReadNDM').checked,
    lockout_ipset_bantime_seconds: configNumber('cfgLockoutBanTime')
  };
}

function configPayloadFromSnapshot(cfg) {
  return {
    isp_interfaces: (cfg.isp_interfaces || []).join(' '),
    ports: (cfg.ports || []).join(','),
    forwarded_ports: (cfg.forwarded_ports || []).join(','),
    enable_honeypot: Boolean(cfg.enable_honeypot),
    honeypot_ports: (cfg.honeypot_ports || []).join(','),
    honeypot_bantime_seconds: Number(cfg.honeypot_bantime_seconds || 0),
    enable_ips_ban: Boolean(cfg.enable_ips_ban),
    rules_mask: cfg.rules_mask || '255.255.255.255',
    recent_connections_time_seconds: Number(cfg.recent_connections_time_seconds || 0),
    recent_connections_hitcount: Number(cfg.recent_connections_hitcount || 0),
    recent_connections_limit: Number(cfg.recent_connections_limit || 0),
    recent_connections_bantime_seconds: Number(cfg.recent_connections_bantime_seconds || 0),
    different_ip_candidates_storage_seconds: Number(cfg.different_ip_candidates_storage_seconds || 0),
    different_ip_threshold: Number(cfg.different_ip_threshold || 0),
    subnets_bantime_seconds: Number(cfg.subnets_bantime_seconds || 0),
    ipsets_directory: cfg.ipsets_directory || '',
    save_ipsets: Boolean(cfg.save_ipsets),
    save_on_exit: Boolean(cfg.save_on_exit),
    use_custom_exclude_list: Boolean(cfg.use_custom_exclude_list),
    custom_lists_block_mode: cfg.custom_lists_block_mode || '0',
    geoblock_mode: cfg.geoblock_mode || '0',
    geoblock_countries: (cfg.geoblock_countries || []).join(' '),
    geo_exclude_countries: (cfg.geo_exclude_countries || []).join(' '),
    read_ndm_lockout_ipsets: Boolean(cfg.read_ndm_lockout_ipsets),
    lockout_ipset_bantime_seconds: Number(cfg.lockout_ipset_bantime_seconds || 0)
  };
}

function setConfigFormPayload(cfg) {
  $('cfgIspInterfaces').value = cfg.isp_interfaces;
  $('cfgPorts').value = cfg.ports;
  $('cfgForwardedPorts').value = cfg.forwarded_ports;
  $('cfgEnableHoneypot').checked = cfg.enable_honeypot;
  $('cfgHoneypotPorts').value = cfg.honeypot_ports;
  $('cfgHoneypotBanTime').value = cfg.honeypot_bantime_seconds;
  $('cfgEnableIpsBan').checked = cfg.enable_ips_ban;
  $('cfgRulesMask').value = cfg.rules_mask;
  $('cfgRecentTime').value = cfg.recent_connections_time_seconds;
  $('cfgRecentHitcount').value = cfg.recent_connections_hitcount;
  $('cfgRecentLimit').value = cfg.recent_connections_limit;
  $('cfgRecentBanTime').value = cfg.recent_connections_bantime_seconds;
  $('cfgCandidateStorage').value = cfg.different_ip_candidates_storage_seconds;
  $('cfgDifferentThreshold').value = cfg.different_ip_threshold;
  $('cfgSubnetBanTime').value = cfg.subnets_bantime_seconds;
  $('cfgIPSetsDirectory').value = cfg.ipsets_directory;
  $('cfgSaveIPSets').checked = cfg.save_ipsets;
  $('cfgSaveOnExit').checked = cfg.save_on_exit;
  $('cfgUseCustomExclude').checked = cfg.use_custom_exclude_list;
  $('cfgCustomMode').value = cfg.custom_lists_block_mode;
  $('cfgGeoMode').value = cfg.geoblock_mode;
  $('cfgGeoCountries').value = cfg.geoblock_countries;
  $('cfgGeoExcludeCountries').value = cfg.geo_exclude_countries;
  $('cfgReadNDM').checked = cfg.read_ndm_lockout_ipsets;
  $('cfgLockoutBanTime').value = cfg.lockout_ipset_bantime_seconds;
}

function populateConfigEditor(force = false) {
  const form = $('configForm');
  if (!form) return;
  if (!snapshot?.detected || !snapshot?.config) {
    $('configState').className = 'state neutral';
    $('configState').textContent = 'НЕДОСТУПНО';
    $('configHint').textContent = 'Antiscan или ascn.conf не обнаружен.';
    $('applyConfig').disabled = true;
    $('resetConfig').disabled = true;
    return;
  }

  if (!configDirty || force) {
    const payload = configPayloadFromSnapshot(snapshot.config);
    setConfigFormPayload(payload);
    configBaseline = JSON.stringify(payload);
    configBaseSHA = snapshot.config_sha256 || '';
    configDirty = false;
  }
  updateConfigEditorState();
}

function updateConfigEditorState() {
  if (!$('configForm')) return;
  const current = currentConfigFormPayload();
  configDirty = Boolean(configBaseline) && JSON.stringify(current) !== configBaseline;
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const stale = Boolean(configDirty && configBaseSHA && snapshot?.config_sha256 && snapshot.config_sha256 !== configBaseSHA);

  const state = $('configState');
  const hint = $('configHint');
  state.className = 'state';
  if (configBusy) {
    state.classList.add('info');
    state.textContent = 'ПРИМЕНЕНИЕ…';
    hint.textContent = 'Создаём резервную копию → атомарно записываем → перечитываем Antiscan → проверяем результат. При ошибке выполняется откат.';
  } else if (stale) {
    state.classList.add('bad');
    state.textContent = 'УСТАРЕЛО';
    hint.textContent = 'ascn.conf изменился после открытия формы. Сбросьте форму к свежим данным перед применением.';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
    hint.textContent = 'Antiscan сейчас перечитывает конфигурацию или Geo-данные. Применение временно заблокировано.';
  } else if (configDirty) {
    state.classList.add('warn');
    state.textContent = 'ИЗМЕНЕНО';
    hint.textContent = snapshot?.running
      ? 'Изменения будут безопасно применены через штатное перечитывание S99ascn.'
      : 'Antiscan остановлен: файл будет сохранён и проверен, а настройки вступят в силу при следующем запуске.';
  } else {
    state.classList.add('good');
    state.textContent = 'СИНХРОНИЗИРОВАНО';
    hint.textContent = snapshot?.running ? 'Форма соответствует активному ascn.conf.' : 'Форма соответствует сохранённому ascn.conf.';
  }

  $('configHash').textContent = configBaseSHA ? `SHA256 ${configBaseSHA.slice(0, 16)}…` : 'SHA256 —';
  $('applyConfig').disabled = configBusy || !configDirty || !configBaseSHA || upstreamBusy || stale;
  $('resetConfig').disabled = configBusy || !configDirty;

  const mobileRisk = current.enable_ips_ban && current.different_ip_threshold > 0 &&
    current.different_ip_threshold <= 5 && current.different_ip_candidates_storage_seconds >= 86400;
  const risk = $('configMobileRisk');
  risk.className = `config-risk ${mobileRisk ? 'warn' : 'good'}`;
  risk.textContent = mobileRisk
    ? `⚠ /24: порог ${current.different_ip_threshold}, кандидаты хранятся ${fmtDuration(current.different_ip_candidates_storage_seconds)} — для мобильных пулов настройка может быть слишком агрессивной.`
    : '✓ Сочетание порога /24 и времени хранения кандидатов не попадает под встроенный профиль повышенного риска.';
}

async function applyConfigEditor() {
  if (!configDirty || !configBaseSHA) return;
  const current = currentConfigFormPayload();
  const warning = snapshot?.running
    ? 'Будет создана резервная копия, ascn.conf запишется атомарно, затем Antiscan штатно перечитает настройки. При ошибке RouterForge восстановит предыдущий файл и рабочее состояние.'
    : 'Antiscan остановлен. Будет создана резервная копия и атомарно сохранён ascn.conf; настройки вступят в силу при следующем запуске.';
  if (!window.confirm(`Применить изменения ascn.conf?\n\n${warning}`)) return;

  configBusy = true;
  updateConfigEditorState();
  try {
    const result = await mutate('config', { ...current, base_sha256: configBaseSHA, confirm: 'APPLY_CONFIG' });
    showMutationResult(result, false, 'config');
    configDirty = false;
    configBaseline = '';
    await loadStatus();
    populateConfigEditor(true);
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'config');
    await loadStatus();
  } finally {
    configBusy = false;
    updateConfigEditorState();
  }
}

function diagnosticStateView(state) {
  return {
    pass: ['good', 'ГОТОВО'],
    warn: ['warn', 'ВНИМАНИЕ'],
    fail: ['bad', 'ОШИБКА'],
    info: ['info', 'ИНФО']
  }[state] || ['neutral', 'НЕИЗВЕСТНО'];
}

function renderDiagnostics(payload) {
  const overall = diagnosticStateView(payload?.overall || 'info');
  const state = $('diagnosticsState');
  state.className = `state ${overall[0]}`;
  state.textContent = overall[1];

  const generated = payload?.generated_at ? fmtAuditTime(payload.generated_at) : '—';
  $('diagnosticsMeta').textContent = `Проверено: ${generated} · Antiscan ${payload?.installed_version || 'не определён'} · ${payload?.running ? 'работает' : 'остановлен'}`;

  const contract = payload?.contract || {};
  const rows = [
    ['Upstream', contract.repository || '—'],
    ['Зафиксированная версия', contract.version || '—'],
    ['Upstream SHA', contract.pinned_sha || '—'],
    ['Параметры ascn.conf', `${(contract.config_keys || []).length} / 25`],
    ['Известные ipset', `${(contract.ipsets || []).length} / 11`],
    ['CLI-команды', String((contract.commands || []).length)],
    ['Разрешённые cron-задачи', (contract.valid_tasks || []).join(' · ') || '—']
  ];
  $('diagnosticsContract').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');

  const checks = payload?.checks || [];
  const root = $('diagnosticsChecks');
  if (!checks.length) {
    root.className = 'diagnostics-list empty-state';
    root.textContent = 'Проверки не вернули данных.';
    return;
  }

  root.className = 'diagnostics-list';
  root.innerHTML = checks.map((check) => {
    const view = diagnosticStateView(check.state);
    const details = (check.details || []).map((detail) => `<small class="mono">${escapeHTML(detail)}</small>`).join('');
    return `
      <article class="diagnostic-row">
        <div class="diagnostic-main">
          <div class="diagnostic-title"><strong>${escapeHTML(check.label || check.id || 'Проверка')}</strong><span class="state ${view[0]}">${view[1]}</span></div>
          <p>${escapeHTML(check.summary || '—')}</p>
          ${details}
        </div>
      </article>`;
  }).join('');
}

async function loadDiagnostics() {
  const button = $('reloadDiagnostics');
  if (button) button.disabled = true;
  $('diagnosticsState').className = 'state info';
  $('diagnosticsState').textContent = 'ПРОВЕРКА…';
  try {
    const payload = await api('diagnostics');
    diagnosticsLoaded = true;
    renderDiagnostics(payload);
  } catch (error) {
    diagnosticsLoaded = true;
    $('diagnosticsState').className = 'state bad';
    $('diagnosticsState').textContent = 'ОШИБКА';
    $('diagnosticsMeta').textContent = localizeMessage(error.message || 'Не удалось выполнить диагностику Antiscan.');
    $('diagnosticsChecks').className = 'diagnostics-list empty-state bad-text';
    $('diagnosticsChecks').textContent = 'Диагностика недоступна.';
  } finally {
    if (button) button.disabled = false;
  }
}

function renderCompatibility(payload) {
  compatibilityLoaded = true;
  const state = $('compatibilityState');
  const meta = $('compatibilityMeta');
  const grid = $('compatibilityGrid');
  const warnings = $('compatibilityWarnings');
  if (!state || !meta || !grid || !warnings) return;

  const view = diagnosticStateView(payload?.state || 'info');
  state.className = `state ${view[0]}`;
  state.textContent = view[1];
  meta.textContent = payload?.summary || 'Состояние совместимости не определено.';

  const update = payload?.update || {};
  const firmware = payload?.firmware || {};
  const hook = payload?.hook || {};
  const packageVersion = payload?.package_version || 'не найден';
  const packageMatch = payload?.package_version
    ? (payload?.package_runtime_match ? 'совпадает' : 'расходится')
    : 'не проверено';
  const updateValue = update.available
    ? `${update.available_version || '—'} · ${update.available_channel || 'upstream'}`
    : (update.eligibility_known ? 'по текущему upstream cache нет' : 'не определено');
  const cacheValue = !update.cache_present
    ? 'нет'
    : `${update.cache_fresh ? 'свежий' : 'устарел'} · ${Number(update.cache_age_seconds || 0)} сек.`;
  const netfilterValue = !firmware.netfilter_known
    ? 'не определено'
    : (firmware.netfilter_present ? 'есть' : 'не найден');
  const hookValue = hook.present
    ? (hook.executable ? 'установлен · исполняем' : 'установлен · не исполняем')
    : 'не найден';

  const rows = [
    ['Runtime S99ascn', payload?.script_version || '—'],
    ['OPKG Antiscan', packageVersion],
    ['Пакет / runtime', packageMatch],
    ['Проверенный RouterForge', payload?.pinned_version || '—'],
    ['Pinned contract', payload?.contract_exact ? 'точное совпадение' : 'не совпадает'],
    ['Доступное обновление', updateValue],
    ['Upstream update cache', cacheValue],
    ['Прошивка Keenetic', firmware.version || 'не определена'],
    ['Netfilter', netfilterValue],
    ['Netfilter hook', hookValue],
    ['Кто обновляет Antiscan', payload?.update_owner === 'upstream-opkg' ? 'официальный upstream OPKG' : (payload?.update_owner || '—')]
  ];
  grid.innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');

  const messages = payload?.warnings || [];
  warnings.innerHTML = messages.length
    ? messages.map((item) => `<div class="warning-item"><span>⚠</span><p>${escapeHTML(String(item))}</p></div>`).join('')
    : '<div class="ok-note">Версия, package metadata, Netfilter и hook согласованы с проверенным контрактом RouterForge.</div>';
}

async function loadCompatibility() {
  const button = $('reloadCompatibility');
  if (button) button.disabled = true;
  $('compatibilityState').className = 'state info';
  $('compatibilityState').textContent = 'ПРОВЕРКА…';
  try {
    const payload = await api('compatibility');
    renderCompatibility(payload);
  } catch (error) {
    compatibilityLoaded = true;
    $('compatibilityState').className = 'state bad';
    $('compatibilityState').textContent = 'ОШИБКА';
    $('compatibilityMeta').textContent = localizeMessage(error.message || 'Не удалось проверить совместимость Antiscan.');
    $('compatibilityGrid').innerHTML = '';
    $('compatibilityWarnings').innerHTML = '';
  } finally {
    if (button) button.disabled = false;
  }
}

function rciAuthLabel(state) {
  return {
    required: 'требуется',
    not_required: 'не требуется',
    unknown: 'ещё не определено'
  }[state] || 'неизвестно';
}

function renderRCITokenStatus(payload) {
  rciTokenLoaded = true;
  const state = $('rciTokenState');
  const summary = $('rciTokenSummary');
  const meta = $('rciTokenMeta');
  const setButton = $('setRCIToken');
  const checkButton = $('checkRCIToken');
  const deleteButton = $('deleteRCIToken');
  if (!state || !summary || !meta) return;

  const complete = Boolean(payload?.complete);
  const supported = Boolean(payload?.supported);
  const authState = payload?.auth_state || 'unknown';

  state.className = 'state';
  if (!supported) {
    state.classList.add('neutral');
    state.textContent = 'НЕ ПОДДЕРЖИВАЕТСЯ';
    summary.textContent = 'Upstream отключил token-flow для этой версии прошивки.';
  } else if (authState === 'required' && !complete) {
    state.classList.add('bad');
    state.textContent = 'НУЖЕН ТОКЕН';
    summary.textContent = 'RCI требует токен, но комплект token/key Antiscan неполный.';
  } else if (complete) {
    state.classList.add('good');
    state.textContent = 'НАСТРОЕН';
    summary.textContent = authState === 'required'
      ? 'RCI требует токен; зашифрованный token/key комплект Antiscan присутствует.'
      : 'Зашифрованный token/key комплект Antiscan присутствует.';
  } else if (authState === 'not_required') {
    state.classList.add('info');
    state.textContent = 'НЕ ТРЕБУЕТСЯ';
    summary.textContent = 'Upstream cache сообщает, что этой прошивке RCI-токен сейчас не требуется.';
  } else {
    state.classList.add('warn');
    state.textContent = 'НЕ ОПРЕДЕЛЕНО';
    summary.textContent = 'Требование RCI ещё не закэшировано upstream.';
  }

  meta.textContent = `RCI: ${rciAuthLabel(authState)} · token ${payload?.token_present ? 'есть' : 'нет'} · key ${payload?.key_present ? 'есть' : 'нет'} · Antiscan ${payload?.running ? 'работает' : 'остановлен'}`;
  setButton.disabled = rciTokenBusy || !supported;
  checkButton.disabled = rciTokenBusy || !supported || !complete;
  deleteButton.disabled = rciTokenBusy || (!payload?.token_present && !payload?.key_present);
}

async function loadRCITokenStatus() {
  const button = $('reloadRCIToken');
  if (button) button.disabled = true;
  try {
    const payload = await api('rci-token');
    renderRCITokenStatus(payload);
  } catch (error) {
    rciTokenLoaded = true;
    $('rciTokenState').className = 'state bad';
    $('rciTokenState').textContent = 'ОШИБКА';
    $('rciTokenSummary').textContent = localizeMessage(error.message || 'Не удалось прочитать состояние RCI-токена.');
  } finally {
    if (button) button.disabled = false;
  }
}

async function performRCITokenAction(action) {
  const input = $('rciTokenInput');
  let token = '';
  let confirmValue = '';
  let prompt = '';

  if (action === 'set') {
    token = input.value.trim();
    if (!token) return;
    if (!/^[A-Za-z0-9]+$/.test(token)) {
      showMutationResult({ error: 'RCI token must contain only ASCII letters and digits' }, true, 'rci-token:set');
      return;
    }
    confirmValue = 'SET_TOKEN';
    prompt = 'Сохранить новый RCI-токен?\n\nТокен будет передан штатной команде Antiscan через stdin, проверен upstream и не попадёт в argv или журнал.';
  } else if (action === 'check') {
    confirmValue = 'CHECK_TOKEN';
    prompt = 'Проверить сохранённый RCI-токен через штатную команду Antiscan?';
  } else if (action === 'delete') {
    confirmValue = 'DELETE_TOKEN';
    prompt = 'Удалить RCI-токен Antiscan?\n\nЕсли прошивка требует RCI-аутентификацию, upstream может штатно остановить Antiscan после удаления.';
  } else {
    return;
  }

  if (!window.confirm(prompt)) return;

  const body = { action, confirm: confirmValue };
  if (action === 'set') body.token = token;
  if (input) input.value = '';
  token = '';

  rciTokenBusy = true;
  try {
    const result = await mutate('rci-token-action', body);
    showMutationResult(result, false, `rci-token:${action}`);
    await loadRCITokenStatus();
    await loadStatus();
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `rci-token:${action}`);
    await loadRCITokenStatus();
    await loadStatus();
  } finally {
    rciTokenBusy = false;
    if (rciTokenLoaded) {
      try {
        const payload = await api('rci-token');
        renderRCITokenStatus(payload);
      } catch (_) {
        // Previous error state is already visible.
      }
    }
  }
}
const schedulerTaskLabels = {
  read_candidates: ['Обработка кандидатов', 'Проверка накопленных IP-кандидатов и перенос /24 при достижении порога.'],
  read_ndm_ipsets: ['Импорт блокировок Keenetic', 'Перенос системных lockout IP из Keenetic в runtime Antiscan.'],
  save_ipsets: ['Сохранение ipset', 'Периодический экспорт runtime-наборов согласно SAVE_IPSETS.'],
  'update_ipsets geo': ['Обновление Geo', 'Плановое обновление списков подсетей настроенных стран.'],
  retry_load_geo: ['Повтор Geo после ошибки', 'Временная аварийная задача, которую создаёт и удаляет сам upstream Antiscan.']
};

function schedulerManagedPayload() {
  return Array.from(document.querySelectorAll('[data-scheduler-task]'))
    .filter((row) => row.dataset.automatic !== 'true')
    .map((row) => ({
      task: row.dataset.schedulerTask || '',
      enabled: Boolean(row.querySelector('[data-scheduler-enabled]')?.checked),
      schedule: (row.querySelector('[data-scheduler-expression]')?.value || '').trim().replace(/\s+/g, ' ')
    }));
}

function updateSchedulerState() {
  const apply = $('applyScheduler');
  const reset = $('resetScheduler');
  const state = $('schedulerState');
  if (!schedulerSnapshot || !apply || !reset || !state) return;

  const current = JSON.stringify(schedulerManagedPayload());
  schedulerDirty = Boolean(schedulerBaseline) && current !== schedulerBaseline;
  if (!schedulerSnapshot.valid) {
    state.className = 'state bad';
    state.textContent = 'ОШИБКА ФАЙЛА';
  } else if (schedulerBusy) {
    state.className = 'state info';
    state.textContent = 'ПРИМЕНЕНИЕ…';
  } else if (schedulerDirty) {
    state.className = 'state warn';
    state.textContent = 'ИЗМЕНЕНО';
  } else if (!schedulerSnapshot.synced) {
    state.className = 'state warn';
    state.textContent = 'НЕ СИНХРОНИЗИРОВАНО';
  } else {
    state.className = 'state good';
    state.textContent = 'СИНХРОНИЗИРОВАНО';
  }

  apply.disabled = schedulerBusy || !schedulerSnapshot.valid || (!schedulerDirty && schedulerSnapshot.synced);
  reset.disabled = schedulerBusy || !schedulerDirty;
}

function renderScheduler(payload, force = false) {
  schedulerLoaded = true;
  schedulerSnapshot = payload;
  const root = $('schedulerTaskRows');
  const meta = $('schedulerMeta');
  if (!root || !meta) return;

  const errors = payload?.errors || [];
  meta.textContent = `ascn_crontab.conf: ${payload?.task_count ?? 0} задач · active crontab: ${payload?.active_count ?? 0} · ${payload?.synced ? 'синхронизировано' : 'есть расхождение'} · SHA256 ${(payload?.source_sha256 || '—').slice(0, 16)}…`;
  if (errors.length) {
    root.className = 'scheduler-list';
    root.innerHTML = `<div class="notice bad">${errors.map((item) => escapeHTML(localizeMessage(item))).join(' · ')}</div>`;
    schedulerBaseline = '';
    schedulerDirty = false;
    updateSchedulerState();
    return;
  }

  root.className = 'scheduler-list';
  root.innerHTML = (payload?.tasks || []).map((item) => {
    const copy = schedulerTaskLabels[item.task] || [item.task, 'Штатная задача Antiscan.'];
    const automatic = Boolean(item.automatic);
    return `
      <article class="scheduler-row" data-scheduler-task="${escapeHTML(item.task)}" data-automatic="${automatic}">
        <label class="scheduler-toggle">
          <input data-scheduler-enabled type="checkbox" ${item.enabled ? 'checked' : ''} ${automatic ? 'disabled' : ''}>
          <span><strong>${escapeHTML(copy[0])}</strong><small class="mono">${escapeHTML(item.task)}</small></span>
        </label>
        <div class="scheduler-expression-wrap">
          <input data-scheduler-expression class="mono" value="${escapeHTML(item.schedule || '')}" spellcheck="false" ${automatic ? 'disabled' : ''} aria-label="Cron ${escapeHTML(item.task)}">
          <small>${escapeHTML(copy[1])}</small>
        </div>
        ${automatic ? '<span class="state info">АВТОМАТИЧЕСКИ</span>' : '<span class="state neutral">5 полей cron</span>'}
      </article>`;
  }).join('');

  if (force || !schedulerBaseline) {
    schedulerBaseline = JSON.stringify(schedulerManagedPayload());
    schedulerDirty = false;
  }
  updateSchedulerState();
}

async function loadScheduler() {
  const button = $('reloadScheduler');
  if (button) button.disabled = true;
  try {
    const payload = await api('schedule');
    schedulerBaseline = '';
    renderScheduler(payload, true);
  } catch (error) {
    schedulerLoaded = true;
    schedulerSnapshot = null;
    $('schedulerState').className = 'state bad';
    $('schedulerState').textContent = 'ОШИБКА';
    $('schedulerMeta').textContent = localizeMessage(error.message || 'Не удалось прочитать расписание Antiscan.');
    $('schedulerTaskRows').className = 'scheduler-list empty-state bad-text';
    $('schedulerTaskRows').textContent = 'Расписание недоступно.';
    $('applyScheduler').disabled = true;
    $('resetScheduler').disabled = true;
  } finally {
    if (button) button.disabled = false;
  }
}

async function applyScheduler() {
  if (!schedulerSnapshot?.source_sha256 || !schedulerSnapshot?.valid) return;
  const tasks = schedulerManagedPayload();
  if (!window.confirm('Применить расписание Antiscan?\n\nRouterForge атомарно обновит ascn_crontab.conf, вызовет штатный update_crontab и проверит активный crontab. При ошибке будет восстановлено предыдущее состояние.')) return;

  schedulerBusy = true;
  updateSchedulerState();
  try {
    const result = await mutate('schedule-action', {
      tasks,
      base_sha256: schedulerSnapshot.source_sha256,
      confirm: 'APPLY_SCHEDULE'
    });
    showMutationResult(result, false, 'schedule:apply');
    await loadScheduler();
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'schedule:apply');
    await loadScheduler();
  } finally {
    schedulerBusy = false;
    if (schedulerSnapshot) updateSchedulerState();
  }
}

const flushTargetLabels = {
  candidates: 'Кандидаты',
  ips: 'Прямые IP',
  subnets: 'Подсети /24',
  custom_whitelist: 'Пользовательский белый список',
  custom_blacklist: 'Пользовательский чёрный список',
  custom_exclude: 'Пользовательские исключения',
  geo: 'Все Geo-наборы',
  ndm_lockout: 'Блокировки Keenetic',
  honeypot: 'Ловушка',
  all: 'Upstream «все»'
};

function flushTargetLabel(target) {
  return flushTargetLabels[target] || target || 'Наборы Antiscan';
}

function renderFlushPreview(payload) {
  flushPreviewLoaded = true;
  flushPreview = payload;
  const state = $('flushState');
  const meta = $('flushMeta');
  const warnings = $('flushWarnings');
  const button = $('flushSelected');
  if (!state || !meta || !warnings || !button) return;

  state.className = `state ${payload?.allowed ? 'warn' : 'bad'}`;
  state.textContent = payload?.allowed ? 'ГОТОВО К ОЧИСТКЕ' : 'ЗАБЛОКИРОВАНО';
  const sets = (payload?.affected_sets || []).map((item) => {
    const count = item.count_known ? Number(item.count || 0) : '—';
    return `${item.name}: ${count}`;
  });
  meta.textContent = `${flushTargetLabel(payload?.target)} · записей сейчас ${Number(payload?.before_entries || 0)} · ${sets.join(' · ') || 'наборы не найдены'}`;

  const messages = [...localizeMessages(payload?.warnings || [])];
  if (payload?.destructive_file) messages.unshift('Операция затрагивает сохранённые файлы upstream, а не только runtime.');
  if (payload?.restart_required) messages.push('После этой очистки upstream требует восстановить данные и перезапустить Antiscan перед возвратом соответствующей защиты.');
  if (payload?.block_reason) messages.unshift(localizeMessage(payload.block_reason));
  warnings.innerHTML = messages.length
    ? messages.map((item) => `<div class="warning-item"><span>⚠</span><p>${escapeHTML(item)}</p></div>`).join('')
    : '<div class="empty-state">Дополнительных предупреждений для этой цели нет.</div>';
  button.disabled = flushBusy || !payload?.allowed;
}

async function loadFlushPreview() {
  const select = $('flushTarget');
  const reload = $('reloadFlushPreview');
  if (!select || !reload) return;
  reload.disabled = true;
  try {
    const payload = await api(`flush-preview?target=${encodeURIComponent(select.value)}`);
    renderFlushPreview(payload);
  } catch (error) {
    flushPreviewLoaded = true;
    flushPreview = null;
    $('flushState').className = 'state bad';
    $('flushState').textContent = 'ОШИБКА';
    $('flushMeta').textContent = localizeMessage(error.message || 'Не удалось построить preview очистки.');
    $('flushWarnings').innerHTML = '';
    $('flushSelected').disabled = true;
  } finally {
    reload.disabled = false;
  }
}

async function performFlush() {
  const target = $('flushTarget')?.value || '';
  if (!flushPreview || flushPreview.target !== target || !flushPreview.allowed) return;
  const warnings = localizeMessages(flushPreview.warnings || []);
  const detail = [
    `Цель: ${flushTargetLabel(target)}`,
    `Записей сейчас: ${Number(flushPreview.before_entries || 0)}`,
    flushPreview.destructive_file ? 'Будут изменены/удалены сохранённые файлы upstream.' : '',
    ...warnings
  ].filter(Boolean).join('\n');
  if (!window.confirm(`Выполнить штатный Antiscan flush?\n\n${detail}\n\nОперация необратимо очищает выбранные данные.`)) return;

  flushBusy = true;
  $('flushSelected').disabled = true;
  try {
    const result = await mutate('flush', {
      target,
      confirm: flushPreview.confirm
    });
    showMutationResult(result, false, `flush:${target}`);
    await loadFlushPreview();
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
    if (browserState.customSource) await loadCustomList();
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `flush:${target}`);
    await loadFlushPreview();
  } finally {
    flushBusy = false;
    if (flushPreview) $('flushSelected').disabled = !flushPreview.allowed;
  }
}

function activateTab(name) {
  document.documentElement.dataset.section = name;
  document.querySelectorAll('.tab').forEach((button) => {
    button.classList.toggle('active', button.dataset.tab === name);
  });
  document.querySelectorAll('.tab-page').forEach((page) => {
    const pageName = page.dataset.page;
    const grouped =
      (name === 'overview' && pageName === 'protection') ||
      (name === 'blocked' && pageName === 'inspect') ||
      (name === 'geo' && pageName === 'lists');
    page.classList.toggle('active', pageName === name || grouped);
  });
}

function renderState() {
  const state = $('targetState');
  state.className = 'state';
  if (!snapshot?.detected) {
    state.classList.add('neutral');
    state.textContent = 'НЕ ОБНАРУЖЕН';
  } else if (snapshot.running) {
    state.classList.add('good');
    state.textContent = 'РАБОТАЕТ';
  } else {
    state.classList.add('warn');
    state.textContent = 'ОСТАНОВЛЕН';
  }

  $('version').textContent = snapshot?.version || '—';
  $('blockedIps').textContent = countFor('ascn_ips');
  $('blockedSubnets').textContent = countFor('ascn_subnets');
  $('candidateCount').textContent = countFor('ascn_candidates');
  $('ipsetBinary').textContent = `ipset: ${snapshot?.ipset_binary || 'не найден'}`;

  const notice = $('notice');
  if (!snapshot?.detected) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan не обнаружен. RouterForge не устанавливает пакет Antiscan автоматически.';
  } else if (!snapshot.running) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = 'Antiscan обнаружен, но /tmp/ascn.run отсутствует. Текущее состояние блокировок нельзя считать достоверным.';
  } else if ((snapshot.errors || []).length) {
    notice.hidden = false;
    notice.className = 'notice warn';
    notice.textContent = localizeMessages(snapshot.errors).join(' · ');
  } else {
    notice.hidden = true;
  }

  renderLifecycleControls();
}

function renderLifecycleControls() {
  const start = $('startAntiscan');
  const stop = $('stopAntiscan');
  const reload = $('reloadAntiscan');
  const restart = $('restartAntiscan');
  const state = $('lifecycleState');
  const hint = $('lifecycleHint');
  if (!start || !stop || !reload || !restart || !state || !hint) return;

  const detected = Boolean(snapshot?.detected);
  const running = Boolean(snapshot?.running);
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const busy = lifecycleBusy || operationBusy || configBusy;

  start.disabled = busy || !detected || running || upstreamBusy;
  stop.disabled = busy || !detected || !running || upstreamBusy;
  reload.disabled = busy || !detected || !running || upstreamBusy;
  restart.disabled = busy || !detected || upstreamBusy;

  state.className = 'state';
  if (lifecycleBusy) {
    state.classList.add('info');
    state.textContent = 'ОПЕРАЦИЯ…';
    hint.textContent = 'Ждём завершения штатной команды Antiscan и проверяем итоговое состояние.';
  } else if (operationBusy) {
    state.classList.add('info');
    state.textContent = 'ОБСЛУЖИВАНИЕ…';
    hint.textContent = 'Выполняется штатная сервисная команда Antiscan.';
  } else if (!detected) {
    state.classList.add('neutral');
    state.textContent = 'НЕДОСТУПНО';
    hint.textContent = 'Antiscan не обнаружен — команды управления недоступны.';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
    hint.textContent = 'Antiscan уже перечитывает конфигурацию или Geo-данные. Новое действие временно заблокировано.';
  } else if (running) {
    state.classList.add('good');
    state.textContent = 'РАБОТАЕТ';
    hint.textContent = 'Можно перечитать ascn.conf, штатно перезапустить или остановить Antiscan.';
  } else {
    state.classList.add('warn');
    state.textContent = 'ОСТАНОВЛЕН';
    hint.textContent = 'Можно штатно запустить Antiscan; restart в этом состоянии также выполнит запуск upstream-командой.';
  }

  renderOperationControls();
}

function renderOperationControls() {
  const state = $('operationState');
  const buttons = Array.from(document.querySelectorAll('[data-operation]'));
  if (!state || !buttons.length) return;

  const detected = Boolean(snapshot?.detected);
  const running = Boolean(snapshot?.running);
  const upstreamBusy = Boolean(snapshot?.config_reload_in_progress || snapshot?.geo_reload_in_progress);
  const busy = lifecycleBusy || operationBusy || configBusy;

  buttons.forEach((button) => {
    const requiresRunning = button.dataset.operation !== 'update_crontab';
    button.disabled = busy || !detected || upstreamBusy || (requiresRunning && !running);
  });

  state.className = 'state';
  if (operationBusy) {
    state.classList.add('info');
    state.textContent = 'ВЫПОЛНЕНИЕ…';
  } else if (!detected) {
    state.classList.add('neutral');
    state.textContent = 'НЕДОСТУПНО';
  } else if (upstreamBusy) {
    state.classList.add('warn');
    state.textContent = 'ANTISCAN ЗАНЯТ';
  } else if (!running) {
    state.classList.add('warn');
    state.textContent = 'ТОЛЬКО CRON';
  } else {
    state.classList.add('good');
    state.textContent = 'ГОТОВО';
  }
}

function renderRuntimeSummary() {
  const cfg = snapshot?.config || {};
  const rows = [
    ['Antiscan', snapshot?.detected ? 'Обнаружен' : 'Не обнаружен'],
    ['Маркер работы', snapshot?.running ? 'Есть · /tmp/ascn.run' : 'Нет'],
    ['Перечитывание конфигурации', snapshot?.config_reload_in_progress ? 'Выполняется' : 'Нет'],
    ['Обновление Geo', snapshot?.geo_reload_in_progress ? 'Выполняется' : 'Нет'],
    ['Интерфейсы', (cfg.isp_interfaces || []).join(', ') || '—'],
    ['Защищаемые порты', (cfg.ports || []).join(', ') || '—']
  ];
  $('runtimeSummary').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');
}

function renderRiskSummary() {
  const p = snapshot?.protection || {};
  const risk = p.ips_ban_enabled && Number(p.different_ip_threshold || 0) > 0 &&
    Number(p.different_ip_threshold || 0) <= 5 && Number(p.candidate_storage_seconds || 0) >= 86400;
  const root = $('riskSummary');
  if (!snapshot?.detected) {
    root.innerHTML = '<div class="risk-empty">Antiscan не обнаружен — анализировать нечего.</div>';
    return;
  }
  root.innerHTML = `
    <div class="risk-verdict ${risk ? 'warn' : 'good'}">
      <span>${risk ? '!' : '✓'}</span>
      <div>
        <strong>${risk ? 'Есть риск ложной /24-блокировки' : 'Явного риска ложной /24-блокировки не видно'}</strong>
        <p>${risk
          ? `Порог ${escapeHTML(String(p.different_ip_threshold))} адресов сочетается с хранением кандидатов ${escapeHTML(fmtDuration(p.candidate_storage_seconds))}. Для мобильных пулов это может быть слишком агрессивно.`
          : 'Текущее сочетание порога и времени хранения кандидатов не попало под встроенный профиль повышенного риска.'}</p>
      </div>
    </div>
    <div class="risk-pairs">
      <div><span>Порог /24</span><strong>${escapeHTML(String(p.different_ip_threshold || '—'))}</strong></div>
      <div><span>Кандидаты хранятся</span><strong>${escapeHTML(fmtDuration(p.candidate_storage_seconds))}</strong></div>
      <div><span>Блокировка подсети</span><strong>${escapeHTML(fmtDuration(p.subnet_ban_seconds))}</strong></div>
      <div><span>Порог новых подключений</span><strong>${escapeHTML(String(p.recent_hitcount || '—'))}</strong></div>
    </div>
  `;
}

function renderProtection() {
  const p = snapshot?.protection || {};
  const cfg = snapshot?.config || {};
  const rows = [
    ['Интерфейсы', (cfg.isp_interfaces || []).join(', ') || '—'],
    ['Порты роутера', (cfg.ports || []).join(', ') || '—'],
    ['Проброшенные порты', (cfg.forwarded_ports || []).join(', ') || '—'],
    ['Защита IP и подсетей', boolText(Boolean(p.ips_ban_enabled))],
    ['Окно новых соединений', fmtDuration(p.recent_window_seconds)],
    ['Порог новых подключений', p.recent_hitcount || '—'],
    ['Лимит одновременных соединений', p.concurrent_connection_limit || '—'],
    ['Блокировка IP', fmtDuration(p.direct_ip_ban_seconds)],
    ['Порог разных IP в /24', p.different_ip_threshold || '—'],
    ['Хранение кандидатов', fmtDuration(p.candidate_storage_seconds)],
    ['Блокировка /24', fmtDuration(p.subnet_ban_seconds)],
    ['Ловушка', boolText(Boolean(p.honeypot_enabled))],
    ['Порты ловушки', (cfg.honeypot_ports || []).join(', ') || '—'],
    ['Пользовательские исключения', boolText(Boolean(cfg.use_custom_exclude_list))],
    ['Режим пользовательского списка', cfg.custom_lists_block_mode || '0'],
    ['Режим Geo', cfg.geoblock_mode || '0'],
    ['Страны Geo', (cfg.geoblock_countries || []).join(', ') || '—'],
    ['Страны-исключения Geo', (cfg.geo_exclude_countries || []).join(', ') || '—'],
    ['Импорт блокировок Keenetic', boolText(Boolean(cfg.read_ndm_lockout_ipsets))],
    ['Сохранение наборов ipset', boolText(Boolean(cfg.save_ipsets))]
  ];
  $('protection').innerHTML = rows.map(([key, value]) => `
    <div class="kv"><span>${escapeHTML(key)}</span><strong>${escapeHTML(String(value))}</strong></div>
  `).join('');
}

function renderWarnings() {
  const warnings = localizeMessages([...(snapshot?.warnings || []), ...(snapshot?.errors || [])]);
  if (!warnings.length) {
    $('warnings').innerHTML = '<div class="ok-note">Явных признаков повышенного риска в текущем состоянии не найдено.</div>';
    return;
  }
  $('warnings').innerHTML = warnings.map((warning) => `
    <div class="warning-item"><span>!</span><p>${escapeHTML(warning)}</p></div>
  `).join('');
}

function isBenignUnknownSetCount(item) {
  return Boolean(
    item?.exists &&
    !item?.count_known &&
    item?.error === 'ipset entry count missing'
  );
}

function renderSets() {
  const sets = snapshot?.ipsets || [];
  $('sets').innerHTML = sets.map((item) => {
    const countUnavailable = isBenignUnknownSetCount(item);
    const hasOperationalError = Boolean(item.error) && !countUnavailable;
    const state = item.exists ? (hasOperationalError ? 'ЧАСТИЧНО' : 'АКТИВЕН') : 'НЕТ';
    const cls = item.exists ? (hasOperationalError ? 'warn' : 'good') : 'neutral';
    const count = item.count_known ? String(item.count ?? 0) : '—';
    const detail = countUnavailable
      ? '<small class="row-note">Число записей недоступно</small>'
      : (hasOperationalError ? `<small class="row-error">${escapeHTML(localizeMessage(item.error))}</small>` : '');
    return `<tr>
      <td class="mono strong">${escapeHTML(item.name)}</td>
      <td>${escapeHTML(setDescriptions[item.name] || 'Набор Antiscan')}</td>
      <td><span class="state ${cls}">${state}</span>${detail}</td>
      <td class="mono">${count}</td>
    </tr>`;
  }).join('');
}

function verdictTitle(result) {
  if (result.verdict === 'blocked') return 'ЗАБЛОКИРОВАН';
  if (result.verdict === 'excluded') return 'ИСКЛЮЧЁН';
  if (result.verdict === 'not-blocked') return 'НЕ ЗАБЛОКИРОВАН';
  if (result.verdict === 'candidate') return 'КАНДИДАТ';
  if (result.verdict === 'stopped') return 'ANTISCAN ОСТАНОВЛЕН';
  if (result.verdict === 'not-detected') return 'ANTISCAN НЕ ОБНАРУЖЕН';
  return 'НЕДОСТАТОЧНО ДАННЫХ';
}

function renderInspect(result) {
  const root = $('inspectResult');
  const cls = result.blocked ? 'bad' : result.conclusive ? 'good' : 'warn';
  const evidence = (result.evidence || []).map((item) => `
    <div class="evidence-row">
      <div><strong>${escapeHTML(reasonLabel(item.kind))}</strong><span class="mono">${escapeHTML(item.set || '')}</span></div>
      <p>${escapeHTML(evidenceSummary(item))}</p>
    </div>
  `).join('');
  const warnings = localizeMessages(result.warnings).map((item) => `<div class="inspect-warning">${escapeHTML(item)}</div>`).join('');
  const explanation = reasonExplanation(result);
  const actions = result.ip && (result.blocked || result.verdict === 'candidate') ? `
    <div class="inspect-actions">
      <button class="button small" type="button" data-action="exclude" data-entry="${escapeHTML(result.ip)}">Добавить IP в исключения</button>
    </div>` : '';
  root.className = `inspect-result ${cls}`;
  root.innerHTML = `
    <div class="inspect-verdict">
      <div><span class="mono">${escapeHTML(result.ip || '')}</span><strong>${verdictTitle(result)}</strong></div>
      <span class="state ${cls}">${escapeHTML(reasonLabel(result.reason || result.verdict || 'unknown'))}</span>
    </div>
    ${explanation ? `<div class="reason-explanation">${escapeHTML(explanation)}</div>` : ''}
    ${evidence || '<div class="muted inspect-empty">Совпадения в доступных наборах Antiscan не найдены.</div>'}
    ${warnings}
    ${actions}
  `;
}

function entrySecondary(entry) {
  const bits = [];
  if (entry.timeout_known) {
    const seconds = Number(entry.timeout_seconds || 0);
    if (seconds === 0) {
      bits.push('без таймаута');
    } else {
      const expires = new Date(Date.now() + seconds * 1000);
      bits.push(`осталось ${fmtDuration(seconds)} · примерно до ${expires.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`);
    }
  }
  if (entry.packets_known) bits.push(`${entry.packets} пак.`);
  if (entry.bytes_known) bits.push(fmtBytes(entry.bytes));
  return bits.join(' · ') || 'запись в наборе';
}

function canUnbanSet(name) {
  return ['ascn_ips', 'ascn_subnets', 'ascn_honeypot'].includes(name);
}

function blockedEntryActions(setName, entry) {
  const buttons = [];
  if (canUnbanSet(setName)) {
    buttons.push(`<button class="button small danger" type="button" data-action="unban" data-set="${escapeHTML(setName)}" data-entry="${escapeHTML(entry.value)}">Снять блокировку</button>`);
  }
  if (['ascn_ips', 'ascn_subnets', 'ascn_honeypot', 'ascn_ndm_lockout', 'ascn_candidates'].includes(setName)) {
    buttons.push(`<button class="button small" type="button" data-action="exclude" data-entry="${escapeHTML(entry.value)}">В исключения</button>`);
  }
  return buttons.length ? `<span class="entry-actions">${buttons.join('')}</span>` : '';
}

function currentRuntimeListMode() {
  return document.documentElement.dataset.section === 'geo' ? 'geo' : 'lists';
}

function runtimeListOptions(mode) {
  return mode === 'geo'
    ? [
        ['ascn_geo_exclude', 'Geo-исключения'],
        ['ascn_geo_blacklist', 'Geo-чёрный список'],
        ['ascn_geo_whitelist', 'Geo-белый список']
      ]
    : [
        ['ascn_custom_exclude', 'Пользовательские исключения'],
        ['ascn_custom_blacklist', 'Пользовательский чёрный список'],
        ['ascn_custom_whitelist', 'Пользовательский белый список']
      ];
}

function configureListSetMode(mode) {
  const select = $('listSet');
  const title = $('runtimeListsTitle');
  const description = $('runtimeListsDescription');
  const filter = $('listFilter');
  if (!select || !title || !description || !filter) return;

  const options = runtimeListOptions(mode);
  const previousName = browserState[mode]?.name || '';
  const changedMode = select.dataset.mode !== mode;
  select.dataset.mode = mode;
  select.innerHTML = options.map(([value, label]) =>
    `<option value="${value}">${label}</option>`
  ).join('');
  if (previousName && options.some(([value]) => value === previousName)) {
    select.value = previousName;
  }
  if (changedMode) filter.value = '';

  if (mode === 'geo') {
    title.textContent = 'Geo-наборы Antiscan';
    description.textContent = 'Текущее содержимое Geo ipset. Для больших наборов показываются первые 500 записей.';
  } else {
    title.textContent = 'Активные пользовательские списки';
    description.textContent = 'Текущее содержимое runtime ipset для blacklist, whitelist и exclude.';
  }

  if (browserState[mode]) {
    renderSetPage(mode);
  } else {
    $('listMeta').textContent = '—';
    $('listEntries').className = 'entry-table empty-state';
    $('listEntries').textContent = 'Выберите набор и загрузите текущее состояние.';
  }
}

function renderSetPage(kind) {
  const page = browserState[kind];
  const blocked = kind === 'blocked';
  if (!blocked && kind !== currentRuntimeListMode()) return;
  const target = blocked ? $('blockedEntries') : $('listEntries');
  const meta = blocked ? $('blockedMeta') : $('listMeta');
  const filter = (blocked ? $('blockedFilter').value : $('listFilter').value).trim().toLowerCase();

  if (!page) {
    target.className = 'entry-table empty-state';
    target.textContent = 'Набор ещё не загружен.';
    meta.textContent = '—';
    return;
  }
  if (page.error) {
    target.className = 'entry-table empty-state bad-text';
    target.textContent = localizeMessage(page.error);
    meta.textContent = page.name || '—';
    return;
  }
  if (!page.exists) {
    target.className = 'entry-table empty-state';
    target.textContent = `${page.name}: набор сейчас отсутствует.`;
    meta.textContent = 'НЕТ';
    return;
  }

  const all = page.entries || [];
  const entries = filter ? all.filter((entry) => String(entry.value || '').toLowerCase().includes(filter)) : all;
  const total = page.count_known ? page.count : all.length;
  meta.textContent = `${page.name} · ${total} записей${page.truncated ? ` · показаны первые ${page.limit}` : ''}`;

  if (!entries.length) {
    target.className = 'entry-table empty-state';
    target.textContent = filter ? 'По фильтру ничего не найдено.' : 'Набор пуст.';
    return;
  }

  target.className = 'entry-table';
  target.innerHTML = entries.map((entry) => {
    const actions = blocked ? blockedEntryActions(page.name, entry) : '';
    return `
      <div class="entry-row ${actions ? 'has-actions' : ''}">
        <span class="mono entry-value">${escapeHTML(entry.value)}</span>
        <span class="entry-meta-wrap">
          <span class="entry-meta">${escapeHTML(entrySecondary(entry))}</span>
          <small>${escapeHTML(setOperationalExplanation(page.name))}</small>
        </span>
        ${actions}
      </div>`;
  }).join('');
}

async function loadSet(kind) {
  const blocked = kind === 'blocked';
  const select = blocked ? $('blockedSet') : $('listSet');
  const button = blocked ? $('reloadBlocked') : $('reloadList');
  const target = blocked ? $('blockedEntries') : $('listEntries');
  button.disabled = true;
  if (blocked || kind === currentRuntimeListMode()) {
    target.className = 'entry-table empty-state';
    target.textContent = 'Читаем текущий набор ipset…';
  }
  try {
    browserState[kind] = await api(`sets?name=${encodeURIComponent(select.value)}&limit=500`);
    renderSetPage(kind);
  } catch (error) {
    browserState[kind] = { name: select.value, error: error.message || 'Не удалось прочитать ipset.' };
    renderSetPage(kind);
  } finally {
    button.disabled = false;
  }
}

function mutationSuccessMessage(context, payload) {
  if (!payload?.changed) {
    if (context === 'unban') return 'Запись уже отсутствовала в активной блокировке.';
    if (context === 'list-entry') return 'Такая запись уже есть в выбранном списке.';
    if (context === 'lifecycle:start') return 'Antiscan уже запущен.';
    if (context === 'lifecycle:stop') return 'Antiscan уже остановлен.';
    if (context === 'schedule:apply') return 'ascn_crontab.conf уже совпадал с формой; активный crontab синхронизирован.';
    if (context.startsWith('flush:')) return 'Выбранные данные уже были пусты; штатная очистка проверена.';
    if (context.startsWith('operation:')) return 'Команда не потребовалась: текущее состояние уже корректно.';
    return 'Состояние уже соответствовало запросу.';
  }
  if (context === 'unban') return 'Блокировка снята, результат проверен.';
  if (context === 'list-entry') return 'Запись добавлена в список и результат проверен.';
  if (context === 'lifecycle:start') return 'Antiscan запущен, состояние проверено.';
  if (context === 'lifecycle:stop') return 'Antiscan остановлен. Защита отключена до следующего запуска.';
  if (context === 'lifecycle:reload') return 'Antiscan перечитал конфигурацию, состояние проверено.';
  if (context === 'lifecycle:restart') return 'Antiscan штатно перезапущен, состояние проверено.';
  if (context === 'rci-token:set') return 'RCI-токен сохранён и проверен штатной командой Antiscan.';
  if (context === 'rci-token:check') return 'Сохранённый RCI-токен прошёл проверку Keenetic.';
  if (context === 'rci-token:delete') return 'RCI-токен удалён, состояние проверено.';
  if (context === 'schedule:apply') return 'Расписание Antiscan обновлено и синхронизировано с активным crontab.';
  if (context.startsWith('flush:')) return 'Штатная очистка Antiscan выполнена, результат проверен.';
  if (context.startsWith('operation:')) return 'Штатная сервисная команда Antiscan выполнена и проверена.';
  if (context === 'config') {
    return payload?.runtime_applied
      ? 'Настройки сохранены, применены и проверены.'
      : 'Настройки сохранены и проверены. Они вступят в силу при следующем запуске Antiscan.';
  }
  return 'Изменение применено и проверено.';
}

function showMutationResult(payload, failed = false, context = '') {
  const notice = $('actionNotice');
  notice.hidden = false;
  notice.className = `notice action-notice ${failed ? 'bad' : 'good'}`;
  const warnings = localizeMessages(payload?.warnings);
  if (failed) {
    const prefix = payload?.rollback_performed
      ? 'Операция не выполнена. Предыдущее состояние восстановлено.'
      : 'Операцию выполнить не удалось.';
    const reason = localizeMessage(payload?.error || payload?.message || '');
    notice.textContent = [prefix, reason, ...warnings].filter(Boolean).join(' · ');
    return;
  }
  const messages = [mutationSuccessMessage(context, payload), ...warnings];
  if (payload?.restart_required) {
    messages.push(context.startsWith('flush:')
      ? 'После штатной очистки восстановите нужные данные и перезапустите Antiscan перед возвратом соответствующей защиты.'
      : 'Требуется остановить и снова запустить Antiscan, чтобы полностью применить новый IPSETS_DIRECTORY.');
  }
  notice.textContent = messages.filter(Boolean).join(' · ');
}

async function performUnban(setName, entry) {
  if (!window.confirm(`Снять блокировку только с этой записи?\n\n${setName}\n${entry}\n\nОстальные записи не изменятся.`)) return;
  try {
    const result = await mutate('unban', { set: setName, entry, confirm: 'UNBAN' });
    showMutationResult(result, false, 'unban');
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'unban');
  }
}

async function performListEntry(listName, entry) {
  const target = listName === 'exclude' ? 'пользовательские исключения' : 'пользовательский белый список';
  if (!entry) return;
  if (!window.confirm(`Добавить запись в ${target}?\n\n${entry}\n\nФайл изменится атомарно. Если список активен, Antiscan перечитает его и RouterForge проверит результат.`)) return;
  try {
    const result = await mutate('list-entry', { list: listName, entry, confirm: 'ADD' });
    showMutationResult(result, false, 'list-entry');
    await loadStatus();
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, 'list-entry');
  }
}

function customListLabel(name) {
  return {
    blacklist: 'Чёрный список',
    whitelist: 'Белый список',
    exclude: 'Исключения'
  }[name] || name || 'Пользовательский список';
}

function renderCustomList(page) {
  browserState.customSource = page;
  const target = $('customListEntries');
  const meta = $('customListMeta');
  const clearButton = $('clearCustomList');
  const reloadRuntimeButton = $('reloadCustomListRuntime');
  if (!target || !meta) return;

  if (page?.error) {
    target.className = 'entry-table empty-state bad-text';
    target.textContent = localizeMessage(page.error);
    meta.textContent = customListLabel(page.list);
    return;
  }

  const state = page?.active ? 'активен' : page?.configured ? 'включён, Antiscan остановлен' : 'выключен';
  const runtime = page?.active ? (page.runtime_exists ? 'runtime есть' : 'runtime отсутствует') : 'runtime не требуется';
  const skipped = Number(page?.skipped_invalid || 0);
  meta.textContent = `${customListLabel(page?.list)} · записей ${Number(page?.count || 0)} · ${state} · ${runtime}${skipped ? ` · пропущено строк ${skipped}` : ''}`;

  if (clearButton) {
    clearButton.disabled = customListBusy || Boolean(page?.configured) || Number(page?.count || 0) === 0;
    clearButton.title = page?.configured ? 'Сначала отключите список в ascn.conf.' : '';
  }
  if (reloadRuntimeButton) {
    reloadRuntimeButton.disabled = customListBusy || !page?.configured || !page?.running || Number(page?.count || 0) === 0;
  }

  const entries = page?.entries || [];
  if (!entries.length) {
    target.className = 'entry-table empty-state';
    target.textContent = 'В исходном файле нет корректных IPv4/CIDR записей.';
    return;
  }

  target.className = 'entry-table';
  target.innerHTML = entries.map((entry) => {
    const finalConfiguredEntry = Boolean(page?.configured) && entries.length === 1;
    return `<div class="entry-row has-actions">
      <span class="mono entry-value">${escapeHTML(entry)}</span>
      <span class="entry-meta-wrap"><span class="entry-meta">исходный файл ${escapeHTML(page?.set || '')}</span><small>${page?.active ? 'Изменение будет применено через штатный update_ipsets custom.' : 'Изменение затронет только исходный файл.'}</small></span>
      <span class="entry-actions"><button class="button small danger" type="button" data-custom-delete="${escapeHTML(entry)}" ${finalConfiguredEntry ? 'disabled title="Сначала отключите список в ascn.conf."' : ''}>Удалить</button></span>
    </div>`;
  }).join('');
}

async function loadCustomList() {
  const select = $('customListName');
  const button = $('reloadCustomList');
  if (!select || !button) return;
  button.disabled = true;
  try {
    const page = await api(`custom-lists?list=${encodeURIComponent(select.value)}`);
    renderCustomList(page);
  } catch (error) {
    renderCustomList({ list: select.value, error: error.message || 'Не удалось прочитать исходный пользовательский список.' });
  } finally {
    button.disabled = false;
  }
}

async function performCustomListMutation(action, entry = '') {
  const list = $('customListName')?.value || '';
  const prompts = {
    add: `Добавить запись в ${customListLabel(list)}?\n\n${entry}\n\nФайл будет изменён атомарно. Если список активен, Antiscan штатно перечитает custom ipset и RouterForge проверит результат.`,
    delete: `Удалить запись из ${customListLabel(list)}?\n\n${entry}\n\nЕсли список активен, Antiscan перечитает custom ipset и RouterForge проверит удаление.`,
    clear: `Очистить все адреса из ${customListLabel(list)}?\n\nКомментарии сохранятся. Очистка разрешена только для списка, отключённого в ascn.conf.`,
    reload: `Перечитать ${customListLabel(list)} из исходного файла?\n\nБудет вызван штатный update_ipsets custom и проверен runtime ipset.`
  };
  if (!prompts[action]) return;
  if ((action === 'add' || action === 'delete') && !entry) return;
  if (!window.confirm(prompts[action])) return;

  customListBusy = true;
  try {
    const result = await mutate('custom-list', {
      action,
      list,
      entry: action === 'add' || action === 'delete' ? entry : '',
      confirm: action.toUpperCase()
    });
    showMutationResult(result, false, `custom-list:${action}`);
    if (action === 'add') $('customListEntry').value = '';
    await loadCustomList();
    await loadStatus();
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `custom-list:${action}`);
    await loadCustomList();
  } finally {
    customListBusy = false;
  }
}

async function performLifecycle(action) {
  const prompts = {
    start: 'Запустить Antiscan?\n\nБудут созданы штатные наборы ipset и правила фильтрации Antiscan.',
    stop: 'Остановить Antiscan?\n\nЗащита Antiscan будет отключена до следующего запуска. Сохранение состояния при остановке выполняет сам Antiscan согласно SAVE_ON_EXIT.',
    reload: 'Перечитать текущий ascn.conf?\n\nAntiscan штатно перестроит необходимые правила и наборы, после чего RouterForge проверит итоговое состояние.',
    restart: 'Штатно перезапустить Antiscan?\n\nБудет вызван именно S99ascn restart. Upstream выполнит свой stop 1, затем start и сохранит предусмотренную им семантику перезапуска.'
  };
  if (!prompts[action] || !window.confirm(prompts[action])) return;

  lifecycleBusy = true;
  renderLifecycleControls();
  try {
    const result = await mutate('lifecycle', { action, confirm: action.toUpperCase() });
    showMutationResult(result, false, `lifecycle:${action}`);
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    showMutationResult(error.payload || { error: error.message }, true, `lifecycle:${action}`);
  } finally {
    lifecycleBusy = false;
    renderLifecycleControls();
  }
}

async function performOperation(action, scope = '') {
  const key = scope ? `${action}:${scope}` : action;
  const prompts = {
    update_rules: 'Восстановить правила Antiscan?\n\nКоманда update_rules будет вызвана только если цепочка ANTISCAN сейчас отсутствует. Это защищает от повторного добавления jump-правил.',
    read_candidates: 'Обработать текущих кандидатов?\n\nAntiscan проверит накопленные адреса и при достижении порога перенесёт соответствующие /24 в блокировку подсетей.',
    read_ndm_ipsets: 'Импортировать текущие блокировки Keenetic?\n\nAntiscan перечитает системные lockout ipset и добавит найденные IPv4 в свой runtime-набор.',
    save_ipsets: 'Сохранить runtime ipset?\n\nAntiscan выполнит штатный save_ipsets согласно SAVE_IPSETS и IPSETS_DIRECTORY.',
    'update_ipsets:custom': 'Перечитать активные пользовательские списки?\n\nAntiscan штатно пересоздаст активные custom ipset и восстановит правила.',
    'update_ipsets:geo': 'Обновить Geo-списки сейчас?\n\nAntiscan скачает актуальные подсети настроенных стран. Операция может занять несколько минут.',
    retry_load_geo: 'Повторить неудачную загрузку Geo?\n\nAntiscan выполнит штатный retry_load_geo и при успехе уберёт временную retry-задачу.',
    update_crontab: 'Синхронизировать cron с ascn_crontab.conf?\n\nБудут изменены только строки Antiscan, штатной командой update_crontab.'
  };
  if (!prompts[key] || !window.confirm(prompts[key])) return;

  operationBusy = true;
  renderLifecycleControls();
  const output = $('operationOutput');
  if (output) output.textContent = 'Выполняется штатная команда Antiscan…';
  try {
    const result = await mutate('operation', { action, scope, confirm: 'RUN' });
    showMutationResult(result, false, `operation:${key}`);
    if (output) output.textContent = result.output || 'Команда завершена без текстового вывода.';
    await loadStatus();
    if (browserState.blocked) await loadSet('blocked');
    if (browserState.lists) await loadSet('lists');
    if (browserState.geo) await loadSet('geo');
    if (diagnosticsLoaded) await loadDiagnostics();
  } catch (error) {
    const payload = error.payload || { error: error.message };
    showMutationResult(payload, true, `operation:${key}`);
    if (output) output.textContent = localizeMessage(payload.error || error.message || 'Ошибка сервисной операции.');
  } finally {
    operationBusy = false;
    renderLifecycleControls();
  }
}

function auditActionTitle(action) {
  if (String(action || '').startsWith('flush:')) {
    return `Очистка / восстановление: ${flushTargetLabel(String(action).slice(6))}`;
  }
  return {
    unban: 'Снятие одной блокировки',
    'list-entry': 'Изменение пользовательского списка',
    'custom-list:add': 'Добавление в пользовательский список',
    'custom-list:delete': 'Удаление из пользовательского списка',
    'custom-list:clear': 'Очистка пользовательского списка',
    'custom-list:reload': 'Перечитывание пользовательского списка',
    'rci-token:set': 'Установка RCI-токена',
    'rci-token:check': 'Проверка RCI-токена',
    'rci-token:delete': 'Удаление RCI-токена',
    'lifecycle:start': 'Запуск Antiscan',
    'lifecycle:stop': 'Остановка Antiscan',
    'lifecycle:reload': 'Перечитывание конфигурации',
    'lifecycle:restart': 'Перезапуск Antiscan',
    'operation:update_rules': 'Восстановление правил Antiscan',
    'operation:read_candidates': 'Обработка кандидатов',
    'operation:read_ndm_ipsets': 'Импорт блокировок Keenetic',
    'operation:save_ipsets': 'Сохранение runtime ipset',
    'operation:update_ipsets:custom': 'Перечитывание пользовательских списков',
    'operation:update_ipsets:geo': 'Обновление Geo-списков',
    'operation:retry_load_geo': 'Повторная загрузка Geo',
    'operation:update_crontab': 'Синхронизация cron',
    'schedule:apply': 'Изменение расписания Antiscan',
    config: 'Изменение конфигурации'
  }[action] || action || 'Действие Antiscan Manager';
}

function renderHistory(page) {
  const events = page?.events || [];
  const target = $('historyEntries');
  const meta = $('historyMeta');
  historyLoaded = true;
  meta.textContent = `${events.length} событий · новые сверху${page?.skipped_invalid_lines ? ` · пропущено повреждённых строк: ${page.skipped_invalid_lines}` : ''}`;

  if (!events.length) {
    target.className = 'history-list empty-state';
    target.textContent = 'В журнале пока нет действий Antiscan Manager.';
    return;
  }

  target.className = 'history-list';
  target.innerHTML = events.map((event) => {
    const failed = event.outcome !== 'success';
    const flags = [];
    if (event.changed) flags.push('<span class="history-flag">изменено</span>');
    if (event.verified) flags.push('<span class="history-flag good">проверено</span>');
    if (event.rollback) flags.push('<span class="history-flag bad">выполнен откат</span>');
    if (event.restart_required) flags.push('<span class="history-flag warn">нужен перезапуск</span>');
    const warnings = localizeMessages(event.warnings).map((item) => `<small>⚠ ${escapeHTML(item)}</small>`).join('');
    return `
      <article class="history-row ${failed ? 'failed' : ''}">
        <div class="history-main">
          <div class="history-title">
            <span class="state ${failed ? 'bad' : 'good'}">${failed ? 'ОШИБКА' : 'УСПЕХ'}</span>
            <strong>${escapeHTML(auditActionTitle(event.action))}</strong>
            <span class="mono muted">${escapeHTML(fmtAuditTime(event.timestamp))}</span>
          </div>
          <p>${escapeHTML(localizeMessage(event.summary || '—'))}</p>
          ${event.target ? `<span class="mono history-target">${escapeHTML(event.target)}</span>` : ''}
          ${warnings}
        </div>
        <div class="history-side">
          <span class="mono">${Number(event.duration_ms || 0)} мс</span>
          <span class="mono">HTTP ${escapeHTML(String(event.http_status || '—'))}</span>
          <div class="history-flags">${flags.join('')}</div>
        </div>
      </article>`;
  }).join('');
}

async function loadHistory() {
  const button = $('reloadHistory');
  if (button) button.disabled = true;
  try {
    const page = await api('history?limit=50');
    renderHistory(page);
  } catch (error) {
    historyLoaded = true;
    $('historyMeta').textContent = 'Не удалось прочитать историю';
    $('historyEntries').className = 'history-list empty-state bad-text';
    $('historyEntries').textContent = localizeMessage(error.message || 'Не удалось прочитать журнал действий.');
  } finally {
    if (button) button.disabled = false;
  }
}

async function loadStatus() {
  $('refresh').disabled = true;
  try {
    snapshot = await api('status');
    renderState();
    renderRuntimeSummary();
    renderRiskSummary();
    renderProtection();
    renderWarnings();
    renderSets();
    populateConfigEditor(false);
  } catch (error) {
    const notice = $('notice');
    notice.hidden = false;
    notice.className = 'notice bad';
    notice.textContent = localizeMessage(error.message || 'Не удалось получить статус Antiscan Manager.');
  } finally {
    $('refresh').disabled = false;
  }
}

async function inspectIP() {
  const ip = $('inspectIp').value.trim();
  if (!ip) return;
  $('inspectButton').disabled = true;
  $('inspectResult').className = 'inspect-result empty';
  $('inspectResult').textContent = 'Проверяем текущее состояние Antiscan…';
  try {
    const result = await api(`inspect?ip=${encodeURIComponent(ip)}`);
    renderInspect(result);
  } catch (error) {
    const payload = error.payload || {};
    renderInspect({
      ip,
      conclusive: false,
      blocked: false,
      verdict: payload.verdict || 'unknown',
      warnings: localizeMessages(payload.warnings || [error.message || 'Ошибка проверки'])
    });
  } finally {
    $('inspectButton').disabled = false;
  }
}

document.querySelectorAll('.tab').forEach((button) => {
  button.addEventListener('click', () => {
    const section = button.dataset.tab;
    activateTab(section);
    if (section === 'blocked' && !browserState.blocked) loadSet('blocked');
    if (section === 'blocked' && !flushPreviewLoaded) loadFlushPreview();
    if (section === 'lists') {
      configureListSetMode('lists');
      if (!browserState.lists) loadSet('lists');
      if (!browserState.customSource) loadCustomList();
    }
    if (section === 'geo') {
      configureListSetMode('geo');
      if (!browserState.geo) loadSet('geo');
    }
    if (section === 'history' && !historyLoaded) loadHistory();
    if (section === 'diagnostics' && !diagnosticsLoaded) loadDiagnostics();
    if (section === 'diagnostics' && !compatibilityLoaded) loadCompatibility();
    if (section === 'diagnostics' && !rciTokenLoaded) loadRCITokenStatus();
    if (section === 'schedule' && !schedulerLoaded) loadScheduler();
  });
});

$('refresh').addEventListener('click', async () => {
  await loadStatus();
  if (browserState.blocked) await loadSet('blocked');
  if (flushPreviewLoaded) await loadFlushPreview();
  if (browserState.lists) await loadSet('lists');
  if (browserState.customSource) await loadCustomList();
  if (historyLoaded) await loadHistory();
  if (diagnosticsLoaded) await loadDiagnostics();
  if (compatibilityLoaded) await loadCompatibility();
  if (rciTokenLoaded) await loadRCITokenStatus();
  if (schedulerLoaded) await loadScheduler();
});
$('inspectButton').addEventListener('click', inspectIP);
$('inspectIp').addEventListener('keydown', (event) => {
  if (event.key === 'Enter') inspectIP();
});
$('reloadBlocked').addEventListener('click', () => loadSet('blocked'));
$('reloadList').addEventListener('click', () => loadSet(currentRuntimeListMode()));
$('blockedSet').addEventListener('change', () => loadSet('blocked'));
$('listSet').addEventListener('change', () => loadSet(currentRuntimeListMode()));
$('blockedFilter').addEventListener('input', () => renderSetPage('blocked'));
$('listFilter').addEventListener('input', () => renderSetPage(currentRuntimeListMode()));
$('flushTarget').addEventListener('change', loadFlushPreview);
$('reloadFlushPreview').addEventListener('click', loadFlushPreview);
$('flushSelected').addEventListener('click', performFlush);
$('customListName').addEventListener('change', loadCustomList);
$('reloadCustomList').addEventListener('click', loadCustomList);
$('addCustomListEntry').addEventListener('click', () => performCustomListMutation('add', $('customListEntry').value.trim()));
$('customListEntry').addEventListener('keydown', (event) => {
  if (event.key === 'Enter') performCustomListMutation('add', $('customListEntry').value.trim());
});
$('reloadCustomListRuntime').addEventListener('click', () => performCustomListMutation('reload'));
$('clearCustomList').addEventListener('click', () => performCustomListMutation('clear'));
$('startAntiscan').addEventListener('click', () => performLifecycle('start'));
$('reloadAntiscan').addEventListener('click', () => performLifecycle('reload'));
$('restartAntiscan').addEventListener('click', () => performLifecycle('restart'));
$('stopAntiscan').addEventListener('click', () => performLifecycle('stop'));
document.querySelectorAll('[data-operation]').forEach((button) => {
  button.addEventListener('click', () => performOperation(button.dataset.operation || '', button.dataset.scope || ''));
});
$('reloadHistory').addEventListener('click', loadHistory);
$('reloadDiagnostics').addEventListener('click', loadDiagnostics);
$('reloadCompatibility').addEventListener('click', loadCompatibility);
$('reloadRCIToken').addEventListener('click', loadRCITokenStatus);
$('setRCIToken').addEventListener('click', () => performRCITokenAction('set'));
$('checkRCIToken').addEventListener('click', () => performRCITokenAction('check'));
$('deleteRCIToken').addEventListener('click', () => performRCITokenAction('delete'));
$('rciTokenInput').addEventListener('keydown', (event) => {
  if (event.key === 'Enter') performRCITokenAction('set');
});
$('reloadScheduler').addEventListener('click', loadScheduler);
$('resetScheduler').addEventListener('click', () => {
  if (schedulerSnapshot) renderScheduler(schedulerSnapshot, true);
});
$('applyScheduler').addEventListener('click', applyScheduler);
$('schedulerTaskRows').addEventListener('input', updateSchedulerState);
$('schedulerTaskRows').addEventListener('change', updateSchedulerState);
$('configForm').addEventListener('submit', (event) => event.preventDefault());
$('configForm').addEventListener('input', updateConfigEditorState);
$('configForm').addEventListener('change', updateConfigEditorState);
$('resetConfig').addEventListener('click', () => {
  configDirty = false;
  populateConfigEditor(true);
});
$('applyConfig').addEventListener('click', applyConfigEditor);
document.addEventListener('click', (event) => {
  const customDelete = event.target.closest('[data-custom-delete]');
  if (customDelete) {
    performCustomListMutation('delete', customDelete.dataset.customDelete || '');
    return;
  }
  const button = event.target.closest('[data-action]');
  if (!button) return;
  const action = button.dataset.action;
  const entry = button.dataset.entry || '';
  if (action === 'unban') performUnban(button.dataset.set || '', entry);
  if (action === 'exclude') performListEntry('exclude', entry);
});

startRouterForgeBridge();
loadStatus();
