import assert from 'node:assert/strict';
import test from 'node:test';

import { parseReleaseNote, plainMarkdown } from '../src/lib/release-notes.js';

test('parses headings, paragraphs and fenced code without HTML execution', () => {
  const blocks = parseReleaseNote(`# RouterForge 1.2.3

Hello **world** with \`code\`.

\`\`\`text
:8080/custom/
\`\`\`

<script>alert(1)</script>
`);

  assert.deepEqual(blocks[0], { type:'heading', level:1, text:'RouterForge 1.2.3' });
  assert.deepEqual(blocks[1], { type:'paragraph', text:'Hello world with code.' });
  assert.deepEqual(blocks[2], { type:'code', language:'text', text:':8080/custom/' });
  assert.equal(blocks[3].text, '<script>alert(1)</script>');
});

test('parses markdown tables and bullet lists', () => {
  const blocks = parseReleaseNote(`| Component | Version |
| --- | --- |
| Core | **0.10.2** |

- First
- Second
`);

  assert.deepEqual(blocks[0], {
    type:'table',
    headers:['Component', 'Version'],
    rows:[['Core', '0.10.2']]
  });
  assert.deepEqual(blocks[1], { type:'list', items:['First', 'Second'] });
});

test('plainMarkdown removes presentation markers but keeps text', () => {
  assert.equal(plainMarkdown('**Core** `0.10.2` [notes](https://example.test)'), 'Core 0.10.2 notes');
});


// P41: release-notes route must participate in the same native Svelte Core
// page grid as App Center. Never implement a second HTML shell or DOM patch.
test('P41: release notes use native Core page margins and responsive Svelte layout', async () => {
  const { readFileSync } = await import('node:fs');
  const route = readFileSync(new URL('../src/routes/release-notes/+page.svelte', import.meta.url), 'utf8');
  const appCenter = readFileSync(new URL('../src/routes/apps/+page.svelte', import.meta.url), 'utf8');
  const shell = readFileSync(new URL('../src/admin-shell.css', import.meta.url), 'utf8');
  assert.match(route, /<div class="page rf-release-page">/);
  assert.match(route, /<div class="page-head rf-release-heading">/);
  assert.match(route, /<span class="page-kicker">ROUTERFORGE \/ RELEASE NOTES<\/span>/);
  assert.match(route, /<ReleaseNotesBrowser \{locale\}\s*\/>/);
  assert.match(route, /\$derived\(\$settings\.locale/);
  assert.match(appCenter, /<div class="page catalog-page app-center-page">/);
  assert.match(appCenter, /<div class="page-head">/);
  assert.match(shell, /\.page\s*\{[^}]*padding:\s*var\(--page-pad\)/s);
  assert.match(route, /grid-template-rows:\s*auto minmax\(0,\s*1fr\)/);
  assert.match(route, /@media\s*\(max-width:\s*760px\)/);
  assert.match(route, /\.rf-release-view\s*\{[^}]*min-width:\s*0/s);
  assert.doesNotMatch(route, /<iframe\b|<script\s+src=|document\.querySelector|innerHTML\s*=|onMount\(/i);
});
