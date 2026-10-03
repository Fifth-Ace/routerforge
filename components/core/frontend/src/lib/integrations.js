export function railInstalledIntegrations(integrations = []) {
  const groups = new Map();

  for (const item of integrations) {
    if (!item?.installed) continue;

    const projectURL = String(item?.project_url || '').trim().replace(/\/+$/, '').toLowerCase();
    const name = String(item?.name || item?.id || '').trim().toLowerCase();
    const key = projectURL ? `project:${projectURL}` : `name:${name}`;

    const current = groups.get(key);
    if (!current) {
      groups.set(key, { ...item });
      continue;
    }

    const preferred = Number(Boolean(item?.web)) + Number(Boolean(item?.managed))
      > Number(Boolean(current?.web)) + Number(Boolean(current?.managed))
      ? item
      : current;

    groups.set(key, {
      ...preferred,
      installed: true,
      update_available: Boolean(current?.update_available || item?.update_available),
      service_running: Boolean(current?.service_running || item?.service_running),
      version: preferred?.version || current?.version || item?.version || '',
      available_version:
        preferred?.available_version ||
        current?.available_version ||
        item?.available_version ||
        ''
    });
  }

  return [...groups.values()];
}

export function integrationProviders(modules = [], integrations = []) {
  const installedTargets = new Set(
    integrations.filter((item) => item?.installed).map((item) => item.id)
  );
  return modules
    .filter((item) => {
      if (!item?.installed) return false;
      const meta = item?.presentation?.integration;
      if (!meta?.enabled) return false;
      const targets = Array.isArray(meta.target_ids) ? meta.target_ids : [];
      return targets.some((id) => installedTargets.has(id));
    })
    .sort((a, b) =>
      Number(a?.presentation?.integration?.order || 50) -
      Number(b?.presentation?.integration?.order || 50)
    );
}

export function manageableExternalIntegrations(modules = [], integrations = []) {
  const providerTargets = new Set();
  for (const item of integrationProviders(modules, integrations)) {
    for (const id of item?.presentation?.integration?.target_ids || []) {
      providerTargets.add(id);
    }
  }

  return integrations
    .filter((item) =>
      item?.installed &&
      (Boolean(item?.web) || providerTargets.has(item.id))
    )
    .sort((a, b) => String(a?.name || a?.id).localeCompare(String(b?.name || b?.id)));
}

export function integrationHubAvailable(modules = [], integrations = []) {
  return integrationProviders(modules, integrations).length > 0 ||
    manageableExternalIntegrations(modules, integrations).length > 0;
}

export function catalogItemServiceNeedsAttention(item) {
  if (!item?.installed || item?.builtin || item?.service_running) return false;
  return Boolean(item?.service);
}
