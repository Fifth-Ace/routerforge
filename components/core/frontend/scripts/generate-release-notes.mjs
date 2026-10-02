import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const frontend = path.resolve(here, '..');
const root = path.resolve(frontend, '../../..');
const docs = path.join(root, 'docs');
const output = path.join(frontend, 'static', 'release-notes');

function semver(value) {
  return String(value).split('.').map((part) => Number(part) || 0);
}

function compareVersions(a, b) {
  const left = semver(a);
  const right = semver(b);
  const size = Math.max(left.length, right.length);
  for (let i = 0; i < size; i += 1) {
    const delta = (right[i] || 0) - (left[i] || 0);
    if (delta) return delta;
  }
  return 0;
}

await rm(output, { recursive:true, force:true });
await mkdir(output, { recursive:true });

const names = (await readdir(docs))
  .filter((name) => /^RELEASE_NOTES_\d+\.\d+\.\d+\.md$/.test(name));

const releases = [];
for (const name of names) {
  const version = name.match(/^RELEASE_NOTES_(\d+\.\d+\.\d+)\.md$/)?.[1];
  if (!version) continue;

  const markdown = await readFile(path.join(docs, name), 'utf8');
  const title = markdown.match(/^#\s+(.+)$/m)?.[1]?.trim() || `RouterForge ${version}`;
  const file = `${version}.md`;

  await writeFile(path.join(output, file), markdown.replace(/\r\n/g, '\n'), 'utf8');
  releases.push({ version, file, title });
}

releases.sort((a, b) => compareVersions(a.version, b.version));

let currentStable = '';
try {
  const stable = JSON.parse(await readFile(path.join(root, 'release', 'channels', 'stable.json'), 'utf8'));
  currentStable = String(stable.release_version || '');
} catch {
  currentStable = '';
}

await writeFile(
  path.join(output, 'index.json'),
  JSON.stringify({
    schema_version:1,
    generated_from:'docs/RELEASE_NOTES_*.md',
    current_stable:currentStable,
    releases
  }, null, 2) + '\n',
  'utf8'
);

console.log(`RELEASE_NOTES_GENERATED=${releases.length}`);
console.log(`RELEASE_NOTES_CURRENT_STABLE=${currentStable || 'unknown'}`);
