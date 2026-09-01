export async function getSnapshot() {
  const r = await fetch('/api/snapshot', { cache: 'no-store' });
  if (!r.ok) throw new Error(`snapshot HTTP ${r.status}`);
  return r.json();
}

export async function getHistory(minutes = 60) {
  const r = await fetch(`/api/history?minutes=${encodeURIComponent(minutes)}`, { cache: 'no-store' });
  if (!r.ok) throw new Error(`history HTTP ${r.status}`);
  return r.json();
}

export async function getSystem() {
  const r = await fetch('/api/system', { cache: 'no-store' });
  if (!r.ok) throw new Error(`system HTTP ${r.status}`);
  return r.json();
}

export async function getClients() { const r=await fetch('/api/clients',{cache:'no-store'}); if(!r.ok) throw new Error(`clients HTTP ${r.status}`); return r.json(); }
export async function getInterfaces() { const r=await fetch('/api/interfaces',{cache:'no-store'}); if(!r.ok) throw new Error(`interfaces HTTP ${r.status}`); return r.json(); }

export async function getClient(ip, limit=500) { const r=await fetch(`/api/client?ip=${encodeURIComponent(ip)}&limit=${encodeURIComponent(limit)}`,{cache:'no-store'}); if(!r.ok) throw new Error(`client HTTP ${r.status}`); return r.json(); }
