import { rfUi } from './rf-localization.js';
// P32 strictly offline DNS stamp parsing: no DNS lookup or network activity.
export function parseP32Stamp(value) {
  const stamp=String(value||'').trim();
  if (!/^sdns:\/\/[A-Za-z0-9_-]{14,4090}={0,2}$/.test(stamp)) throw Error(rfUi('Нужен корректный sdns:// DNS Stamp',"Enter a valid sdns:// DNS stamp"));
  let bytes;
  try {bytes=Uint8Array.from(atob(stamp.slice(7).replace(/-/g,'+').replace(/_/g,'/') .padEnd(Math.ceil((stamp.length-7)/4)*4,'=')), c=>c.charCodeAt(0));}
  catch (_) {throw Error(rfUi('Повреждённый DNS Stamp',"Corrupted DNS stamp"));}
  if (bytes.length<11 || ![1,2,3,4].includes(bytes[0])) throw Error(rfUi('Поддерживаются DNSCrypt, DoH, DoT и DoQ Stamps',"DNSCrypt, DoH, DoT, and DoQ stamps are supported"));
  const length=bytes[9];
  if (!length || bytes.length<=10+length) throw Error(rfUi('Stamp обязан содержать IP DNS-сервера',"The stamp must include the DNS server IP address"));
  if(bytes[0]!==1 && bytes[10+length]!==0) throw Error(rfUi('Stamp с TLS-пином требует отдельной проверки сертификата',"A stamp with a TLS pin requires separate certificate verification"));
  const raw=new TextDecoder('utf-8',{fatal:true}).decode(bytes.slice(10,10+length));
  const fallbackPort=bytes[0]===3||bytes[0]===4?'853':'443';
  let host=raw,port=fallbackPort;
  if (raw.startsWith('[')) {
    const m=raw.match(/^\[([^\]]+)\](?::(\d+))?$/);if(!m)throw Error(rfUi('Некорректный IPv6 Stamp',"Invalid IPv6 stamp"));host=m[1];port=m[2]||port;
  } else if (raw.includes(':')&&raw.split(':').length===2) {
    [host,port]=raw.split(':');
  }
  if(host.includes(':')) {
    if(!/^[0-9a-f:]+$/i.test(host))throw Error(rfUi('Некорректный IPv6 адрес в Stamp',"Invalid IPv6 address in stamp"));
    try {host=new URL('http://['+host+']/').hostname.slice(1,-1);} catch(_) {throw Error(rfUi('Некорректный IPv6 адрес в Stamp',"Invalid IPv6 address in stamp"));}
  } else if (!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(host)||host.split('.').some(x=>Number(x)>255||(/^0\d/.test(x)))) {
    throw Error(rfUi('DNS Stamp должен содержать IP, а не доменное имя',"DNS stamp must include an IP address, not a host name"));
  }
  if(Number(port)<1||Number(port)>65535)throw Error(rfUi('Некорректный порт DNS Stamp',"Invalid DNS stamp port"));
  const address=(host.includes(':')?'['+host+']':host)+':'+Number(port);
  return {address,host,transport:'stamp',stamp};
}
