const CATEGORY_ORDER = [
  'system',
  'network',
  'dns',
  'routing',
  'proxy',
  'adaptation',
  'automation',
  'security',
  'other'
];

const RAW_CATEGORY_MAP = new Map([
  ['core', 'system'],
  ['integrations', 'other'],
  ['vpn / routing', 'routing'],
  ['remote access', 'routing'],
  ['routing', 'routing'],
  ['dpi / bypass', 'adaptation'],
  ['dns', 'dns'],
  ['dns / filtering', 'dns'],
  ['dns / routing', 'dns'],
  ['proxy', 'proxy'],
  ['proxy / routing', 'proxy'],
  ['network tools', 'network'],
  ['toolkit', 'network'],
  ['diagnostics', 'network'],
  ['monitoring', 'network'],
  ['administration', 'system'],
  ['automation', 'automation'],
  ['backup', 'automation'],
  ['downloads', 'automation'],
  ['development', 'automation'],
  ['security', 'security']
]);

const LABELS = {
  ru: {
    system: 'Система и администрирование',
    network: 'Сеть и диагностика',
    dns: 'DNS и фильтрация',
    routing: 'Маршрутизация и туннелирование',
    proxy: 'Прокси',
    adaptation: 'Сетевая совместимость',
    automation: 'Автоматизация и сервисы',
    security: 'Безопасность',
    other: 'Прочее',
    state_updates: 'Есть обновления',
    state_installed: 'Установлено',
    state_available: 'Доступно к установке',
    state_manual: 'Ручная установка / просмотр'
  },
  en: {
    system: 'System & Administration',
    network: 'Network & Diagnostics',
    dns: 'DNS & Filtering',
    routing: 'Routing & Tunneling',
    proxy: 'Proxy',
    adaptation: 'Network Compatibility',
    automation: 'Automation & Services',
    security: 'Security',
    other: 'Other',
    state_updates: 'Updates available',
    state_installed: 'Installed',
    state_available: 'Available to install',
    state_manual: 'Manual / preview'
  }
};

function lang(locale) {
  return locale === 'en' ? 'en' : 'ru';
}

function moduleCategory(item) {
  const id = String(item?.id || '').toLowerCase();
  if (id === 'dns') return 'dns';
  if (['routerforge-core', 'admin', 'control'].includes(id)) return 'system';
  if (['monitoring', 'system', 'thermal', 'storage', 'network', 'network-tools'].includes(id)) return 'network';
  if (id.includes('nfqws') || id.includes('antiscan') || id.includes('dpi')) return 'adaptation';
  if (id.includes('vpn') || id.includes('wireguard') || id.includes('wg') || id.includes('route') || id.includes('routing')) return 'routing';
  if (id.includes('proxy')) return 'proxy';
  return 'system';
}

export function catalogCategoryKey(item) {
  const raw = String(item?.category || '').trim().toLowerCase();
  if (RAW_CATEGORY_MAP.has(raw)) return RAW_CATEGORY_MAP.get(raw);
  if (item?.kind === 'module') return moduleCategory(item);
  return 'other';
}

export function catalogCategoryLabel(item, locale = 'ru') {
  return LABELS[lang(locale)][catalogCategoryKey(item)] || LABELS[lang(locale)].other;
}

export function buildCatalogCategoryOptions(items = [], locale = 'ru') {
  const counts = new Map();
  for (const item of items) {
    const key = catalogCategoryKey(item);
    counts.set(key, Number(counts.get(key) || 0) + 1);
  }
  return CATEGORY_ORDER
    .filter((key) => counts.has(key))
    .map((key) => ({
      key,
      name: LABELS[lang(locale)][key] || LABELS[lang(locale)].other,
      count: counts.get(key)
    }));
}

function compareName(left, right, locale) {
  return String(left?.name || left?.id || '').localeCompare(
    String(right?.name || right?.id || ''),
    lang(locale) === 'en' ? 'en' : 'ru',
    { numeric: true, sensitivity: 'base' }
  );
}

function comparePublisher(left, right, locale) {
  const a = String(left?.publisher?.name || left?.source || '').trim();
  const b = String(right?.publisher?.name || right?.source || '').trim();
  return a.localeCompare(b, lang(locale) === 'en' ? 'en' : 'ru', { numeric: true, sensitivity: 'base' })
    || compareName(left, right, locale);
}

export function sortCatalogItems(items = [], mode = 'name', locale = 'ru') {
  const result = [...items];
  result.sort((left, right) => {
    if (mode === 'updates') {
      return Number(Boolean(right?.update_available)) - Number(Boolean(left?.update_available))
        || Number(Boolean(right?.installed)) - Number(Boolean(left?.installed))
        || compareName(left, right, locale);
    }
    if (mode === 'installed') {
      return Number(Boolean(right?.installed)) - Number(Boolean(left?.installed))
        || Number(Boolean(right?.update_available)) - Number(Boolean(left?.update_available))
        || compareName(left, right, locale);
    }
    if (mode === 'publisher') {
      return comparePublisher(left, right, locale);
    }
    if (mode === 'category') {
      return CATEGORY_ORDER.indexOf(catalogCategoryKey(left)) - CATEGORY_ORDER.indexOf(catalogCategoryKey(right))
        || compareName(left, right, locale);
    }
    return compareName(left, right, locale);
  });
  return result;
}

function stateKey(item) {
  if (item?.update_available) return 'state_updates';
  if (item?.installed) return 'state_installed';
  if (item?.actions?.install) return 'state_available';
  return 'state_manual';
}

function groupDescriptor(item, mode, locale) {
  const language = lang(locale);
  if (mode === 'category') {
    const key = catalogCategoryKey(item);
    return {
      key: `category:${key}`,
      label: LABELS[language][key] || LABELS[language].other,
      order: CATEGORY_ORDER.indexOf(key)
    };
  }
  if (mode === 'state') {
    const key = stateKey(item);
    const order = ['state_updates', 'state_installed', 'state_available', 'state_manual'].indexOf(key);
    return { key: `state:${key}`, label: LABELS[language][key], order };
  }
  if (mode === 'publisher') {
    const label = String(item?.publisher?.name || item?.source || (language === 'ru' ? 'Неизвестный издатель' : 'Unknown publisher')).trim();
    return { key: `publisher:${label.toLowerCase()}`, label, order: 0 };
  }
  return { key: 'all', label: '', order: 0 };
}

export function buildCatalogDisplayRows(items = [], groupMode = 'category', sortMode = 'name', locale = 'ru') {
  const sorted = sortCatalogItems(items, sortMode, locale);
  if (groupMode === 'none') {
    return sorted.map((item) => ({ type: 'item', key: `item:${item.id}`, item }));
  }

  const groups = new Map();
  for (const item of sorted) {
    const descriptor = groupDescriptor(item, groupMode, locale);
    let group = groups.get(descriptor.key);
    if (!group) {
      group = { ...descriptor, items: [] };
      groups.set(descriptor.key, group);
    }
    group.items.push(item);
  }

  const ordered = [...groups.values()].sort((left, right) => {
    if (groupMode === 'publisher') {
      return left.label.localeCompare(right.label, lang(locale) === 'en' ? 'en' : 'ru', { numeric: true, sensitivity: 'base' });
    }
    return left.order - right.order || left.label.localeCompare(right.label);
  });

  const rows = [];
  for (const group of ordered) {
    rows.push({ type: 'group', key: `group:${group.key}`, label: group.label, count: group.items.length });
    for (const item of group.items) {
      rows.push({ type: 'item', key: `item:${item.id}`, item });
    }
  }
  return rows;
}
