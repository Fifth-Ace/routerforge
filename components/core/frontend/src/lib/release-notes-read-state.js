import { writable } from 'svelte/store';
import { noteVersionParts, newerNoteVersion } from './release-notes-feed.js';

// Native Svelte state shared by sidebar, modal and full-size release history.
export const releaseNoteReads = writable({ beta: '', stable: '' });
const readKey = (channel) => `routerforge:release-notes:last-read:v1:${channel}`;

function storage() {
  try { return window.localStorage; } catch { return null; }
}

export function loadReleaseNoteReads() {
  const store = storage();
  const state = { beta: '', stable: '' };
  for (const channel of ['beta', 'stable']) {
    try {
      const value = store?.getItem(readKey(channel)) || '';
      if (noteVersionParts(value)) state[channel] = value;
    } catch { /* Session-only state if browser persistence is unavailable. */ }
  }
  releaseNoteReads.set(state);
}

export function markReleaseNoteRead(channel, version) {
  if ((channel !== 'beta' && channel !== 'stable') || !noteVersionParts(version)) return;
  releaseNoteReads.update((current) => {
    const previous = current[channel];
    if (previous && !newerNoteVersion(version, previous)) return current;
    const next = { ...current, [channel]: version };
    try { storage()?.setItem(readKey(channel), version); } catch {}
    return next;
  });
}
