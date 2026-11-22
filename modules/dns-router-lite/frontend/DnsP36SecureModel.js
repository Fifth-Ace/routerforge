// P36 pure local settings mapper. Never reads certificates, keys or network.
export const P36_CERT_ROOT='/opt/etc/routerforge/secure-dns/';
function path(value, label){
  const s=String(value||'').trim();
  if(!s.startsWith(P36_CERT_ROOT)||s===P36_CERT_ROOT||s.includes('..')||s.includes('\\')||/\s/.test(s)||s.includes('\0'))
    throw Error('P36 '+label+': путь должен находиться в '+P36_CERT_ROOT);
  return s;
}
function listen(value, label, plain){
  const s=String(value||'').trim();
  if(!s)return '';
  const re=/^(\d{1,3}(?:\.\d{1,3}){3}):(\d{4,5})$/;
  const m=s.match(re);
  if(!m||Number(m[2])<1025||Number(m[2])>65535||m[1]!==plain.split(':')[0])
    throw Error('P36 '+label+': укажи адрес DNS-сервиса и отдельный порт 1025..65535');
  return s;
}
export function buildP36SecureDNS(fields){
  if(!fields.p36Enabled)return null;
  const plain=String(fields.serviceListen||'');
  const dot_listen=listen(fields.p36DoT,'DoT',plain);
  const doh_listen=listen(fields.p36DoH,'DoH',plain);
  const doq_listen=listen(fields.p36DoQ,'DoQ',plain);
  const active=[dot_listen,doh_listen,doq_listen].filter(Boolean);
  if(!active.length || new Set([...active,plain]).size!==active.length+1)
    throw Error('P36: включи хотя бы один протокол, порты не должны совпадать');
  const certificate_file=path(fields.p36CertificateFile,'сертификат');
  const private_key_file=path(fields.p36PrivateKeyFile,'приватный ключ');
  const client_ca_file=path(fields.p36ClientCAFile,'CA клиентов');
  if(new Set([certificate_file,private_key_file,client_ca_file]).size!==3)
    throw Error('P36: файлы сертификата, ключа и CA должны различаться');
  return {enabled:true,dot_listen,doh_listen,doq_listen,certificate_file,private_key_file,client_ca_file};
}
export function hydrateP36(fields, service){
  const c=service?.secure_dns||{};
  fields.p36Enabled=Boolean(c.enabled);
  fields.p36DoT=c.dot_listen||'';
  fields.p36DoH=c.doh_listen||'';
  fields.p36DoQ=c.doq_listen||'';
  fields.p36CertificateFile=c.certificate_file||'';
  fields.p36PrivateKeyFile=c.private_key_file||'';
  fields.p36ClientCAFile=c.client_ca_file||'';
}
