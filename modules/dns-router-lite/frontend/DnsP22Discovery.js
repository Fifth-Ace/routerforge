// Pure Keenetic device inventory normalization for P21/P22 shared GET.
// Failed or untrusted identification is never treated as proof of identity.
export function normalizeDiscoveredDevices(rows) {
  if (!Array.isArray(rows)) return [];
  const deduped = new Map();
  for (const row of rows.slice(0, 4096)) {
    if (!row || typeof row.mac !== 'string' ||
        !/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(row.mac)) continue;
    const mac = row.mac.toLowerCase();
    const prior = deduped.get(mac);
    if (!prior || (!prior.trusted && row.trusted === true)) {
      deduped.set(mac, {
        mac, ip: String(row.ip || ''), name: String(row.name || ''),
        trusted: row.trusted === true
      });
    }
  }
  return [...deduped.values()].sort((a,b) =>
    Number(b.trusted) - Number(a.trusted) ||
    (a.name || a.mac).localeCompare(b.name || b.mac));
}
