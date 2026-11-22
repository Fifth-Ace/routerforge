// Pure NFQWS domain and Keenetic policy draft mutations; no IO, no apply.
export const DOMAIN_MODES = Object.freeze(['MODE_LIST','MODE_AUTO','MODE_ALL']);
export const POLICY_MODES = Object.freeze(['none','include','exclude']);
export function domainMode(values) {
  return String(values?.NFQWS_EXTRA_ARGS ?? '').replace(/^\$/,'').replace(/[{}]/g,'');
}
export function policyMode(values) {
  if(!String(values?.POLICY_NAME ?? '').trim())return 'none';
  return /^(1|true|yes)$/i.test(String(values?.POLICY_EXCLUDE ?? ''))?'exclude':'include';
}
export function updateModeDraft(values,kind,mode) {
  if(!values || typeof values!=='object' || Array.isArray(values))throw Error('NFQWS_MODE_CONFIG_INVALID');
  const next={...values};
  if(kind==='domain'){
    if(!DOMAIN_MODES.includes(mode))throw Error('NFQWS_DOMAIN_MODE_INVALID');
    next.NFQWS_EXTRA_ARGS='$'+mode;
  }else if(kind==='policy'){
    if(!POLICY_MODES.includes(mode))throw Error('NFQWS_POLICY_MODE_INVALID');
    if(mode==='none')next.POLICY_NAME='';
    next.POLICY_EXCLUDE=mode==='exclude'?'1':'0';
  }else throw Error('NFQWS_MODE_KIND_UNKNOWN');
  return next;
}
