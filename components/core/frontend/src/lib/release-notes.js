export function plainMarkdown(value = '') {
  return String(value)
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/__([^_]+)__/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .trim();
}

function tableCells(line) {
  return String(line)
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map((cell) => plainMarkdown(cell.trim()));
}

function isTableSeparator(line) {
  const cells = tableCells(line);
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(cell.replace(/\s+/g, '')));
}

function startsBlock(lines, index) {
  const line = lines[index] || '';
  if (!line.trim()) return true;
  if (/^#{1,6}\s+/.test(line)) return true;
  if (/^```/.test(line)) return true;
  if (/^\s*[-*]\s+/.test(line)) return true;
  if (/^\s*>\s?/.test(line)) return true;
  if (line.includes('|') && index + 1 < lines.length && isTableSeparator(lines[index + 1])) return true;
  return false;
}

export function parseReleaseNote(markdown = '') {
  const lines = String(markdown).replace(/\r\n/g, '\n').split('\n');
  const blocks = [];

  for (let i = 0; i < lines.length;) {
    const raw = lines[i];
    const line = raw.trim();

    if (!line) {
      i += 1;
      continue;
    }

    const heading = raw.match(/^(#{1,6})\s+(.+)$/);
    if (heading) {
      blocks.push({
        type:'heading',
        level:heading[1].length,
        text:plainMarkdown(heading[2])
      });
      i += 1;
      continue;
    }

    if (/^```/.test(raw)) {
      const language = raw.replace(/^```/, '').trim();
      const code = [];
      i += 1;
      while (i < lines.length && !/^```/.test(lines[i])) {
        code.push(lines[i]);
        i += 1;
      }
      if (i < lines.length) i += 1;
      blocks.push({ type:'code', language, text:code.join('\n') });
      continue;
    }

    if (raw.includes('|') && i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
      const headers = tableCells(raw);
      const rows = [];
      i += 2;
      while (i < lines.length && lines[i].includes('|') && lines[i].trim()) {
        rows.push(tableCells(lines[i]));
        i += 1;
      }
      blocks.push({ type:'table', headers, rows });
      continue;
    }

    if (/^\s*[-*]\s+/.test(raw)) {
      const items = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
        items.push(plainMarkdown(lines[i].replace(/^\s*[-*]\s+/, '')));
        i += 1;
      }
      blocks.push({ type:'list', items });
      continue;
    }

    if (/^\s*>\s?/.test(raw)) {
      const parts = [];
      while (i < lines.length && /^\s*>\s?/.test(lines[i])) {
        parts.push(plainMarkdown(lines[i].replace(/^\s*>\s?/, '')));
        i += 1;
      }
      blocks.push({ type:'quote', text:parts.join(' ') });
      continue;
    }

    const paragraph = [line];
    i += 1;
    while (i < lines.length && lines[i].trim() && !startsBlock(lines, i)) {
      paragraph.push(lines[i].trim());
      i += 1;
    }
    blocks.push({ type:'paragraph', text:plainMarkdown(paragraph.join(' ')) });
  }

  return blocks;
}
