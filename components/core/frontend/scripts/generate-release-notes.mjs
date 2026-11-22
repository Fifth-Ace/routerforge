import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const frontend = path.resolve(here, '..');
const root = path.resolve(frontend, '../../..');
const docs = path.join(root, 'docs');
const output = path.join(frontend, 'static', 'release-notes');
const authoredStories = path.join(frontend, 'release-stories');

const parseVersion = (s) => {
  const m = String(s).match(/^(\d+)\.(\d+)\.(\d+)(?:-beta\.(\d+))?$/);
  return m ? [Number(m[1]), Number(m[2]), Number(m[3]), m[4] === undefined ? Infinity : Number(m[4])] : [0, 0, 0, -1];
};
function compareVersions(a, b) {
  const x = parseVersion(a), y = parseVersion(b);
  for (let i = 0; i < x.length; i += 1) if (x[i] !== y[i]) return y[i] - x[i];
  return 0;
}
async function channelVersion(channel) {
  try {
    const config = JSON.parse(await readFile(path.join(root, 'release', 'channels', `${channel}.json`), 'utf8'));
    return String(config.release_version || '');
  } catch { return ''; }
}
async function copyDoc(input, file) {
  const content = (await readFile(path.join(docs, input), 'utf8')).replace(/\r\n/g, '\n');
  await writeFile(path.join(output, file), content, 'utf8');
  return content.match(/^#\s+(.+)$/m)?.[1]?.trim() || '';
}

async function copyStory(version, filename) {
  const content = (await readFile(path.join(authoredStories, `${version}.md`), 'utf8')).replace(/\r\n/g, '\n');
  await writeFile(path.join(output, filename), content, 'utf8');
  return content.match(/^#\s+(.+)$/m)?.[1]?.trim() || '';
}

await rm(output, { recursive: true, force: true });
await mkdir(output, { recursive: true });
const names = await readdir(docs);
const storyNames = await readdir(authoredStories).catch(() => []);
const releases = [];
const stableRe = /^RELEASE_NOTES_(\d+\.\d+\.\d+)\.md$/;
const betaRe = /^RELEASE_NOTES_(\d+\.\d+\.\d+-beta\.\d+)\.md$/;
for (const name of names) {
  const stable = stableRe.exec(name), beta = betaRe.exec(name);
  if (!stable && !beta) continue;
  const version = (stable || beta)[1];
  const channel = stable ? 'stable' : 'beta';
  const item = { version, channel, title: '', summary_file: '', story_file: '' };
  const summarySource = `RELEASE_SUMMARY_${version}.md`;
  if (names.includes(summarySource)) {
    item.summary_file = `${version}-summary.md`;
    await copyDoc(summarySource, item.summary_file);
  } else if (channel === 'beta') {
    item.summary_file = `${version}-summary.md`;
    item.title = await copyDoc(name, item.summary_file);
  }
  if (channel === 'stable') {
    if (storyNames.includes(`${version}.md`)) {
      item.story_file = `${version}-story.md`;
      await copyStory(version, item.story_file);
    } else if (Number(version.split('.')[0]) < 1) {
      // Preserve the legacy local history, without publishing a new author story.
      item.story_file = `${version}-story.md`;
      item.title = await copyDoc(name, item.story_file);
    }
    if (!item.title && item.summary_file) item.title = `RouterForge ${version}`;
  }
  if (!item.title) item.title = `RouterForge ${version}`;
  releases.push(item);
}
releases.sort((a, b) => compareVersions(a.version, b.version));
const currentStable = await channelVersion('stable');
const currentBeta = await channelVersion('beta');
await writeFile(path.join(output, 'index.json'), JSON.stringify({
  schema_version: 2,
  generated_from: 'docs/RELEASE_NOTES_*.md + docs/RELEASE_SUMMARY_*.md + UI-local author stories',
  current_stable: currentStable,
  current_beta: currentBeta,
  releases
}, null, 2) + '\n', 'utf8');
console.log(`RELEASE_NOTES_GENERATED=${releases.length}`);
console.log(`RELEASE_NOTES_STABLE=${currentStable || 'unknown'} BETA=${currentBeta || 'unknown'}`);
