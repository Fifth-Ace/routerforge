import { writable } from 'svelte/store';
import { getSnapshot } from '$lib/api.js';

const empty = {
  upstreams: [],
  flow: [],
  errors: [],
  top_domains: [],
  fallback_edges: [],
  fallback_edges_5m: [],
  fallback_edges_1h: [],
  fallback_edges_24h: [],
  error_bursts: []
};

export const snapshot = writable({ ...empty });
export const backendOnline = writable(false);
export const streamMode = writable('connecting');

export function startSnapshotStream(intervalMs = 2000) {
  let closed = false;
  let eventSource = null;
  let fallbackTimer = null;
  const interval = Math.max(1000, Math.min(30000, Number(intervalMs || 2000)));

  const publish = (data) => {
    if (!data || typeof data !== 'object') return;
    snapshot.set({ ...empty, ...data });
    backendOnline.set(true);
  };

  const fetchOnce = async () => {
    try {
      publish(await getSnapshot());
      return true;
    } catch {
      backendOnline.set(false);
      return false;
    }
  };

  const stopFallback = () => {
    if (!fallbackTimer) return;
    clearInterval(fallbackTimer);
    fallbackTimer = null;
  };

  const startFallback = () => {
    if (fallbackTimer || closed) return;
    streamMode.set('polling');
    fallbackTimer = setInterval(fetchOnce, interval);
  };

  fetchOnce();

  if (typeof EventSource === 'undefined') {
    startFallback();
  } else {
    eventSource = new EventSource(`/api/events?interval_ms=${encodeURIComponent(interval)}`);
    eventSource.onopen = () => {
      backendOnline.set(true);
      streamMode.set('sse');
      stopFallback();
    };
    eventSource.addEventListener('snapshot', (event) => {
      try {
        publish(JSON.parse(event.data));
        streamMode.set('sse');
        stopFallback();
      } catch {}
    });
    eventSource.onerror = () => {
      backendOnline.set(false);
      startFallback();
    };
  }

  return () => {
    closed = true;
    stopFallback();
    eventSource?.close();
  };
}
