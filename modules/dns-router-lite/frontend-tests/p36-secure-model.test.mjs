import test from 'node:test';
import assert from 'node:assert/strict';
import {buildP36SecureDNS,hydrateP36} from '../frontend/DnsP36SecureModel.js';
const root='/opt/etc/routerforge/secure-dns/';
function config(){return {serviceListen:'192.168.10.1:55358',p36Enabled:true,p36DoT:'192.168.10.1:10853',p36DoH:'192.168.10.1:10443',p36DoQ:'192.168.10.1:10854',p36CertificateFile:root+'server.pem',p36PrivateKeyFile:root+'server.key',p36ClientCAFile:root+'clients.pem'};}
test('P36 disabled is inert and opt-in mTLS has no inline secrets',()=>{
  assert.equal(buildP36SecureDNS({p36Enabled:false}),null);
  const c=buildP36SecureDNS(config());
  assert.equal(c.enabled,true);
  assert.equal(c.doh_listen,'192.168.10.1:10443');
  assert.ok(!JSON.stringify(c).includes('BEGIN PRIVATE KEY'));
  const fields={};hydrateP36(fields,{secure_dns:c});
  assert.deepEqual(buildP36SecureDNS({...fields,serviceListen:'192.168.10.1:55358'}),c);
});
test('P36 refuses public bind, conflicts, missing mTLS CA and unsafe paths',()=>{
 for(const change of [
   {p36DoH:'8.8.8.8:10443'}, {p36DoT:'192.168.10.2:10853'},
   {p36DoH:'192.168.10.1:10853'}, {p36DoQ:'192.168.10.1:55358'},
   {p36DoT:'',p36DoH:'',p36DoQ:''}, {p36PrivateKeyFile:'/tmp/key'},
   {p36ClientCAFile:''}, {p36CertificateFile:root+'../another.pem'},
 ])assert.throws(()=>buildP36SecureDNS({...config(),...change}));
});
