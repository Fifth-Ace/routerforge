import assert from 'node:assert/strict';
import test from 'node:test';
import { mergeNoteIndexes, compareNoteVersions, normalizeNoteItem } from '../src/lib/release-notes-feed.js';

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
