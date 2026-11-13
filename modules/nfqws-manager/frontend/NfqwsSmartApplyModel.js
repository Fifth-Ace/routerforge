// Read-only plan projection; NEVER authorizes Apply by itself. The command
// revalidates active SHA and sends the server's explicit transactional token.
export function nfqwsSmartApplyView(view={}) {
  const plan=view.plan||null;
  const writes=plan?[...(plan.lists||[]),...(plan.blobs||[])].filter(x=>x.state==='write').length:0;
  const blocked=plan?.blocked||[];
  return {
    open:Boolean(view.open),busy:Boolean(view.busy),preparing:Boolean(view.preparing),
    error:String(view.error||''),plan,writes,blocked,
    canApply:Boolean(view.open && plan && !view.busy && !view.preparing && blocked.length===0 &&
      typeof plan.expectedActiveSha==='string'&&/^[0-9a-f]{64}$/i.test(plan.expectedActiveSha)&&
      typeof plan.configSha==='string'&&/^[0-9a-f]{64}$/i.test(plan.configSha)),
    source:plan?`${String(plan.item?.source||'local')} · ${String(plan.item?.name||'')} · ref ${String(plan.ref||'').slice(0,12)}`:'—'
  };
}
