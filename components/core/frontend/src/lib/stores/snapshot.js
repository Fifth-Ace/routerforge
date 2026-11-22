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
  let fetchController = null;
  let sseActive = false;
  const interval = Math.max(1000, Math.min(30000, Number(intervalMs || 2000)));

  const publish = (data) => {
    if (closed || !data || typeof data !== 'object') return;
    snapshot.set({ ...empty, ...data });
    backendOnline.set(true);
    backendReady.set(true);
  };

  const fetchOnce = () => {
    if (fetchInFlight) return fetchInFlight;

    const controller = new AbortController();
    fetchController = controller;
    fetchInFlight = (async () => {
      try {
        const data = await getSnapshot(controller.signal);
        if (!controller.signal.aborted && !sseActive) publish(data);
        return !controller.signal.aborted;
      } catch {
        if (!closed && !controller.signal.aborted && !sseActive) {
          backendOnline.set(false);
          backendReady.set(true);
        }
        return false;
      } finally {
        if (fetchController === controller) fetchController = null;
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
      if (closed) return;
      sseActive = true;
      fetchController?.abort();
      backendOnline.set(true);
      backendReady.set(true);
      streamMode.set('sse');
      stopFallback();
    };
    eventSource.addEventListener('snapshot', (event) => {
      if (closed) return;
      try {
        publish(JSON.parse(event.data));
        streamMode.set('sse');
        stopFallback();
      } catch {}
    });
    eventSource.onerror = () => {
      if (closed) return;
      sseActive = false;
      startFallback();
      void fetchOnce();
    };
  }

  return () => {
    closed = true;
    fetchController?.abort();
    stopFallback();
    eventSource?.close();
  };
}
