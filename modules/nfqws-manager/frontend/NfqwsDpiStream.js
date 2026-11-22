// NFQWS DPI Detector command boundary: only explicit callers can start an NDJSON
// stream. No DOM, Svelte lifecycle, network activity or global UI state at import.
export async function consumeDpiNdjson({url, payload, signal, onEvent, fetchImpl = fetch}) {
  if (typeof url !== 'string' || !url.startsWith('/') || url.startsWith('//') ||
      !url.endsWith('/dpi-detector/v5/stream')) throw Error('DPI_STREAM_PATH_INVALID');
  if (typeof onEvent !== 'function' || typeof fetchImpl !== 'function') throw Error('DPI_STREAM_DEPENDENCY_INVALID');
  if (signal?.aborted) throw Error('DPI_STREAM_CANCELLED');
  const response = await fetchImpl(url, {
    method: 'POST',cache: 'no-store',credentials: 'same-origin',signal,
    headers: {Accept:'application/x-ndjson','Content-Type':'application/json'},
    body: JSON.stringify(payload)
  });
  if (!response.ok) {
    const text = await response.text();let details={};
    try { details = JSON.parse(text); } catch { details = {error:text}; }
    throw Error(String(details?.error || details?.detail || `HTTP ${response.status}`));
  }
  if (!response.body) throw Error('streaming body unavailable');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let pending = '';
  // Backend console/report capture is capped at 4 MiB; JSON escaping can expand it.
  const lineLimit = 16 * 1024 * 1024;
  const publish = line => {
    if (!line.trim() || signal?.aborted) return;
    let event;
    try { event = JSON.parse(line); } catch { return; } // Preserve historical malformed-line tolerance.
    onEvent(event);
  };
  try {
    for (;;) {
      if (signal?.aborted) throw Error('DPI_STREAM_CANCELLED');
      const part = await reader.read();
      pending += decoder.decode(part.value || new Uint8Array(), {stream: !part.done});
      let nl;
      while ((nl = pending.indexOf('\n')) >= 0) {
        if (nl > lineLimit) throw Error('DPI_STREAM_LINE_LIMIT');
        const line = pending.slice(0,nl); pending = pending.slice(nl+1);
        publish(line);
      }
      if (pending.length > lineLimit) throw Error('DPI_STREAM_LINE_LIMIT');
      if (part.done) break;
    }
    publish(pending);
    if (signal?.aborted) throw Error('DPI_STREAM_CANCELLED');
  } finally {
    try { await reader.cancel(); } catch (_) {}
    try { reader.releaseLock(); } catch (_) {}
  }
}
