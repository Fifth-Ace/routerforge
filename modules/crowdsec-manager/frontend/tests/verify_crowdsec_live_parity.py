#!/usr/bin/env python3
"""CrowdSec migration browser acceptance, no network or real API effects.

Compare current Svelte production bundle to immutable legacy Git source for
healthy/degraded API fixtures at desktop and mobile. Assert exact pixel equality,
original ID order, status verdict, and no JavaScript errors.

Requires: Python playwright, Pillow, Chromium installed on test runner.
"""
import argparse
import base64
import json
import re
import subprocess
import asyncio
from pathlib import Path
from html.parser import HTMLParser
from PIL import Image, ImageChops
from playwright.async_api import async_playwright

PINNED_BASE = '280040812dcd97940bfe89de2ba8f6fae3afbd53'

class Refs(HTMLParser):
    def __init__(self):
        super().__init__()
        self.src = []
        self.css = []
    def handle_starttag(self, tag, attrs):
        d = dict(attrs)
        if tag == 'script' and d.get('src'):
            self.src.append(d['src'])
        if tag == 'link' and d.get('rel') == 'stylesheet' and d.get('href'):
            self.css.append(d['href'])


def legacy_file(root: Path, path: str, supplied: Path | None) -> str:
    if supplied:
        return (supplied / path.rsplit('/', 1)[-1]).read_text('utf-8')
    return subprocess.check_output(['git','show', PINNED_BASE + ':' + path],cwd=root).decode('utf-8')


def all_data_urls(index: Path, build: Path, fallback_css: str):
    index_html = index.read_text('utf-8')
    refs = Refs();refs.feed(index_html)
    if len(refs.src) != 1 or not refs.src[0].startswith('./assets/'):
        raise ValueError('BUNDLE_ENTRY_NOT_EXACTLY_ONE_STATIC_ASSET')
    jsname = refs.src[0].lstrip('./')
    jsfile = build / jsname
    if not jsfile.is_file() or jsfile.stat().st_size < 1000:
        raise ValueError('COMPILED_SVELTE_ENTRY_MISSING')
    code = jsfile.read_bytes()
    # Deliberately reject dynamic imports: all runtime logic must be in shipped
    # assets resolved and checked here, not an untracked external module.
    if re.search(rb'\bimport\s*\(', code):
        raise ValueError('DYNAMIC_IMPORT_REQUIRES_EXPLICIT_ASSET_SUPPORT')
    styles=[]
    for path in refs.css:
        if not path.startswith('./assets/'):
            raise ValueError('UNEXPECTED_CSS_ASSET '+path)
        f=build/path.lstrip('./')
        if not f.is_file() or not f.read_bytes():
            raise ValueError('CSS_ASSET_MISSING '+path)
        styles.append(f.read_text('utf-8'))
    if not styles: # module CSS must be in the compiled bundle, fail if not.
        raise ValueError('COMPILED_STYLES_MISSING')
    # Vite may hoist CSS and combine it; enforce original CSS selectors instead
    # of replacing compiled styles with legacy CSS.
    merged='\n'.join(styles)
    for marker in ('.cs', '.stats', '.gridtwo', '--cs-panel'):
        if marker not in merged:
            raise ValueError('VISUAL_STYLES_MISSING '+marker)
    # Vite may optimize whitespace; the stylesheet is nevertheless required to
    # produce pixel-equivalent output in the browser.
    url='data:text/javascript;base64,'+base64.b64encode(code).decode()
    return f'''<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>{merged}</style></head><body><div id="app"></div><script type="module" src="{url}"></script></body></html>'''


def make_html(legacy: Path | None, root: Path, build: Path) -> tuple[str,str]:
    rel='modules/crowdsec-manager/frontend/'
    old=legacy_file(root,rel+'index.html',legacy)
    old_js=legacy_file(root,rel+'app.js',legacy)
    old_css=legacy_file(root,rel+'module.css',legacy)
    if old.count('<script src="app.js"></script>')!=1:
        raise ValueError('LEGACY_SCRIPT_ANCHOR_CHANGED')
    if old.count('<link rel="stylesheet" href="module.css">')!=1:
        raise ValueError('LEGACY_CSS_ANCHOR_CHANGED')
    old=old.replace('<link rel="stylesheet" href="module.css">','<style>'+old_css+'</style>')
    old=old.replace('<script src="app.js"></script>','<script>'+old_js+'</script>')
    return old,all_data_urls(build/'index.html',build,old_css)


