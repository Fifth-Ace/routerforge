// Two navigation namespaces are intentionally distinct. The literal leaf
// `strategies` is the Configuration page, while primary group `strategies`
// denotes the Strategy Library group. Never resolve a leaf through lastTab.
export const NFQWS_PRIMARY_GROUPS = Object.freeze({
  configuration: Object.freeze(['strategies','configs','config','lists','geo-assets']),
  strategies: Object.freeze(['strategy-library-v2','strategy-registry-v1','strategy-import']),
  menu: Object.freeze(['catalog','upstream-lists','blobs']),
  tools: Object.freeze(['preflight','dpi-detector','domain-v2','prodlab-v2','clienthello']),
  selection: Object.freeze(['observed-v1','auto-setup-v1','selector-v2','sandbox-v2','target-memory-v2']),
  maintenance: Object.freeze(['runs','logs','backups','runtime-health-v2','scripts','keenetic'])
});
export const NFQWS_PRIMARY_DEFAULTS = Object.freeze({
  configuration:'strategies', strategies:'strategy-library-v2', menu:'catalog',
  tools:'domain-v2', selection:'selector-v2', maintenance:'runs'
});
export function primaryForLeaf(leaf){
  if(leaf==='overview')return 'overview';
  for(const [group,tabs] of Object.entries(NFQWS_PRIMARY_GROUPS)){
    if(tabs.includes(leaf))return group;
  }
  throw Error('NFQWS_NAV_UNKNOWN_LEAF: '+String(leaf));
}
export function resolveLeafTab(leaf){primaryForLeaf(leaf);return leaf;}
export function resolvePrimaryTab(group,history=NFQWS_PRIMARY_DEFAULTS){
  if(!Object.hasOwn(NFQWS_PRIMARY_GROUPS,group))throw Error('NFQWS_NAV_UNKNOWN_GROUP: '+String(group));
  const leaf=history?.[group]||NFQWS_PRIMARY_DEFAULTS[group];
  if(!NFQWS_PRIMARY_GROUPS[group].includes(leaf))throw Error('NFQWS_NAV_INVALID_GROUP_HISTORY: '+group);
  return leaf;
}
