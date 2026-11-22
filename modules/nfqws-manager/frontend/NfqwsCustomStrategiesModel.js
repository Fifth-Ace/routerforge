// Pure Svelte-owned NFQWS_ARGS_CUSTOM editor model. No IO, no DOM, no config writes.
export const DEFAULT_CUSTOM_STRATEGY = '--filter-tcp=443 --dpi-desync=fake';
export function normalizeCustomLabel(value) {
  return String(value ?? '').replace(/[\r\n]+/g, ' ').trim().slice(0, 120);
}
export function customSnapshot(blocks = [], labels = []) {
  if (!Array.isArray(blocks) || !Array.isArray(labels) || blocks.length !== labels.length ||
      blocks.some(v => typeof v !== 'string') || labels.some(v => typeof v !== 'string')) {
    throw Error('NFQWS_CUSTOM_SNAPSHOT_INVALID');
  }
  return {blocks: [...blocks], labels: labels.map(normalizeCustomLabel)};
}
export function editCustom(snapshot, command, index = -1, value = '') {
  const {blocks, labels} = customSnapshot(snapshot?.blocks, snapshot?.labels);
  if (command === 'add') {
    blocks.push(DEFAULT_CUSTOM_STRATEGY); labels.push('');
    return {blocks, labels};
  }
  if (!Number.isInteger(index) || index < 0 || index >= blocks.length)
    throw Error('NFQWS_CUSTOM_STALE_INDEX');
  switch (command) {
    case 'text': blocks[index] = String(value); break;
    case 'label': labels[index] = normalizeCustomLabel(value); break;
    case 'up':
    case 'down': {
      const other = index + (command === 'up' ? -1 : 1);
      if (other < 0 || other >= blocks.length) throw Error('NFQWS_CUSTOM_MOVE_OUT_OF_RANGE');
      [blocks[index], blocks[other]] = [blocks[other], blocks[index]];
      [labels[index], labels[other]] = [labels[other], labels[index]];
      break;
    }
    case 'clone':
      blocks.splice(index + 1, 0, blocks[index]);
      labels.splice(index + 1, 0, labels[index] ? `${labels[index]} — копия` : '');
      break;
    case 'remove': blocks.splice(index, 1); labels.splice(index, 1); break;
    default: throw Error('NFQWS_CUSTOM_COMMAND_UNKNOWN');
  }
  return {blocks, labels};
}
export function customSummaryTokens(value) {
  const source = String(value ?? '');
  const tokens = source.match(/(?:[^\s"']+|"[^"]*"|'[^']*')+/g) || [];
  const picks = [['filter-tcp','TCP'], ['filter-udp','UDP'], ['filter-l7','L7'],
    ['payload','payload'], ['lua-desync','lua'], ['dpi-desync','desync'],
    ['hostlist','hostlist'], ['ipset','ipset']];
  return picks.flatMap(([key,label]) => {
    const found = tokens.find(token => token.startsWith(`--${key}=`));
    if (!found) return [];
    return [{key, label, value:found.slice(key.length + 3), type:
      ['filter-tcp','filter-udp'].includes(key) ? 'good' :
      ['lua-desync','dpi-desync'].includes(key) ? 'warn' : ''}];
  });
}
export function serializeCustom(snapshot) {
  return customSnapshot(snapshot?.blocks, snapshot?.labels).blocks.map(s => s.trim()).filter(Boolean).join('\n--new\n');
}
