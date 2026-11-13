import test from 'node:test';
import assert from 'node:assert/strict';
import {highlightCustomStrategy} from './NfqwsCustomSyntax.js';
test('Svelte-owned custom syntax highlighter emits escaped text, never executable markup',()=>{
 const html=highlightCustomStrategy('<img src=x onerror=alert(1)> --filter-tcp=443');
 assert.ok(html.includes('&lt;img'));
 assert.ok(!html.includes('<img'));
 assert.ok(html.includes('syn-flag'));
});
test('comments, quotes and variables retain syntax colors without raw HTML',()=>{
 const html=highlightCustomStrategy('# Strategy header\n--hostlist="user.list" $MODE_LIST');
 assert.ok(html.includes('syn-strategy'));assert.ok(html.includes('syn-string'));assert.ok(html.includes('syn-variable'));
});
test('empty custom block produces visible blank marker',()=>{
 assert.equal(highlightCustomStrategy(''),'&#8203;');
});
