// The server owns eligibility, package identity and release commit.
export function previewSelection(snapshot) {
  const commit = snapshot?.release_commit;
  if (typeof commit !== 'string' || !/^[a-f0-9]{40}$/.test(commit)) return [];
  const rows = Array.isArray(snapshot?.eligible_packages) ? snapshot.eligible_packages : [];
  const seen = new Set();
  return rows.filter((row) => {
    if (typeof row?.package !== 'string' || !/^routerforge-[a-z0-9-]+$/.test(row.package) || seen.has(row.package)) return false;
    seen.add(row.package);
    return typeof row.installed_version === 'string' && typeof row.target_version === 'string';
  });
}
export function selectionKey(snapshot) {
  return JSON.stringify([snapshot?.release_commit || '', previewSelection(snapshot).map((row) => [row.package, row.installed_version, row.target_version])]);
}
export function previewMatches(preview, commit, selected) {
  if (typeof preview?.execution_available !== 'boolean' || preview?.plan?.commit !== commit || preview?.plan?.channel !== 'dev' || preview?.plan?.target !== 'aarch64-3.10' || !/^[a-f0-9]{64}$/.test(preview?.plan_sha256 || '')) return false;
  if (!Array.isArray(preview?.plan?.packages)) return false;
  const names = preview.plan.packages.map((p) => p?.name);
  return Array.isArray(names) && names.length === selected.length && new Set(names).size === names.length && names.every((name) => selected.includes(name));
}
export function transactionProgress(slot) {
  const total = slot?.plan?.packages?.length;
  const verified = slot?.journal?.next;
  if (!Number.isInteger(total) || total < 1 || !Number.isInteger(verified) || verified < 0 || verified > total) return null;
  return { verified, total };
}

// Only an explicitly admitted preview may be submitted. The server rebuilds
// the exact plan under its lease; the client cannot supply executable metadata.
export function executionRequest(snapshot, preview, selected) {
  if (snapshot?.execution_available !== true || preview?.execution_available !== true || !previewMatches(preview,snapshot.release_commit,selected) || !/^[a-f0-9]{32}$/.test(preview.plan.id || '') || !['idle','completed'].includes(snapshot?.slot?.state)) return null;
  if (selected.some((name) => !previewSelection(snapshot).some((row) => row.package === name && row.target_version === preview.plan.packages.find((p) => p.name === name)?.version))) return null;
  if (!Array.isArray(preview.management) || preview.management.length !== selected.length || selected.some((name) => !preview.management.some((row) => row.package === name && row.state === 'management_ready'))) return null;
  return { id:preview.plan.id, expected_commit:preview.plan.commit, packages:[...selected], plan_sha256:preview.plan_sha256, confirm:'EXECUTE:'+preview.plan_sha256 };
}
