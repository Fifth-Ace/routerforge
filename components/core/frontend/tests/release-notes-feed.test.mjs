import assert from 'node:assert/strict';
import test from 'node:test';
import { mergeNoteIndexes, compareNoteVersions, normalizeNoteItem, noteChannelForCoreVersion, noteVersionForCore, newerNoteVersion, latestPublishedNoteVersion, unreadPublishedNoteVersion, preferredReleaseNotesChannel } from '../src/lib/release-notes-feed.js';

test('stable, beta, old v1 archives and version ordering', () => {
  const local={releases:[
    {version:'0.13.0',file:'0.13.0.md'},
    {version:'0.15.0-beta.2',channel:'beta',summary_file:'0.15.0-beta.2-summary.md'}
  ]};
  const remote={releases:[
    {version:'0.15.0-beta.3',channel:'beta',summary:{asset:'x'}},
    {version:'0.13.0',channel:'stable',story:{asset:'x'}}
  ]};
  const merged=mergeNoteIndexes(local,remote);
  assert.deepEqual(merged.map(x=>x.version),['0.15.0-beta.3','0.15.0-beta.2','0.13.0']);
  assert.equal(merged[2].source,'remote');
  assert.equal(merged[2].story,true);
});
test('remote notes do not accept unknown version/channel',()=>{
  assert.equal(normalizeNoteItem({version:'../../bad',channel:'beta'},'remote'),null);
  assert.equal(normalizeNoteItem({version:'0.15.0-beta.3',channel:'stable'},'remote'),null);
  assert.ok(compareNoteVersions({version:'0.15.0'},{version:'0.15.0-beta.7'})<0);
  assert.ok(compareNoteVersions({version:'0.15.0-beta.10'},{version:'0.15.0-beta.9'})<0);
});


test('installed Core selects Beta/Dev or Stable without confusing package syntax', () => {
  assert.equal(noteChannelForCoreVersion('0.15.0~beta.5'), 'beta');
  assert.equal(noteChannelForCoreVersion('0.15.0-beta.6'), 'beta');
  assert.equal(noteChannelForCoreVersion('1~dev.r1360.abc'), 'beta');
  assert.equal(noteChannelForCoreVersion('0.13.0'), 'stable');
  assert.equal(noteChannelForCoreVersion(''), null);
  assert.equal(noteVersionForCore('0.15.0~beta.5'), '0.15.0-beta.5');
  assert.equal(noteVersionForCore('0.15.0'), '0.15.0');
  assert.equal(noteVersionForCore('1~dev.r1360.abc'), '');
});

test('latest published notes are channel-scoped and sorted numerically', () => {
  const releases = [
    {version:'0.15.0-beta.9',channel:'beta',summary:{asset:'note9'}},
    {version:'0.15.0-beta.10',channel:'beta',summary:{asset:'note10'}},
    {version:'0.15.0',channel:'stable',story:{asset:'stable'}},
    {version:'0.15.0-beta.11',channel:'stable',summary:{asset:'wrong-channel'}},
    {version:'0.15.0-beta.20',channel:'beta'}, // never notify an unviewable entry
  ];
  assert.equal(latestPublishedNoteVersion(releases,'beta'),'0.15.0-beta.10');
  assert.equal(latestPublishedNoteVersion(releases,'stable'),'0.15.0');
  assert.equal(latestPublishedNoteVersion(releases,'dev'),'');
});

test('App Center channel wins over installed Core while explicit tab navigation stays independent', () => {
  assert.equal(preferredReleaseNotesChannel('beta', '0.14.0'), 'beta');
  assert.equal(preferredReleaseNotesChannel('stable', '0.15.0~beta.6'), 'stable');
  assert.equal(preferredReleaseNotesChannel('dev', '0.14.0'), 'beta');
  assert.equal(preferredReleaseNotesChannel('', '0.15.0~beta.6'), 'beta');
  assert.equal(preferredReleaseNotesChannel('', '0.14.0'), 'stable');
  assert.equal(preferredReleaseNotesChannel('', ''), 'stable');
});

test('unread glow starts for a newer published patch and ends after reading', () => {
  assert.equal(unreadPublishedNoteVersion('0.15.0-beta.6','','0.15.0-beta.5'),'0.15.0-beta.6');
  assert.equal(unreadPublishedNoteVersion('0.15.0-beta.6','','0.15.0-beta.6'),'0.15.0-beta.6');
  assert.equal(unreadPublishedNoteVersion('0.15.0-beta.6','0.15.0-beta.6','0.15.0-beta.5'),'');
  assert.equal(unreadPublishedNoteVersion('0.15.0-beta.7','0.15.0-beta.6','0.15.0-beta.7'),'0.15.0-beta.7');
  assert.equal(unreadPublishedNoteVersion('0.15.0-beta.5','0.15.0-beta.6','0.15.0-beta.5'),'');
  assert.equal(unreadPublishedNoteVersion('0.15.0-beta.6','',''),'');
  assert.equal(unreadPublishedNoteVersion('0.15.0-beta.5','','0.15.0-beta.6'),'');
  assert.equal(newerNoteVersion('0.15.0-beta.10','0.15.0-beta.9'),true);
  assert.equal(newerNoteVersion('0.15.0-beta.9','0.15.0-beta.10'),false);
  assert.equal(newerNoteVersion('0.15.0-beta.10','0.15.0-beta.10'),false);
});
