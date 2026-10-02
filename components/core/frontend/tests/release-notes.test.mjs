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
