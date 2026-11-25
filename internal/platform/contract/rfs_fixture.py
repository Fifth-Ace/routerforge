#!/usr/bin/env python3
"""Portable rfs-side fixture projection (Python stdlib, no Linux mutation).
This is an interoperability fixture reader, not the production P5 collector.
"""
import json
from pathlib import Path

root = Path(__file__).resolve().parent
with (root / 'golden.json').open(encoding='utf-8') as source:
    document = json.load(source)
assert document['catalog']['schemaVersion'] == 1
sample = document['snapshots'][0]
assert sample['schemaVersion'] == 1
connection = next(c for c in document['catalog']['connections'] if c['id'] == sample['connectionId'])
assert connection['deviceId'] == sample['deviceId'] and connection['adapterId'] == sample['adapterId']
metrics = sample['metrics']
assert metrics['memory']['usedBytes'] / metrics['memory']['totalBytes'] * 100 == 25
assert metrics['swap']['totalBytes'] == 0
assert metrics['network'][0]['rxBps'] is None
assert metrics['uptimeSec'] is None
assert sample['latencyKind'] == 'api'
print('GOLDEN PASS / rfs-side Python fixture projection / NOT production collector')
