#!/usr/bin/env python3
"""CrowdSec browser action parity after production Svelte Vite build.
All endpoints are mocked; no real service or router is accessed.
"""
import argparse,asyncio,json
from pathlib import Path
from playwright.async_api import async_playwright
from verify_crowdsec_live_parity import make_html, inject_api

async def perform(browser, source, task):
    page=await browser.new_page(viewport={'width':1280,'height':900})
    errors=[]
    page.on('pageerror',lambda e:errors.append(str(e)))
    fixtures=Path(task['fixtures'])
    ov=json.loads((fixtures/'healthy-overview.json').read_text())
    inst=json.loads((fixtures/'healthy-installer.json').read_text())
    await page.set_content(inject_api(source,ov,inst),timeout=30000)
    await page.wait_for_timeout(300)
    if task['name']=='lapi':
        await page.locator('#lapi-url').fill('https://10.1.2.3:8080')
        await page.locator('#lapi-key').fill('test-key-not-real')
        await page.locator('#lapi-form').evaluate('(f)=>f.requestSubmit()')
    else:await page.locator(task['selector']).click()
    await page.wait_for_timeout(250)
    data=await page.evaluate('''() => ({calls:window.__rf_calls.filter(x=>x.method==='POST').map(x=>({path:x.path,method:x.method,body:JSON.parse(x.body)})),confirmed:window.__rf_confirm,toast:document.querySelector('#toast')?.textContent||''})''')
    await page.close()
    if errors:
        raise AssertionError('BROWSER_JS_ERRORS '+str(errors))
    return data

async def main(args):
    legacy=Path(args.legacy_dir).resolve() if args.legacy_dir else None
    root=Path(args.repo_root).resolve()
    old,new=make_html(legacy,root,Path(args.build_dir).resolve())
    cases=[
        {'name':'service','selector':'[data-service="restart"]'},
        {'name':'hook','selector':'#hook-remove'},
        {'name':'lapi','selector':'#lapi-form'},
        {'name':'remove','selector':'#install-remove'},
    ]
    async with async_playwright() as p:
        browser=await p.chromium.launch(headless=True,executable_path=args.chromium or None,args=['--no-sandbox'])
        for case in cases:
            case['fixtures']=args.fixtures
            a=await perform(browser,old,case)
            b=await perform(browser,new,case)
            if a!=b or len(a['calls'])!=1 or len(a['confirmed'])!=1:
                raise AssertionError('CROWDSEC_ACTION_PARITY_FAIL '+case['name']+' '+json.dumps({'old':a,'new':b},ensure_ascii=False))
            print('CROWDSEC_ACTION_PARITY: PASS / '+case['name']+' / exact POST+confirm+status')
        await browser.close()
    print('CROWDSEC_ACTION_PARITY: PASS / 4 browser actions / mock API only')

if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('--repo-root',required=True)
    parser.add_argument('--build-dir',required=True)
    parser.add_argument('--legacy-dir')
    parser.add_argument('--fixtures',required=True)
    parser.add_argument('--chromium')
    args=parser.parse_args()
    asyncio.run(main(args))
