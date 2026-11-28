import { writable, get } from 'svelte/store';
import { getAppTransactionStatus, previewAppTransaction, startAppTransaction } from '$lib/api.js';
import { previewSelection, executionRequest, updateRunning } from '$lib/transaction-preview.js';
import { refreshCatalog } from '$lib/stores/catalog.js';

export const appUpdates = writable({snapshot:null,running:false,launching:false,open:false,stale:false,error:'',pending:[]});
let sequence = 0;
export function openUpdateProgress(){appUpdates.update(s=>({...s,open:true}));}
export function closeUpdateProgress(){appUpdates.update(s=>({...s,open:false}));}
export async function refreshUpdateProgress(signal){
 if(get(appUpdates).launching) return;
 const ticket=++sequence;
 try {
  const next=await getAppTransactionStatus(signal,true);
  if(ticket!==sequence || signal?.aborted || get(appUpdates).launching)return;
  const previous=get(appUpdates);
  const running=updateRunning(next?.slot);
  appUpdates.update(s=>({...s,snapshot:next,running,stale:false,open:s.open || (!s.snapshot && running)}));
  if(previous.running && !running) void refreshCatalog();
 }catch(error){
  if(ticket!==sequence || signal?.aborted)return;
  appUpdates.update(s=>({...s,stale:true}));
 }
}
export async function startOfficialUpdates(names){
 const current=get(appUpdates);
 if(current.launching || current.running) throw new Error('Update already in progress');
 if(!names.length || names.some(n=>typeof n!=='string') || new Set(names).size!==names.length)throw new Error('Invalid package selection');
 ++sequence;
 appUpdates.update(s=>({...s,launching:true,open:true,error:'',pending:[...names]}));
 let submitted=false;
 try {
  const snap=await getAppTransactionStatus();
  if(!names.every(n=>previewSelection(snap).some(row=>row.package===n)))throw new Error('Release changed; check updates again');
  const preview=await previewAppTransaction(snap.release_commit,names);
  const body=executionRequest(snap,preview,names);
  if(!body)throw new Error('Update unavailable; refresh catalog and try again');
  submitted=true;
  const result=await startAppTransaction(body);
  if(result?.id!==body.id)throw new Error('Update start was not confirmed');
  appUpdates.update(s=>({...s,snapshot:{...snap,slot:{state:'queued',plan:preview.plan}},running:true,stale:false}));
 }catch(error){
  appUpdates.update(s=>({...s,error:submitted && !error?.status?'START_UNCONFIRMED':String(error?.payload?.error || error?.message || 'Update failed')}));
  throw error;
 }finally{
  appUpdates.update(s=>({...s,launching:false,pending:[]}));
  await refreshUpdateProgress();
 }
}
