<script>
  import { rfUi } from './rf-localization.js';
 import {untrack} from 'svelte';
 let {active=false,onRequest=async()=>{},onNotice=()=>{},onListsChanged=()=>{}}=$props();
 let assets=$state([]), selected=$state(''), category=$state(''), kind=$state('geosite');
 let source=$state(''),sourceRef=$state(''),listName=$state(''),file=$state(null);
 let preview=$state(''), autohostlist=$state(null), error=$state(''), busy=$state(false), loading=$state(false);
 let selectedAsset=$derived(assets.find(x=>x.name===selected)||null);
 let categories=$derived(Array.isArray(selectedAsset?.categories)?selectedAsset.categories:[]);
 let epoch=0, controller=null;
 const safeListName=name=>`geo-${String(name||'').replace(/[^A-Za-z0-9._-]+/g,'-')}.list`;
 function assetChanged(){const asset=assets.find(x=>x.name===selected);source=String(asset?.source||'');sourceRef=String(asset?.source_ref||'');category=String(asset?.categories?.[0]?.name||'');listName=category?safeListName(category):'';preview='';}
 function categoryChanged(){listName=category?safeListName(category):'';preview='';}
 async function refresh(){
   if(!active||loading||busy)return;
   const n=++epoch,c=new AbortController();controller=c;loading=true;error='';
   try{
     const d=await onRequest('/v2/geo/assets',{method:'GET',signal:c.signal});
     if(n!==epoch||c.signal.aborted||!active)return;
     assets=Array.isArray(d?.assets)?d.assets:[];
     if(!assets.some(x=>x.name===selected)){selected=assets[0]?.name||'';assetChanged();}
     const a=await onRequest('/v2/geo/autohostlist',{method:'GET',signal:c.signal});
     if(n===epoch&&!c.signal.aborted&&active)autohostlist=a;
   }catch(e){if(n===epoch&&!c.signal.aborted&&active)error=String(e?.message||e);}
   finally{if(n===epoch){controller=null;loading=false;}}
 }
 async function sha256(buffer){const digest=await crypto.subtle.digest('SHA-256',buffer);return Array.from(new Uint8Array(digest)).map(x=>x.toString(16).padStart(2,'0')).join('');}
 function base64(buffer){let output='',b=new Uint8Array(buffer);for(let i=0;i<b.length;i+=0x8000)output+=String.fromCharCode(...b.subarray(i,i+0x8000));return btoa(output);}
 async function command(name){
   if(!active||busy||loading)return;
   busy=true;error='';
   try{
     if(name==='upload'){
       if(!file)throw Error(rfUi('Выберите GeoSite/GeoIP файл',"Choose a GeoSite/GeoIP file"));
       if(file.size>24*1024*1024)throw Error(rfUi('Файл больше 24 МиБ',"File exceeds 24 MiB"));
       if(!confirm(`Загрузить ${file.name} (${file.size} байт)?\n\nProduction config и NFQWS service не изменяются.`))return;
       const buffer=await file.arrayBuffer(),hash=await sha256(buffer);
       const d=await onRequest('/v2/geo/upload',{method:'POST',body:JSON.stringify({name:file.name,kind,content_base64:base64(buffer),expected_sha256:hash,source:source.trim(),source_ref:sourceRef.trim(),confirm:'ROUTERFORGE_GEO_UPLOAD'})});
       onNotice(`Geo ${d.asset?.name||file.name}: сохранено; production untouched.`,'good');file=null;
     }else if(name==='provenance'){
       if(!selectedAsset)throw Error(rfUi('Выберите asset',"Choose an asset"));
       if(!confirm(`Обновить provenance ${selected}?\n\nAsset bytes, lists и production config не меняются.`))return;
       await onRequest('/v2/geo/source',{method:'POST',body:JSON.stringify({name:selected,source:source.trim(),source_ref:sourceRef.trim(),source_url:selectedAsset.source_url||'',confirm:'ROUTERFORGE_GEO_SOURCE'})});
       onNotice(`Geo ${selected}: provenance обновлён.`,'good');
     }else if(name==='preview'){
       if(!selectedAsset||!category)throw Error(rfUi('Выберите asset и категорию',"Choose an asset and category"));
       const d=await onRequest('/v2/geo/preview',{method:'POST',body:JSON.stringify({name:selected,category,limit:500})});
       preview=`${selected}:${category}\n${d.count}/${d.total}${d.truncated?' (preview truncated)':''}\n\n${(d.entries||[]).join('\n')}`;
     }else if(name==='save'){
       const target=listName.trim();if(!selectedAsset||!category||!target)throw Error(rfUi('Выберите asset, категорию и имя списка',"Choose an asset, category, and list name"));
       const inventory=await onRequest('/list-sources',{method:'GET'});
       const existing=(Array.isArray(inventory?.files)?inventory.files:[]).some(x=>x.name===target);
       if(!confirm(`Geo ${selected}:${category} → ${target}?\n\n${existing?'Existing list will be backed up and replaced.':'New list will be created.'}\nReload/restart: NO`))return;
       const d=await onRequest('/v2/geo/save-list',{method:'POST',body:JSON.stringify({name:selected,category,list_name:target,limit:20000,replace:existing,confirm:existing?'ROUTERFORGE_GEO_LIST_REPLACE':'ROUTERFORGE_GEO_LIST_CREATE'})});
       onNotice(`Geo list ${target}: ${d.count} entries · production config unchanged.`,'good');onListsChanged();
     }
   }catch(e){error=String(e?.message||e);onNotice(`Geo ${name}: ${error}`,'error');}
   finally{busy=false;if(active&&(name==='upload'||name==='provenance'))void refresh();}
 }
 $effect(()=>{
   if(active)untrack(()=>void refresh());
   else{epoch++;controller?.abort();controller=null;loading=false;}
   return()=>{epoch++;controller?.abort();controller=null;loading=false;};
 });
