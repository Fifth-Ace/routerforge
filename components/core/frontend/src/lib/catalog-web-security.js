// Web metadata trust and package lifecycle trust are separate concerns.
// Unverified manifests must not gain installation authority, but a package that
// is already installed locally may expose a probed local Web UI.
export function catalogWebProbeMetadataEligible(item = {}, runtimeLocal = false) {
  const web = item?.web || {};
  const trustStatus = String(item?.trust?.status || '').toLowerCase();

  if (trustStatus === 'official' || trustStatus === 'verified') return true;
  if (runtimeLocal) return true;

  if (item?.registry_source === 'legacy-fallback' && Boolean(item?.web_port_source)) {
    return true;
  }

  return item?.package_installed === true && web.mode === 'probe-required';
}
