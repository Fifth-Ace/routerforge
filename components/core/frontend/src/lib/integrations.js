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
