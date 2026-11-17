import test from 'node:test';
import assert from 'node:assert/strict';
import { SMART_DRIVER_STORAGE_KEY, smartDriverDiskKey, readSmartDriverPreferences, getSmartDriverPreference, setSmartDriverPreference } from './smart-driver-preferences.js';

const memoryStorage = () => {
  const values = new Map();
  return { getItem: (k) => values.get(k) ?? null, setItem: (k,v) => values.set(k,v) };
};
const drive = {id:'hw:serial-123',device:'/dev/sdb',model:'USB 3.1',vendor:'USB',size_bytes:2000398934016};

test('sat survives a page reload and is bound to physical disk ID', () => {
  const storage = memoryStorage();
  const first = setSmartDriverPreference(storage,{},drive,'sat');
  assert.equal(first.saved,true);
  assert.equal(getSmartDriverPreference(readSmartDriverPreferences(storage),drive),'sat');
  assert.equal(getSmartDriverPreference(readSmartDriverPreferences(storage),{...drive,id:'hw:other'}),'auto');
  assert.equal(getSmartDriverPreference(readSmartDriverPreferences(storage),{...drive,device:'/dev/sdc'}),'sat');
});
test('auto removes the override, no per-disk pollution', () => {
  const storage=memoryStorage();
  let p=setSmartDriverPreference(storage,{},drive,'sat').preferences;
  p=setSmartDriverPreference(storage,p,drive,'auto').preferences;
  assert.equal(getSmartDriverPreference(readSmartDriverPreferences(storage),drive),'auto');
  assert.deepEqual(Object.keys(p),[]);
});
test('volatile device paths are fingerprinted with size and model', () => {
  const a={...drive,id:'volatile:sdb'};
  const b={...a,size_bytes:2001};
  assert.notEqual(smartDriverDiskKey(a),smartDriverDiskKey(b));
  const storage=memoryStorage();
  setSmartDriverPreference(storage,{},a,'sat');
  assert.equal(getSmartDriverPreference(readSmartDriverPreferences(storage),b),'auto');
});
test('corrupt, malicious or malformed persisted preferences are ignored', () => {
  const storage=memoryStorage();
  storage.setItem(SMART_DRIVER_STORAGE_KEY,'{invalid');
  assert.deepEqual(readSmartDriverPreferences(storage),{});
  storage.setItem(SMART_DRIVER_STORAGE_KEY,JSON.stringify({'hw:serial-123':'sat;reboot', 'hw:ok':'sat'}));
  assert.deepEqual(readSmartDriverPreferences(storage),{'hw:ok':'sat'});
});
test('storage blocked: choice stays for the current session and warning is available', () => {
  const blocked={getItem(){throw Error('denied')},setItem(){throw Error('quota')}};
  assert.deepEqual(readSmartDriverPreferences(blocked),{});
  const result=setSmartDriverPreference(blocked,{},drive,'sat');
  assert.equal(result.saved,false);
  assert.equal(getSmartDriverPreference(result.preferences,drive),'sat');
});
test('invalid device, unknown driver and missing hardware metadata are rejected', () => {
  assert.equal(smartDriverDiskKey({device:'/dev/sdb1',id:'hw:part'}),'');
  assert.equal(smartDriverDiskKey({device:'/dev/sdb',id:'volatile:sdb',size_bytes:0}),'');
  const storage=memoryStorage();
  assert.equal(setSmartDriverPreference(storage,{},drive,'../../etc').saved,false);
  assert.equal(storage.getItem(SMART_DRIVER_STORAGE_KEY),null);
});
