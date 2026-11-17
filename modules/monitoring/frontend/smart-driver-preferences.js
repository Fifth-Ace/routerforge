// RouterForge Storage Doctor — browser-local preferences. No disk/SMART calls here.
export const SMART_DRIVER_CHOICES = Object.freeze([
  'auto', 'sat', 'sat,12', 'scsi', 'nvme', 'ata',
  'usbjmicron', 'usbcypress', 'usbsunplus', 'sntjmicron', 'sntasmedia', 'sntrealtek'
]);
export const SMART_DRIVER_STORAGE_KEY = 'routerforge.monitoring.smart-driver.v1';
const MAX_ENTRIES = 64;
const MAX_KEY_LENGTH = 256;
const ALLOWED_DEVICES = /^\/dev\/(sd[a-z]+|hd[a-z]+|vd[a-z]+|nvme\d+n\d+|mmcblk\d+)$/;

export function smartDriverDiskKey(disk) {
  if (!disk || typeof disk.device !== 'string' || !ALLOWED_DEVICES.test(disk.device)) return '';
  const id = String(disk.id || '').trim();
  if (id.startsWith('hw:') && id.length > 3 && id.length <= 170) return 'hw:' + id.slice(3);
  // A volatile Linux device path is never used ALONE: /dev/sdb may be another disk later.
  const model = String(disk.model || '').slice(0, 48);
  const vendor = String(disk.vendor || '').slice(0, 48);
  const size = Number(disk.size_bytes);
  if (!Number.isSafeInteger(size) || size <= 0) return '';
  const fingerprint = ['fallback', disk.device, size, vendor, model].join('|');
  return fingerprint.length <= MAX_KEY_LENGTH ? fingerprint : '';
}

export function readSmartDriverPreferences(storage) {
  try {
    const raw = storage?.getItem(SMART_DRIVER_STORAGE_KEY);
    if (!raw || raw.length > 20000) return {};
    const obj = JSON.parse(raw);
    if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return {};
    const valid = Object.entries(obj).filter(([key, driver]) =>
      typeof key === 'string' && key.length > 0 && key.length <= MAX_KEY_LENGTH &&
      SMART_DRIVER_CHOICES.includes(driver) && driver !== 'auto'
    ).slice(-MAX_ENTRIES);
    return Object.fromEntries(valid);
  } catch (_) {
    return {};
  }
}

export function getSmartDriverPreference(preferences, disk) {
  const key = smartDriverDiskKey(disk);
  const selected = key && preferences && Object.prototype.hasOwnProperty.call(preferences, key)
    ? preferences[key] : 'auto';
  return SMART_DRIVER_CHOICES.includes(selected) ? selected : 'auto';
}

export function setSmartDriverPreference(storage, current, disk, driver) {
  if (!SMART_DRIVER_CHOICES.includes(driver)) return { preferences: current, saved: false, reason: 'invalid_driver' };
  const key = smartDriverDiskKey(disk);
  if (!key) return { preferences: current, saved: false, reason: 'identity_unavailable' };
  const entries = Object.entries(current || {}).filter(([k,v]) => k !== key && SMART_DRIVER_CHOICES.includes(v) && v !== 'auto');
  if (driver !== 'auto') entries.push([key, driver]);
  const next = Object.fromEntries(entries.slice(-MAX_ENTRIES));
  let saved = false;
  try {
    storage?.setItem(SMART_DRIVER_STORAGE_KEY, JSON.stringify(next));
    saved = !!storage;
  } catch (_) { /* keep current-session choice, show explicit warning */ }
  return saved ? { preferences: next, saved: true } : { preferences: next, saved: false, reason: 'storage_unavailable' };
}
