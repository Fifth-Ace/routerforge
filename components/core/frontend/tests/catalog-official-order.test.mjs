import assert from 'node:assert/strict';
import test from 'node:test';

import { sortCatalogItems } from '../src/lib/catalogPresentation.js';

test('source sorting preserves backend official module order', () => {
  const items = [
    { id:'routerforge-core', name:'RouterForge Core' },
    { id:'monitoring', name:'RouterForge Monitoring' },
    { id:'dns', name:'RouterForge DNS' },
    { id:'admin', name:'RouterForge Control' },
    { id:'network-tools', name:'Network Tools' },
    { id:'nfqws-manager', name:'RouterForge NFQWS Manager' },
    { id:'antiscan-manager', name:'RouterForge Antiscan Manager' },
    { id:'profiling', name:'Profiling' }
  ];

  assert.deepEqual(
    sortCatalogItems(items, 'source', 'en').map((item) => item.id),
    items.map((item) => item.id)
  );
});
