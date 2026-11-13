// Escaped markup for the native Svelte editor only; never accept raw HTML.
const esc=(s='')=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const nfqwsValueWords=new Set(['http','https','tls','quic','tcp','udp','wireguard','stun','discord','mtproto','unknown','fake','circular','multisplit','hostfakesplit','http_methodeol','tls_client_hello','quic_initial','http_req','MODE_LIST','MODE_ALL','MODE_AUTO']);
  const systemLists=new Set(['user.list','auto.list','exclude.list','ipset.list','ipset_exclude.list']);

  function syn(cls,value){return `<span class="${cls}">${esc(value)}</span>`}
  function highlightArgsLine(line){
    const text=String(line||''),trimmed=text.trimStart(),lead=text.slice(0,text.length-trimmed.length);
    if(trimmed.startsWith('#'))return esc(lead)+syn(/^#\s*(?:Strategy|Стратегия)\b/i.test(trimmed)?'syn-strategy':'syn-comment',trimmed);
    let out='',i=0;
    while(i<text.length){
      const c=text[i];
      if(/\s/.test(c)){let j=i+1;while(j<text.length&&/\s/.test(text[j]))j++;out+=esc(text.slice(i,j));i=j;continue}
      if(text.startsWith('--',i)){let j=i+2;while(j<text.length&&/[A-Za-z0-9_-]/.test(text[j]))j++;out+=syn('syn-flag',text.slice(i,j));i=j;continue}
      if(c==='$'){
        let j=i+1;if(text[j]==='{'){j++;while(j<text.length&&/[A-Za-z0-9_]/.test(text[j]))j++;if(text[j]==='}')j++}else while(j<text.length&&/[A-Za-z0-9_]/.test(text[j]))j++;
        out+=syn('syn-variable',text.slice(i,j));i=j;continue
      }
      if(c==='#'){out+=syn(/^#\s*(?:Strategy|Стратегия)\b/i.test(text.slice(i))?'syn-strategy':'syn-comment',text.slice(i));break}
      if((c==='@'&&text[i+1]==='/')||c==='/'){
        let j=i+1;while(j<text.length&&!/[\s,:]/.test(text[j]))j++;out+=syn('syn-resource',text.slice(i,j));i=j;continue
      }
      if(c==='"'||c==="'"){
        const q=c;let j=i+1,escaped=false;for(;j<text.length;j++){const x=text[j];if(x===q&&!escaped){j++;break}escaped=x==='\\'&&!escaped;if(x!=='\\')escaped=false}out+=syn('syn-string',text.slice(i,j));i=j;continue
      }
      if('=:,'.includes(c)){out+=syn('syn-operator',c);i++;continue}
      if(/[0-9]/.test(c)||((c==='-'||c==='+'||c==='<'||c==='>')&&/[A-Za-z0-9]/.test(text[i+1]||''))){
        let j=i+1;while(j<text.length&&/[A-Za-z0-9._+<>-]/.test(text[j]))j++;const v=text.slice(i,j);out+=syn(/^[-+<>]?[A-Za-z]*\d/.test(v)?'syn-number':'syn-value',v);i=j;continue
      }
      let j=i+1;while(j<text.length&&!/[\s=:,#"']/.test(text[j]))j++;const v=text.slice(i,j);
      let cls='syn-value';if(/^0x[0-9a-f]+$/i.test(v)||/^\d+(?:[-:]\d+)*(?:,\d+(?:[-:]\d+)*)*$/.test(v))cls='syn-number';else if(v.includes('/')||(/^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)+$/.test(v)&&/[A-Za-z]/.test(v)))cls='syn-resource';else if(text[j]==='=')cls='syn-param';else if(!nfqwsValueWords.has(v)&&!/^[A-Za-z_][A-Za-z0-9_.-]*$/.test(v))cls='syn-string';
      out+=syn(cls,v);i=j;
    }
    return out;
  }
  export function highlightCustomStrategy(text){return String(text||'').split(/\r?\n/).map(highlightArgsLine).join('\n')||'&#8203;'}
