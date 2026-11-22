// Post-Chromium acceptance: fail closed unless *every* actual case ran and passed.
// This is NOT a browser test itself; it validates the real browser output.
import fs from 'node:fs';
import path from 'node:path';
const dir=process.env.RF_RTC3_EVIDENCE;
if(!dir||!path.isAbsolute(dir))throw Error('EVIDENCE_DIR_MISSING');
const src=path.join(dir,'RESULT.json');
if(!fs.existsSync(src))throw Error('BROWSER_RESULT_MISSING');
const report=JSON.parse(fs.readFileSync(src,'utf8'));
const EXPECTED_CASE_IDS=Array.from({length:34},(_,i)=>'RTC3-'+String(i+1).padStart(2,'0'));
if(report.scope!=='REAL_COMPILED_SVELTE_LOCALHOST_BROWSER')throw Error('SCOPE_MISMATCH');
if(!Array.isArray(report.cases)||report.cases.length!==34)throw Error('BROWSER_CASE_COUNT_NOT_34');
const ids=report.cases.map(c=>c.id);
if(ids.length!==new Set(ids).size || JSON.stringify([...ids].sort())!==JSON.stringify(EXPECTED_CASE_IDS))
 throw Error('BROWSER_CASE_SET_MISMATCH');
for(const case_ of report.cases){
 if(case_.status!=='PASS')throw Error('BROWSER_CASE_NOT_PASS: '+case_.id);
 const log=path.join(dir,case_.id+'.json');
 if(!fs.existsSync(log))throw Error('CASE_LOG_MISSING: '+case_.id);
 const details=JSON.parse(fs.readFileSync(log,'utf8'));
 if(details.status!=='PASS'||!Array.isArray(details.calls)||!Array.isArray(details.exceptions))throw Error('CASE_EVIDENCE_INVALID: '+case_.id);
 if(details.exceptions.length)throw Error('CASE_BROWSER_EXCEPTION: '+case_.id);
}
for(const image of ['desktop_1440x900.png','mobile_390x844.png','mobile_advanced_390x844.png']) {
 if(!fs.existsSync(path.join(dir,image)))throw Error('BROWSER_VIEWPORT_IMAGE_MISSING: '+image);
}
console.log('RT_C3_EVIDENCE: PASS / 34 executed browser cases / real compiled Svelte');
