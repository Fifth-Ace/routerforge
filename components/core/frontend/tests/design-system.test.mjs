import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const css = readFileSync(new URL('../static/design-system/routerforge-v1.css', import.meta.url), 'utf8');

test('Core owns the opt-in, versioned design system with no global selector takeover', () => {
  for (const token of [
    '--rfds-accent', '--rfds-surface', '--rfds-border', '--rfds-radius-panel',
    '--rfds-control-h', '--rfds-space-3', '--rfds-font',
  ]) assert.ok(css.includes(token), `missing ${token}`);
  for (const component of [
    'rfds-card','rfds-toolbar','rfds-button','rfds-segmented',
    'rfds-tabs','rfds-field','rfds-form-grid',
  ]) assert.ok(css.includes(`.rfds-${component.replace(/^rfds-/, '')}`), component);
  assert.match(css, /\.rfds-scope\s*\{/);
  assert.doesNotMatch(css, /(?:^|\n)\s*(?:html|body|\*|button|input|select|textarea)\s*\{/m);
  assert.match(css, /prefers|@media\(max-width:720px\)/);
});