</script>
<div class="blob-shell" data-svelte-owned="nfqws-geo-assets">
 <article class="panel">
  <div class="panel-head"><div><h2>{rfUi("Геоданные","Geo data")}</h2><div class="section-kicker">{rfUi("GeoSite / GeoIP · локальные assets и категории","GeoSite / GeoIP · local assets and categories")}</div></div><div class="toolbar"><button disabled={busy||loading} onclick={()=>void refresh()}>{rfUi("Обновить","Refresh")}</button></div></div>
  <div class="setup-note">{rfUi("Geo assets и списки сохраняются отдельно. NFQWS не перезапускается, активная конфигурация не меняется.","Geo assets and lists are saved independently. NFQWS is not restarted and active configuration is unchanged.")}</div>
  {#if error}<div class="notice error" role="alert">{error}</div>{/if}
  <div class="muted small">{loading?'Загрузка…':autohostlist?`Autohostlist: ${autohostlist.supported?'поддерживается':'не подтверждён'} · ${autohostlist.configured?'настроен':'не настроен'}`:'Autohostlist: нет данных'}</div>
  <div class="blob-list" id="geoAssets">
   {#if !assets.length}<div class="empty">{loading?'Загружаем Geo assets…':'Geo assets ещё не загружены.'}</div>{/if}
   {#each assets as asset (asset.name)}<div class="blob-row"><div><div class="blob-name">{asset.name} <span class="config-badge">{String(asset.kind||'').toUpperCase()}</span></div><div class="blob-meta">{Number(asset.size||0)} B · sha256 {String(asset.sha256||'').slice(0,16)}… · {asset.categories?.length||0} {rfUi("категорий","categories")}</div><div class="blob-refs">{asset.source||'локальный файл'}{asset.source_ref?` · ref ${asset.source_ref}`:''}</div></div></div>{/each}
  </div>
  <div class="backup-center-layout">
   <div class="panel"><h3>{rfUi("Загрузка Geo asset","Upload Geo asset")}</h3><div class="field"><label for="geoFile">{rfUi("Файл","File")}</label><input id="geoFile" type="file" onchange={e=>file=e.currentTarget.files?.[0]||null}/></div><div class="field"><label for="geoKind">{rfUi("Тип","Type")}</label><select id="geoKind" bind:value={kind}><option value="geosite">GeoSite</option><option value="geoip">GeoIP</option></select></div><button id="geoUpload" disabled={busy||loading||!file} onclick={()=>void command('upload')}>{rfUi("Загрузить с SHA256","Upload with SHA256")}</button></div>
   <div class="panel"><h3>{rfUi("Asset и provenance","Asset and provenance")}</h3><div class="field"><label for="geoAssetSelect">Asset</label><select id="geoAssetSelect" bind:value={selected} onchange={assetChanged}><option value="">{rfUi("Выбрать","Select")}</option>{#each assets as asset (asset.name)}<option value={asset.name}>{asset.name} · {asset.kind}</option>{/each}</select></div><div class="field"><label for="geoSource">{rfUi("Источник","Source")}</label><input id="geoSource" bind:value={source}/></div><div class="field"><label for="geoSourceRef">Source ref</label><input id="geoSourceRef" bind:value={sourceRef}/></div><button id="geoSaveProvenance" disabled={busy||loading||!selectedAsset} onclick={()=>void command('provenance')}>{rfUi("Сохранить provenance","Save source information")}</button></div>
  </div>
  <div class="panel"><h3>{rfUi("Категория → локальный список","Category → local list")}</h3><div class="field"><label for="geoCategorySelect">{rfUi("Категория","Category")}</label><select id="geoCategorySelect" bind:value={category} onchange={categoryChanged}><option value="">{rfUi("Выбрать категорию","Select category")}</option>{#each categories as c (c.name)}<option value={c.name}>{c.name} · {c.count||0}</option>{/each}</select></div><div class="field"><label for="geoListName">{rfUi("Имя списка","List name")}</label><input id="geoListName" bind:value={listName}/></div><div class="toolbar"><button id="geoPreview" disabled={busy||loading||!category} onclick={()=>void command('preview')}>{rfUi("Предпросмотр","Preview")}</button><button id="geoSaveList" disabled={busy||loading||!category||!listName.trim()} onclick={()=>void command('save')}>{rfUi("Сохранить список","Save list")}</button></div><pre id="geoPreviewResult" style="white-space:pre-wrap;overflow:auto;max-height:320px">{preview}</pre></div>
 </article>
</div>