def inject_api(html: str, overview: dict, installer: dict) -> str:
    js=(
        '<script>'
        'window.__rf_calls=[];window.__rf_confirm=[];'
        'window.confirm=q=>{window.__rf_confirm.push(q);return true};'
        'window.__rf_overview='+json.dumps(overview,ensure_ascii=False)+';'
        'window.__rf_installer='+json.dumps(installer,ensure_ascii=False)+';'
        'window.fetch=async (url,opts={})=>{const p=String(url).split("crowdsec-manager/").pop();'
        'window.__rf_calls.push({path:p,method:opts.method||"GET",body:opts.body||null});'
        'const d=p==="overview"?window.__rf_overview:p==="install/status"?window.__rf_installer:{ok:true};'
        'return {ok:true,status:200,json:async()=>d}};'
        '</script>'
    )
    return html.replace('</head>','</head>'+js,1)


async def browser_check(args):
    root=Path(args.repo_root).resolve()
    legacy=Path(args.legacy_dir).resolve() if args.legacy_dir else None
    old,modern=make_html(legacy,root,Path(args.build_dir).resolve())
    fixtures=Path(args.fixtures).resolve()
    dest=Path(args.output).resolve()
    dest.mkdir(parents=True,exist_ok=True)
    async with async_playwright() as p:
        browser=await p.chromium.launch(headless=True,
            executable_path=args.chromium or None,
            args=['--no-sandbox','--disable-extensions'])
        for state in ('healthy','degraded'):
            overview=json.loads((fixtures/(state+'-overview.json')).read_text('utf-8'))
            installer=json.loads((fixtures/(state+'-installer.json')).read_text('utf-8'))
            for width in (1280,375):
                compared=[]
                for name,source in [('legacy',old),('svelte',modern)]:
                    page=await browser.new_page(viewport={'width':width,'height':900},device_scale_factor=1)
                    errors=[]
                    page.on('pageerror',lambda e:errors.append(str(e)))
                    await page.set_content(inject_api(source,overview,installer),timeout=30000)
                    await page.wait_for_timeout(450)
                    shot=dest/f'{state}-{name}-{width}.png'
                    await page.screenshot(path=str(shot),full_page=True)
                    data=await page.evaluate('''() => ({ids:[...document.querySelectorAll('[id]')].map(x=>x.id).filter(x=>x!=='app'),verdict:document.getElementById('verdict')?.textContent||'',initialApiCalls:window.__rf_calls?.length||0})''')
                    if errors or len(data['ids'])!=45 or data['initialApiCalls']<2:
                        raise AssertionError(f'RUNTIME_FAIL {state}/{name}/{width}: errors={errors} metadata={data}')
                    compared.append((shot,data))
                    await page.close()
                if compared[0][1]['ids']!=compared[1][1]['ids']:
                    raise AssertionError('ID_ORDER_CHANGED '+state+'/'+str(width))
                if compared[0][1]['verdict']!=compared[1][1]['verdict']:
                    raise AssertionError('VERDICT_CHANGED '+state+'/'+str(width))
                a,b=[Image.open(x[0]).convert('RGB') for x in compared]
                if a.size!=b.size:
                    raise AssertionError(f'PAGE_SIZE_CHANGED {state}/{width} {a.size} {b.size}')
                diff=ImageChops.difference(a,b)
                changed=sum(1 for rgb in diff.getdata() if any(rgb))
                if changed:
                    diff.save(dest/f'{state}-diff-{width}.png')
                    raise AssertionError(f'PIXEL_PARITY_FAIL {state}/{width} changed={changed}')
                print('CROWDSEC_LIVE_PIXEL_PARITY: PASS /',state,'/',width,'/ 0 changed pixels / 45 ordered IDs')
        await browser.close()
    print('CROWDSEC_PRODUCTION_SVELTE_BROWSER: PASS / 4 scenarios / no network / no router modifications')


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--repo-root',required=True)
    parser.add_argument('--build-dir',required=True)
    parser.add_argument('--legacy-dir')
    parser.add_argument('--fixtures',required=True)
    parser.add_argument('--output',required=True)
    parser.add_argument('--chromium')
    args=parser.parse_args()
    asyncio.run(browser_check(args))
if __name__=='__main__':main()
