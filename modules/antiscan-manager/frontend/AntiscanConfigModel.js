// Antiscan ascn.conf view-model mapping; does not submit, apply, restart or alter config.
function configNumber(cells, id) {
  const raw = String(cells[id].value ?? '').trim();
  return raw === '' ? 0 : Number(raw);
}

export function readAntiscanConfigForm(cells) {
  return {
    isp_interfaces: cells['cfgIspInterfaces'].value.trim(),
    ports: cells['cfgPorts'].value.trim(),
    forwarded_ports: cells['cfgForwardedPorts'].value.trim(),
    enable_honeypot: cells['cfgEnableHoneypot'].checked,
    honeypot_ports: cells['cfgHoneypotPorts'].value.trim(),
    honeypot_bantime_seconds: configNumber(cells, 'cfgHoneypotBanTime'),
    enable_ips_ban: cells['cfgEnableIpsBan'].checked,
    rules_mask: cells['cfgRulesMask'].value.trim(),
    recent_connections_time_seconds: configNumber(cells, 'cfgRecentTime'),
    recent_connections_hitcount: configNumber(cells, 'cfgRecentHitcount'),
    recent_connections_limit: configNumber(cells, 'cfgRecentLimit'),
    recent_connections_bantime_seconds: configNumber(cells, 'cfgRecentBanTime'),
    different_ip_candidates_storage_seconds: configNumber(cells, 'cfgCandidateStorage'),
    different_ip_threshold: configNumber(cells, 'cfgDifferentThreshold'),
    subnets_bantime_seconds: configNumber(cells, 'cfgSubnetBanTime'),
    ipsets_directory: cells['cfgIPSetsDirectory'].value.trim(),
    save_ipsets: cells['cfgSaveIPSets'].checked,
    save_on_exit: cells['cfgSaveOnExit'].checked,
    use_custom_exclude_list: cells['cfgUseCustomExclude'].checked,
    custom_lists_block_mode: cells['cfgCustomMode'].value,
    geoblock_mode: cells['cfgGeoMode'].value,
    geoblock_countries: cells['cfgGeoCountries'].value.trim(),
    geo_exclude_countries: cells['cfgGeoExcludeCountries'].value.trim(),
    read_ndm_lockout_ipsets: cells['cfgReadNDM'].checked,
    lockout_ipset_bantime_seconds: configNumber(cells, 'cfgLockoutBanTime')
  };
}

export function configPayloadFromSnapshot(cfg) {
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

export function writeAntiscanConfigForm(cells, cfg) {
  cells['cfgIspInterfaces'].value = cfg.isp_interfaces;
  cells['cfgPorts'].value = cfg.ports;
  cells['cfgForwardedPorts'].value = cfg.forwarded_ports;
  cells['cfgEnableHoneypot'].checked = cfg.enable_honeypot;
  cells['cfgHoneypotPorts'].value = cfg.honeypot_ports;
  cells['cfgHoneypotBanTime'].value = cfg.honeypot_bantime_seconds;
  cells['cfgEnableIpsBan'].checked = cfg.enable_ips_ban;
  cells['cfgRulesMask'].value = cfg.rules_mask;
  cells['cfgRecentTime'].value = cfg.recent_connections_time_seconds;
  cells['cfgRecentHitcount'].value = cfg.recent_connections_hitcount;
  cells['cfgRecentLimit'].value = cfg.recent_connections_limit;
  cells['cfgRecentBanTime'].value = cfg.recent_connections_bantime_seconds;
  cells['cfgCandidateStorage'].value = cfg.different_ip_candidates_storage_seconds;
  cells['cfgDifferentThreshold'].value = cfg.different_ip_threshold;
  cells['cfgSubnetBanTime'].value = cfg.subnets_bantime_seconds;
  cells['cfgIPSetsDirectory'].value = cfg.ipsets_directory;
  cells['cfgSaveIPSets'].checked = cfg.save_ipsets;
  cells['cfgSaveOnExit'].checked = cfg.save_on_exit;
  cells['cfgUseCustomExclude'].checked = cfg.use_custom_exclude_list;
  cells['cfgCustomMode'].value = cfg.custom_lists_block_mode;
  cells['cfgGeoMode'].value = cfg.geoblock_mode;
  cells['cfgGeoCountries'].value = cfg.geoblock_countries;
  cells['cfgGeoExcludeCountries'].value = cfg.geo_exclude_countries;
  cells['cfgReadNDM'].checked = cfg.read_ndm_lockout_ipsets;
  cells['cfgLockoutBanTime'].value = cfg.lockout_ipset_bantime_seconds;
}

