import ru from './ru.js';
import en from './en.js';

const dictionaries = { ru, en };

export function localeOf(value) {
  return value === 'en' ? 'en' : 'ru';
}

export function t(locale, key, vars = {}) {
  const normalized = localeOf(locale);
  const lookup = (dictionary) => String(key || '').split('.').reduce((value, part) => value?.[part], dictionary);
  let value = lookup(dictionaries[normalized]);
  if (value == null) value = lookup(dictionaries.ru);
  if (value == null) return key;
  return String(value).replace(/\{([a-zA-Z0-9_]+)\}/g, (_, name) => vars[name] ?? `{${name}}`);
}
