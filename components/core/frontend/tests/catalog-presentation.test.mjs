import assert from 'node:assert/strict';
import test from 'node:test';

import {
  catalogCategoryKey,
  catalogCategoryLabel,
  buildCatalogCategoryOptions,
  sortCatalogItems,
  buildCatalogDisplayRows
} from '../src/lib/catalogPresentation.js';

test('normalizes legacy and overlapping catalog categories', () => {
  assert.equal(catalogCategoryKey({ category:'VPN / Routing' }), 'routing');
  assert.equal(catalogCategoryKey({ category:'Remote Access' }), 'routing');
  assert.equal(catalogCategoryKey({ category:'DPI / Bypass' }), 'adaptation');
  assert.equal(catalogCategoryKey({ category:'DNS / Filtering' }), 'dns');
  assert.equal(catalogCategoryKey({ category:'DNS / Routing' }), 'dns');
  assert.equal(catalogCategoryKey({ category:'Proxy / Routing' }), 'proxy');
  assert.equal(catalogCategoryKey({ category:'Toolkit' }), 'network');
  assert.equal(catalogCategoryKey({ category:'Backup' }), 'automation');
  assert.equal(catalogCategoryKey({ category:'Unknown Legacy Category' }), 'other');
});

test('module items do not expose RouterForge as a visible category', () => {
  assert.equal(catalogCategoryLabel({ kind:'module', id:'dns' }, 'ru'), 'DNS и фильтрация');
  assert.equal(catalogCategoryLabel({ kind:'module', id:'admin' }, 'ru'), 'Система и администрирование');
  assert.equal(catalogCategoryLabel({ kind:'module', id:'nfqws-manager' }, 'ru'), 'Сетевая совместимость');
});

test('category labels are softened and localized without mutating source category', () => {
  const routingItem = { category:'VPN / Routing' };
  const adaptationItem = { category:'DPI / Bypass' };
  assert.equal(catalogCategoryLabel(routingItem, 'ru'), 'Маршрутизация и туннелирование');
  assert.equal(catalogCategoryLabel(adaptationItem, 'ru'), 'Сетевая совместимость');
  assert.equal(catalogCategoryLabel(adaptationItem, 'en'), 'Network Compatibility');
  assert.equal(routingItem.category, 'VPN / Routing');
  assert.equal(adaptationItem.category, 'DPI / Bypass');
});

test('category options merge overlapping raw categories', () => {
  const options = buildCatalogCategoryOptions([
    { category:'DNS' },
    { category:'DNS / Filtering' },
    { category:'DNS / Routing' },
    { category:'Proxy' }
  ], 'ru');

  assert.deepEqual(options, [
    { key:'dns', name:'DNS и фильтрация', count:3 },
    { key:'proxy', name:'Прокси', count:1 }
  ]);
});

test('sorting supports update and installed priority', () => {
  const items = [
    { id:'a', name:'Alpha', installed:false, update_available:false },
    { id:'b', name:'Beta', installed:true, update_available:false },
    { id:'c', name:'Gamma', installed:true, update_available:true }
  ];
  assert.deepEqual(sortCatalogItems(items, 'updates', 'en').map((item) => item.id), ['c','b','a']);
  assert.deepEqual(sortCatalogItems(items, 'installed', 'en').map((item) => item.id), ['c','b','a']);
});

test('grouping emits stable category headers and item rows', () => {
  const rows = buildCatalogDisplayRows([
    { id:'vpn-b', name:'Beta', category:'VPN / Routing' },
    { id:'dns-a', name:'Alpha', category:'DNS / Filtering' },
    { id:'vpn-a', name:'Alpha', category:'Remote Access' }
  ], 'category', 'name', 'en');

  assert.deepEqual(rows.map((row) => row.type === 'group' ? `#${row.label}:${row.count}` : row.item.id), [
    '#DNS & Filtering:1',
    'dns-a',
    '#Routing & Tunneling:2',
    'vpn-a',
    'vpn-b'
  ]);
});
