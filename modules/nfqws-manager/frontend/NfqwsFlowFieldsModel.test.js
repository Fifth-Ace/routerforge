import test from 'node:test';
import assert from 'node:assert/strict';
import {hydrateFlowFields,editFlowField,mergeFlowFields,FLOW_UI_FIELDS} from './NfqwsFlowFieldsModel.js';
test('all NFQWS setup form fields hydrate in one immutable Svelte snapshot',()=>{
 const src={NFQWS_ARGS:'--filter-tcp=443',TCP_PORTS:'443',IPV6_ENABLED:'yes',LOG_LEVEL:'false',OTHER:'keep'};
 const out=hydrateFlowFields(src);assert.equal(FLOW_UI_FIELDS.length,12);assert.equal(out.NFQWS_ARGS,src.NFQWS_ARGS);assert.equal(out.IPV6_ENABLED,'yes');assert.equal(out.LOG_LEVEL,'false');assert.equal(src.OTHER,'keep');
});
test('Svelte field edits never change saved draft or other keys',()=>{
 const before=hydrateFlowFields({TCP_PORTS:'80'}),after=editFlowField(before,'TCP_PORTS','80,443');
 assert.equal(before.TCP_PORTS,'80');assert.equal(after.TCP_PORTS,'80,443');assert.equal(after.UDP_PORTS,'');
});
test('save serialization preserves unknown metadata, MODE_* and selected policy',()=>{
 const draft={MODE_LIST:'original',KEEP:'keep',POLICY_EXCLUDE:'0'};
 const fields=hydrateFlowFields({NFQWS_ARGS:'--dpi-desync=fake',POLICY_NAME:'Corporate'});
 const out=mergeFlowFields(draft,fields,'exclude');assert.equal(out.MODE_LIST,'original');assert.equal(out.KEEP,'keep');assert.equal(out.POLICY_EXCLUDE,'1');assert.equal(out.POLICY_NAME,'Corporate');
 assert.equal(mergeFlowFields(draft,fields,'none').POLICY_NAME,'');
});
test('hydration preserves exact legacy boolean representation until explicit user change',()=>{
 const old=hydrateFlowFields({IPV6_ENABLED:'',LOG_LEVEL:'yes'}); assert.equal(old.IPV6_ENABLED,'');assert.equal(old.LOG_LEVEL,'yes');
 const changed=editFlowField(old,'IPV6_ENABLED','1');assert.equal(changed.LOG_LEVEL,'yes');
});
test('unknown fields and nonstring values fail before any persistence',()=>{
 assert.throws(()=>editFlowField(hydrateFlowFields(), 'NFQWS_EXTRA_ARGS', '$MODE_ALL'), /FIELD_INVALID/);
 assert.throws(()=>editFlowField(hydrateFlowFields(), 'ISP_INTERFACE', {}), /VALUE_INVALID/);
 assert.throws(()=>editFlowField(hydrateFlowFields(), 'LOG_LEVEL', '1;rm'), /BOOL_INVALID/);
});
