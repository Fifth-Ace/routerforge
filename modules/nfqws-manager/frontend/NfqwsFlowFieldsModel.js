// The editable NFQWS Setup fields belong to Svelte, not imperative DOM reads.
// This model has no network or persistence side effects.
export const FLOW_TEXT_FIELDS = Object.freeze([
  'ISP_INTERFACE','NFQWS_BASE_ARGS','NFQWS_ARGS','NFQWS_ARGS_QUIC',
  'NFQWS_ARGS_UDP','NFQWS_ARGS_IPSET','TCP_PORTS','UDP_PORTS',
  'POLICY_NAME','LOG_DEBUG_PATH'
]);
export const FLOW_BOOL_FIELDS = Object.freeze(['IPV6_ENABLED','LOG_LEVEL']);
export const FLOW_UI_FIELDS = Object.freeze([...FLOW_TEXT_FIELDS,...FLOW_BOOL_FIELDS]);
export function hydrateFlowFields(source={}) {
  if(!source || typeof source!=='object' || Array.isArray(source))throw Error('NFQWS_FLOW_SOURCE_INVALID');
  return Object.fromEntries(FLOW_UI_FIELDS.map(k=>{
    const value=String(source[k]??'');
    return [k,value];
  }));
}
export function editFlowField(source,key,value) {
  if(!FLOW_UI_FIELDS.includes(key))throw Error('NFQWS_FLOW_FIELD_INVALID');
  if(typeof value!=='string')throw Error('NFQWS_FLOW_VALUE_INVALID');
  if(FLOW_BOOL_FIELDS.includes(key)&&!['0','1'].includes(value))throw Error('NFQWS_FLOW_BOOL_INVALID');
  return {...hydrateFlowFields(source),[key]:value};
}
export function mergeFlowFields(draft,fields,policy) {
  const clean=hydrateFlowFields(fields);
  return {...draft,...clean,POLICY_NAME:policy==='none'?'':clean.POLICY_NAME,
    POLICY_EXCLUDE:policy==='exclude'?'1':'0'};
}
