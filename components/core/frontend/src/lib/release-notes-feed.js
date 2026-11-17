export function noteVersionParts(version) {
  const match=String(version).match(/^(\d+)\.(\d+)\.(\d+)(?:-beta\.(\d+))?$/);
  if(!match) return null;
  return [Number(match[1]),Number(match[2]),Number(match[3]),match[4]===undefined?Infinity:Number(match[4])];
}
export function compareNoteVersions(a,b) {
  const x=noteVersionParts(a.version)||[0,0,0,0],y=noteVersionParts(b.version)||[0,0,0,0];
  for(let n=0;n<x.length;n++) if(x[n]!==y[n]) return y[n]-x[n];
  return 0;
}
export function normalizeNoteItem(item,source) {
  const version=String(item?.version||'');
  const channel=item?.channel==='beta'?'beta':'stable';
  if(!noteVersionParts(version) || (channel==='beta')!==version.includes('-beta.')) return null;
  if(source!=='remote'&&source!=='local') return null;
  return {
    version,channel,title:String(item?.title||`RouterForge ${version}`),source,
    remoteSummary:source==='remote'&&Boolean(item?.summary?.asset),
    remoteStory:source==='remote'&&Boolean(item?.story?.asset),
    summary:source==='remote'?Boolean(item?.summary?.asset):Boolean(item?.summary_file),
    story:source==='remote'?Boolean(item?.story?.asset):Boolean(item?.story_file||item?.file),
    summaryFile:item?.summary_file||'',storyFile:item?.story_file||item?.file||''
  };
}
export function mergeNoteIndexes(local,remote) {
  const map=new Map();
  for(const item of local?.releases||[]) {
    const n=normalizeNoteItem(item,'local');
    if(n) map.set(`${n.channel}:${n.version}`,n);
  }
  for(const item of remote?.releases||[]) {
    const n=normalizeNoteItem(item,'remote');
    if(n) {
      const previous=map.get(`${n.channel}:${n.version}`);
      if(previous) {
        n.localSummaryFile=previous.summaryFile || '';
        n.localStoryFile=previous.storyFile || '';
        n.summary=n.summary || Boolean(n.localSummaryFile);
        n.story=n.story || Boolean(n.localStoryFile);
      }
      map.set(`${n.channel}:${n.version}`,n);
    }
  }
  return Array.from(map.values()).sort(compareNoteVersions);
}


// Published release-note discovery, independent of the installed package feed.
// Keep these helpers pure so the Beta/Stable and unread rules can be regression-tested.
export function noteChannelForCoreVersion(version) {
  const raw = String(version || '').trim().toLowerCase();
  if (!raw) return null;
  if (/(?:~|-|\.)(?:beta|dev)(?:[.~-]|$)/.test(raw)) return 'beta';
  if (/^v?\d+\.\d+\.\d+(?:\+.*)?$/.test(raw) || /(?:~|-|\.)stable(?:[.~-]|$)/.test(raw)) return 'stable';
  return null;
}

export function noteVersionForCore(version) {
  const m = String(version || '').trim().match(/^v?(\d+\.\d+\.\d+)(?:(?:~|-)beta\.(\d+))?(?:\+[^\s]+)?$/i);
  return m ? m[1] + (m[2] === undefined ? '' : '-beta.' + m[2]) : '';
}

export function newerNoteVersion(candidate, current) {
  if (!noteVersionParts(candidate) || !noteVersionParts(current)) return false;
  return compareNoteVersions({ version: candidate }, { version: current }) < 0;
}

export function latestPublishedNoteVersion(releases, channel) {
  if (channel !== 'beta' && channel !== 'stable') return '';
  const valid = (Array.isArray(releases) ? releases : [])
    .map((item) => normalizeNoteItem(item, 'remote'))
    .filter((item) => item && item.channel === channel && (item.summary || item.story));
  valid.sort(compareNoteVersions);
  return valid[0]?.version || '';
}

// A release note is unread until it has actually been viewed, including after
// installing that very release. Installed Core protects against older archives
// appearing "new"; an explicit read checkpoint takes precedence.
export function unreadPublishedNoteVersion(latest, read, installed) {
  if (!noteVersionParts(latest)) return '';
  if (noteVersionParts(read)) return newerNoteVersion(latest, read) ? latest : '';
  if (!noteVersionParts(installed)) return '';
  return newerNoteVersion(installed, latest) ? '' : latest;
}
