import { writable } from 'svelte/store';
import { getSnapshot } from '$lib/api.js';
import { startSerialPolling } from '$lib/polling.js';

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
export const backendReady = writable(false);
export const streamMode = writable('connecting');

export function startSnapshotStream(intervalMs = 2000) {
  let closed = false;
  let eventSource = null;
  let stopFallbackPolling = null;
  let fetchInFlight = null;
  const interval = Math.max(1000, Math.min(30000, Number(intervalMs || 2000)));

  const publish = (data) => {
    if (!data || typeof data !== 'object') return;
    snapshot.set({ ...empty, ...data });
    backendOnline.set(true);
    backendReady.set(true);
  };

  const fetchOnce = () => {
    if (fetchInFlight) return fetchInFlight;

    fetchInFlight = (async () => {
      try {
        publish(await getSnapshot());
        return true;
      } catch {
        backendOnline.set(false);
        backendReady.set(true);
        return false;
      } finally {
        fetchInFlight = null;
      }
    })();

    return fetchInFlight;
  };

  const stopFallback = () => {
    if (!stopFallbackPolling) return;
    stopFallbackPolling();
    stopFallbackPolling = null;
  };

  const startFallback = () => {
    if (stopFallbackPolling || closed) return;
    streamMode.set('polling');
    stopFallbackPolling = startSerialPolling(fetchOnce, interval, { immediate: false });
  };

  void fetchOnce();

  if (typeof EventSource === 'undefined') {
    startFallback();
  } else {
    eventSource = new EventSource(`/api/events?interval_ms=${encodeURIComponent(interval)}`);
    eventSource.onopen = () => {
      backendOnline.set(true);
      backendReady.set(true);
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
      startFallback();
      void fetchOnce();
    };
  }

  return () => {
    closed = true;
    stopFallback();
    eventSource?.close();
  };
}
