// Native NFQWS list editor policy. Only user-confirmed commands may mutate.
// The backend owns atomic writes, backup, SHA compare and rollback.
export const listShaPattern=/^[a-f0-9]{64}$/i;
const systems=new Set(['user.list','auto.list','exclude.list','ipset.list','ipset_exclude.list']);
export function safeListName(name){return typeof name==='string'&&/^[A-Za-z0-9][A-Za-z0-9._-]*\.list(?:-opkg|-old)?$/i.test(name)&&name.length<=160;}
export function newListName(stem){const name=String(stem??'').trim();if(!/^[a-z0-9_-]+$/i.test(name)||name.length>150)throw Error('NFQWS_LIST_INVALID_NEW_NAME');return `${name}.list`;}
export function renameListName(value){let name=String(value??'').trim();if(!/\.list$/i.test(name))name+='.list';if(!/^[A-Za-z0-9][A-Za-z0-9._-]*\.list$/i.test(name)||name.length>160)throw Error('NFQWS_LIST_INVALID_RENAME_NAME');return name;}
export function listGroup(item){const name=String(item?.name||'').toLowerCase();if(name.endsWith('.list-opkg'))return 'opkg';const base=name.endsWith('.list-old')?name.slice(0,-4):name;return systems.has(base)?'system':'user';}
export function sortedListInventory(items){const order={system:0,user:1,opkg:2};return (Array.isArray(items)?items:[]).filter(item=>safeListName(item?.name)).slice().sort((a,b)=>order[listGroup(a)]-order[listGroup(b)]||String(a.name).localeCompare(String(b.name),undefined,{numeric:true,sensitivity:'base'}));}
export function groupListInventory(items,search=''){
  const q=String(search||'').trim().toLowerCase(),all=sortedListInventory(items).filter(item=>item.name.toLowerCase().includes(q));
  return [['system','Системные'],['user','Пользовательские'],['opkg','OPKG']].map(([key,title])=>({key,title,items:all.filter(item=>listGroup(item)===key)})).filter(group=>group.items.length);
}
export function normalizeListEntries(value){const seen=new Set();return String(value??'').split(/\r?\n/).filter(line=>{const text=line.trim();if(!text||text.startsWith('#')||text.startsWith('//'))return true;if(seen.has(text))return false;seen.add(text);return true}).map(line=>{const text=line.trim();return !text||text.startsWith('#')||text.startsWith('//')?line:text}).join('\n');}
export function listStatistics(value){const text=String(value??''),rows=text.split(/\r?\n/);return{lines:rows.length,entries:rows.filter(line=>{const t=line.trim();return t&&!t.startsWith('#')&&!t.startsWith('//')}).length,bytes:new TextEncoder().encode(text).length};}
export function listHTMLHighlight(value){const escape=text=>String(text).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');const rows=String(value??'').split(/\r?\n/);return rows.map(line=>{const match=line.match(/^(\s*)(#|\/\/)(.*)$/);return match?escape(match[1])+'<span class="list-comment">'+escape(match[2]+match[3])+'</span>':escape(line)||'&#8203;'}).join('\n')+(String(value??'').endsWith('\n')?' ':'');}
export function validLoadedList(requested,response){if(!safeListName(requested)||!response||typeof response!=='object'||typeof response.content!=='string'||!listShaPattern.test(String(response.sha256||''))||response.name&&response.name!==requested)throw Error('NFQWS_LIST_BAD_READ_OR_SHA');return {...response,name:requested};}
export function listMutation(kind,reference,content){
  if(kind==='create'){return{path:'/list/create',body:{name:newListName(content),confirm:'NFQWS_LIST_CREATE'}};}
  const name=reference?.name;if(!safeListName(name)||!listShaPattern.test(String(reference?.sha256||'')))throw Error('NFQWS_LIST_INVALID_MUTATION_REFERENCE');
  if(kind==='save'){if(typeof content!=='string')throw Error('NFQWS_LIST_INVALID_CONTENT');return{path:'/list/save',body:{name,content,expected_sha256:reference.sha256,confirm:'NFQWS_LIST_SAVE'}};}
  if(kind==='delete'){if(reference.protected)throw Error('NFQWS_LIST_PROTECTED_DELETE');return{path:'/list/delete',body:{name,expected_sha256:reference.sha256,confirm:'NFQWS_LIST_DELETE'}};}
  throw Error('NFQWS_LIST_UNKNOWN_MUTATION');
}
export function checkedRenamePreview(reference,next,p){
  const newName=renameListName(next);
  if(!reference||!listShaPattern.test(String(reference.sha256||''))||!p?.ok||!p.preview_only||p.mutation_performed||p.old_name!==reference.name||p.new_name!==newName||p.old_list_sha256!==reference.sha256||p.new_name_available!==true||(p.config_changes!==undefined&&!Array.isArray(p.config_changes))||(p.config_changes||[]).length>6)throw Error('NFQWS_LIST_RENAME_PREVIEW_STALE_OR_INVALID');
  return{...p,config_changes:p.config_changes||[],old_name:reference.name,new_name:newName};
}
export function listRenameApply(plan,reference){
  if(!plan||!reference||!listShaPattern.test(String(reference.sha256||''))||reference.name!==plan.old_name||reference.sha256!==plan.old_list_sha256||reference.protected)throw Error('NFQWS_LIST_RENAME_STALE');
  return{path:'/list/rename-apply',body:{old_name:plan.old_name,new_name:plan.new_name,expected_sha256:plan.old_list_sha256,expected_config_changes:plan.config_changes,confirm:'ROUTERFORGE_LIST_RENAME_APPLY'}};
}
export function listSafeAPIPath(path,method='GET'){
  const url=String(path||'');
  if(/[\\\r\n]/.test(url)||url.includes('..'))throw Error('NFQWS_LIST_API_UNTRUSTED_ROUTE');
  if(method==='POST'){
    if(!['/list/save','/list/delete','/list/create','/list/rename-apply'].includes(url))throw Error('NFQWS_LIST_API_UNTRUSTED_METHOD');
  }else if(method==='GET'){
    if(url==='/status')return url;
    const index=url.indexOf('?');if(index<0)throw Error('NFQWS_LIST_API_UNTRUSTED_ROUTE');
    const prefix=url.slice(0,index),params=new URLSearchParams(url.slice(index+1));
    if(prefix==='/list'){
      if([...params.keys()].length!==1||!safeListName(params.get('name')))throw Error('NFQWS_LIST_API_INVALID_LIST');
    }else if(prefix==='/list/rename-preview'){
      const keys=[...params.keys()].sort();
      if(keys.join(',')!=='expected_sha256,new_name,old_name'||!safeListName(params.get('old_name'))||!renameListName(params.get('new_name'))||!listShaPattern.test(params.get('expected_sha256')||''))throw Error('NFQWS_LIST_API_INVALID_PREVIEW');
    }else throw Error('NFQWS_LIST_API_UNTRUSTED_ROUTE');
  }else throw Error('NFQWS_LIST_API_UNTRUSTED_METHOD');
  return url;
}
