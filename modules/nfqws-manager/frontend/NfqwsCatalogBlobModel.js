// Pure projections and strict exact-source guard for NFQWS Menu catalog/blobs.
export function catalogVisible(items,filter='all',search=''){
 const rows=Array.isArray(items)?items:[],q=String(search||'').trim().toLowerCase();
 return rows.filter(x=>(filter==='all'||filter==='local'&&x.kind!=='upstream'||filter==='upstream'&&x.kind==='upstream')&&
   (!q||[x.name,x.path,x.source,x.ref,x.sha256].join(' ').toLowerCase().includes(q)));
}
export function blobSummary(files,dependencies){
 const list=Array.isArray(files)?files:[],deps=Array.isArray(dependencies)?dependencies:[];
 return {installed:list.length,required:deps.length,missing:deps.filter(x=>x.state!=='ok').length,referenced:list.filter(x=>x.referenced===true).length};
}
export function verifiedBlobManifest(row,name){
 if(!row||row.item?.kind!=='upstream'||!row.expected)throw Error('NFQWS_BLOB_MANIFEST_MISSING');
 const ref=String(row.item.ref||''),sha=String(row.expected.sha256||'').toLowerCase(),path=String(row.expected.path||'');
 const file=String(name||'');
 if(!/^[a-f0-9]{40}$/i.test(ref)||!/^[a-f0-9]{64}$/.test(sha)||
   !/^strategies\/blobs\/[A-Za-z0-9._-]+\.bin$/.test(path)||
   !/^[A-Za-z0-9._-]+\.bin$/.test(file)||path.split('/').pop()!==file)
   throw Error('NFQWS_BLOB_UNTRUSTED_MANIFEST');
 return {ref,sha,path,file};
}
