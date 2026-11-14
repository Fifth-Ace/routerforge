#!/usr/bin/env python3
"""Build approved public notes assets; never publish from this script."""
import argparse
import hashlib
import json
import re
from pathlib import Path


def build(channel: str, docs: Path, dest: Path):
    if channel not in ('stable', 'beta'):
        raise ValueError('invalid release channel')
    dest.mkdir(parents=True, exist_ok=True)
    expr = (r'RELEASE_NOTES_(\d+\.\d+\.\d+)\.md' if channel == 'stable'
            else r'RELEASE_NOTES_(\d+\.\d+\.\d+-beta\.\d+)\.md')
    records = []
    files = []
    for source in sorted(docs.iterdir()):
        found = re.fullmatch(expr, source.name)
        if not found or not source.is_file():
            continue
        version = found.group(1)
        entry = {'version': version, 'title': 'RouterForge ' + version}
        primary = source.read_bytes().replace(b'\r\n', b'\n')
        if not primary or len(primary) > 262144:
            raise ValueError('invalid note length: ' + source.name)
        title = re.search(r'^#\s+(.+)$', primary.decode('utf-8'), re.M)
        if title:
            entry['title'] = title.group(1).strip()[:180]
        summary = docs / ('RELEASE_SUMMARY_' + version + '.md')
        items = []
        if channel == 'beta':
            items.append(('summary', summary if summary.is_file() else source))
        else:
            if summary.is_file():
                items.append(('summary', summary))
            items.append(('story', source))
        for kind, filepath in items:
            data = filepath.read_bytes().replace(b'\r\n', b'\n')
            if not data or len(data) > 262144:
                raise ValueError('invalid note length: ' + filepath.name)
            asset = 'routerforge-notes-%s-%s-%s.md' % (channel, version, kind)
            (dest / asset).write_bytes(data)
            entry[kind] = {'asset': asset, 'sha256': hashlib.sha256(data).hexdigest()}
            files.append(asset)
        records.append(entry)
    def sort_key(record):
        return tuple(int(n) for n in re.findall(r'\d+', record['version']))
    records.sort(key=sort_key, reverse=True)
    index = {'schema_version': 2, 'channel': channel, 'releases': records}
    name = 'routerforge-notes-%s-index.json' % channel
    (dest / name).write_text(json.dumps(index, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    return files, name


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--channel', required=True, choices=('stable', 'beta'))
    parser.add_argument('--docs', default='docs')
    parser.add_argument('--dist', default='dist')
    args = parser.parse_args()
    assets, index = build(args.channel, Path(args.docs), Path(args.dist))
    print('NOTES_CATALOG: PASS / %s / %d files' % (args.channel, len(assets)))
    print('NOTES_INDEX: ' + index)
