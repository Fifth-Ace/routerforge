/* R06 Svelte-owned domain lists and strict DNS groups.
 * No implicit network requests, service activation or Keenetic mutations.
 * All local draft changes use R04 generation-bound CAS.
 */
const ID=/^[A-Za-z0-9_-]{1,40}$/;
const MATCH=new Set(['exact','suffix','subdomains']);
const KINDS=new Set(['file','paste','url']);
const ENCRYPTED=new Set(['dot','doh','doq']);
const INTERVALS=new Set([0,60,360,1440]);
const trim=x=>String(x??'').trim();
const fail=m=>{throw Error(m)};
function snapshot(readDraft){
  const v=readDraft();
  if(!v||!v.revision||!Number.isSafeInteger(v.generation)||v.generation<0||!v.config||
    !Array.isArray(v.config.upstreams)||!Array.isArray(v.config.rules))fail('Черновик DNS ещё не загружен');
  return v;
}
function stage(before,config,commit){
  return commit('edited',{previousRevision:before.revision,generation:before.generation,config});
}
function ids(text){return trim(text).split(',').map(x=>x.trim()).filter(Boolean)}
export function createPolicyManager({readDraft,commit,postPreview,confirmAction,notify,createId}){
  for(const fn of [readDraft,commit,postPreview,confirmAction,notify,createId]){
    if(typeof fn!=='function')fail('DNS_RUNTIME_POLICY_BRIDGE_MISSING');
  }
  function addGroup(values){
    const before=snapshot(readDraft),config=structuredClone(before.config);
    const id=trim(values.id),name=trim(values.name),servers=ids(values.servers);
    if(!ID.test(id)||!name||name.length>80)fail('Недопустимый ID или название DNS-группы');
    if((config.dns_groups||[]).some(g=>g.id===id))fail('ID DNS-группы уже существует');
    if(!servers.length||servers.length>8||new Set(servers).size!==servers.length)fail('Требуются 1–8 уникальных DNS-пулов');
    const pool=new Map(config.upstreams.map(u=>[u.id,u]));
    if(servers.some(server=>!pool.has(server)))fail('Все DNS-пулы группы должны существовать');
    const encrypted_only=!!values.encrypted_only;
    if(encrypted_only && servers.some(server=>!ENCRYPTED.has(pool.get(server).transport)))
      fail('Зашифрованная группа принимает только DoT/DoH/DoQ');
    config.dns_groups=[...(config.dns_groups||[]),{id,name,servers,encrypted_only}];
    const outcome=stage(before,config,commit);
    notify('DNS-группа создана в черновике; DNS не запускался');
    return outcome;
  }
  function removeGroup(id){
    const before=snapshot(readDraft),config=structuredClone(before.config);
    const old=(config.dns_groups||[]).find(g=>g.id===id);
    if(!old)fail('DNS-группа больше не существует');
    if(config.rules.some(r=>r.group_id===id)||(config.lists||[]).some(l=>l.group_id===id))
      fail('Группа используется правилами или списками');
    if(!confirmAction('Удалить неиспользуемую DNS-группу «'+old.name+'»?'))return {cancelled:true};
    config.dns_groups=config.dns_groups.filter(g=>g.id!==id);
    const outcome=stage(before,config,commit);
    notify('Группа удалена только из черновика');
    return outcome;
  }
  function removeList(id){
    const before=snapshot(readDraft),config=structuredClone(before.config);
    const old=(config.lists||[]).find(l=>l.id===id);
    if(!old)fail('Список больше не существует');
    if(!confirmAction('Удалить список «'+old.name+'» и все его правила из черновика?'))return {cancelled:true};
    config.lists=config.lists.filter(l=>l.id!==id);
    config.rules=config.rules.filter(r=>r.list_id!==id);
    const outcome=stage(before,config,commit);
    notify('Список и его правила удалены только из черновика');
    return outcome;
  }
  async function previewList(values,text,fileBytes=0){
    const before=snapshot(readDraft);
    // Capture scalar CAS before any async file/API work (do not trust mutable references).
    const baseRevision=before.revision,baseGeneration=before.generation;
    if(before.dirty)fail('Перед обновлением источника сохрани текущий черновик');
    const kind=trim(values.kind),name=trim(values.name),match=trim(values.match),group_id=trim(values.group_id);
    if(!KINDS.has(kind)||!name||name.length>80||!MATCH.has(match))fail('Проверь название, тип источника и совпадение');
    const priority=Number(values.priority);
    if(!Number.isSafeInteger(priority)||priority<0||priority>1000000)fail('Приоритет вне диапазона');
    const interval=kind==='url'?Number(values.auto_update_minutes||0):0;
    if(!INTERVALS.has(interval))fail('Неизвестный интервал обновления');
    const groups=before.config.dns_groups||[];
    const group=group_id?groups.find(g=>g.id===group_id):null;
    if(group_id&&!group)fail('DNS-группа больше не существует');
    const upstream=group?group.servers[0]:trim(values.upstream);
    if(!before.config.upstreams.some(u=>u.id===upstream))fail('DNS-пул не найден');
    const requested=trim(values.editId);
    const old=requested?(before.config.lists||[]).find(l=>l.id===requested):null;
    if(requested&&!old)fail('Редактируемый список больше не существует');
    const id=old?old.id:trim(createId());
    if(!id||id.length>80||(before.config.lists||[]).some(l=>l.id===id&&!old))fail('Не удалось выделить уникальный ID списка');
    const list={id,name,kind,upstream,match,priority,auto_update_minutes:interval};
    if(group_id)list.group_id=group_id;
    if(kind==='url'){
      const url=trim(values.url);
      if(url.length>2048||!/^https:\/\/[^\s/]+(?:\/[^\s]*)?$/i.test(url))fail('Для URL-списка нужен публичный HTTPS-адрес');
      list.url=url;
    }else{
      if(kind==='file'&&(fileBytes<=0||fileBytes>1048576))fail('TXT-файл должен иметь размер до 1 МиБ');
      if(!trim(text))fail('Список пуст');
      if(new TextEncoder().encode(String(text)).length>1048576)fail('Список превышает 1 МиБ');
    }
    const data=await postPreview({list,text:kind==='url'?'':String(text)});
    const now=snapshot(readDraft);
    if(now.revision!==baseRevision||now.generation!==baseGeneration||now.dirty||data.base_revision!==baseRevision)
      fail('Черновик изменён во время предпросмотра; повтори проверку');
    const p=data.preview;
    if(!p||!p.config||!Array.isArray(p.config.upstreams)||!Array.isArray(p.config.rules)||
      !Array.isArray(p.config.lists))fail('Сервер не вернул допустимый предпросмотр');
    return {revision:baseRevision,generation:baseGeneration,config:p.config,
      added:p.added,duplicates:p.duplicates,ignored:p.ignored,invalid:p.invalid,total_rules:p.total_rules,
      errors:Array.isArray(p.errors)?p.errors:[]};
  }
  function acceptList(pending){
    const now=snapshot(readDraft);
    if(!pending||now.dirty||pending.revision!==now.revision||pending.generation!==now.generation)
      fail('Предпросмотр устарел; перечитай и проверь список заново');
    if(!pending.config||!Array.isArray(pending.config.lists)||!Array.isArray(pending.config.rules))
      fail('Недопустимый предпросмотр');
    if(pending.invalid>0&&!confirmAction('Есть ошибки строк: '+pending.invalid+'. Добавить только корректные домены?'))
      return {cancelled:true};
    const outcome=stage(now,structuredClone(pending.config),commit);
    notify('Список добавлен в черновик. Применение DNS отдельно.');
    return outcome;
  }
  return {addGroup,removeGroup,removeList,previewList,acceptList};
}
