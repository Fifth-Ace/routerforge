// P92 text-only domain editor logic. Never calls a DNS/runtime API.
// Deliberately matches the pre-R12 p88ParseDomains policy until that owner is migrated.
export function parseDomainBatch(raw) {
  const found=[];
  const seen=new Set();
  const lines=String(raw||'').split(/\r?\n/);
  if(lines.length>500)return {error:'Слишком много строк. Для больших файлов используй расширенный импорт.'};
  for(const line of lines){
    const stripped=line.split('#')[0].trim();
    if(!stripped)continue;
    for(const item of stripped.split(/[\s,;]+/)){
      if(!item)continue;
      const domain=item.toLowerCase().replace(/\.$/,'');
      if(/^\d+(?:\.\d+){3}$/.test(domain) || domain.length>253 || !/^(?=.{1,253}$)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/.test(domain)){
        return {error:'Некорректный домен: '+item+'. Указывай обычные имена, без URL, IP, масок и ||-фильтров.'};
      }
      if(!seen.has(domain)){found.push(domain);seen.add(domain);}
      if(found.length>200)return {error:'Не больше 200 доменов за один раз. Большие списки добавляй в расширенном режиме.'};
    }
  }
  return {domains:found};
}
export function previewDomainBatch(raw) {
  const parsed=parseDomainBatch(raw);
  if(parsed.error)return {ok:false,count:'Проверь ввод',preview:parsed.error,error:parsed.error,domains:[]};
  const domains=parsed.domains;
  let preview=domains.length ? domains.slice(0,30).map((s,i)=>String(i+1)+'. '+s).join('\n') : 'Вставь домены выше — здесь появится предпросмотр.';
  if(domains.length>30)preview+='\n… ещё '+(domains.length-30);
  return {ok:true,count:domains.length+' доменов',preview,domains};
}
export function mergeDomainFile(existing,fileText,fileSize) {
  if(!Number.isFinite(fileSize)||fileSize<0||fileSize>256*1024)throw Error('Файл слишком большой (максимум 256 КБ).');
  const parsed=parseDomainBatch(String(existing||'')+'\n'+String(fileText||''));
  if(parsed.error)throw Error(parsed.error);
  return parsed.domains.join('\n');
}
export function exportDomainText(raw) {
  const parsed=parseDomainBatch(raw);
  if(parsed.error)throw Error(parsed.error);
  return parsed.domains.join('\n')+'\n';
}
