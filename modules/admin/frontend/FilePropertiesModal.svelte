<script>
  import { getAdminFileHash, adminFileDownloadURL } from '$lib/api.js';
  import { bytes } from '$lib/utils.js';

  export let entry = null;
  export let writable = true;
  export let locale = 'ru';
  export let onClose = () => {};
  export let onEdit = () => {};
  export let onApplyMode = async () => {};

  let hashBusy = '';
  let hashes = { md5: '', sha256: '' };
  let perm = modeToPerm(entry?.mode || '');
  let applying = false;
  let errorText = '';

  $: octal = permToOctal(perm);
  $: symbolic = permToSymbolic(perm, entry?.kind === 'directory');

  $: copy = locale === 'ru' ? {
    title: 'Свойства объекта', type: 'Тип', file: 'Файл', directory: 'Папка', other: 'Объект',
    size: 'Размер', modified: 'Изменён', path: 'Путь', permissions: 'Права доступа',
    owner: 'Владелец', group: 'Группа', others: 'Остальные', read: 'Чтение (r)',
    write: 'Запись (w)', execute: 'Запуск (x)', apply: 'Применить права',
    presets: 'Готовые пресеты', hashes: 'Контрольные суммы', calculate: 'Рассчитать',
    download: 'Скачать', edit: 'Редактировать', close: 'Закрыть',
    readonly: 'Этот том доступен только для чтения', warningZero: 'Внимание: режим 0000 полностью закрывает доступ к объекту.',
    warningDir: 'Без execute (x) каталог может стать недоступен для входа.'
  } : {
    title: 'Object properties', type: 'Type', file: 'File', directory: 'Folder', other: 'Object',
    size: 'Size', modified: 'Modified', path: 'Path', permissions: 'Permissions',
    owner: 'Owner', group: 'Group', others: 'Others', read: 'Read (r)',
    write: 'Write (w)', execute: 'Execute (x)', apply: 'Apply permissions',
    presets: 'Presets', hashes: 'Checksums', calculate: 'Calculate',
    download: 'Download', edit: 'Edit', close: 'Close',
    readonly: 'This volume is read-only', warningZero: 'Warning: mode 0000 removes all access.',
    warningDir: 'Without execute (x), entering this directory may become impossible.'
  };

  const filePresets = [
    ['0600', 'Private'], ['0644', 'Default'], ['0664', 'Group write'],
    ['0755', 'Executable'], ['0775', 'Group exec']
  ];
  const dirPresets = [['0700', 'Private'], ['0755', 'Default'], ['0775', 'Group write']];

  function modeToPerm(mode) {
    const s = String(mode || '');
    const tail = s.length >= 9 ? s.slice(-9) : '---------';
    return {
      ur: tail[0] === 'r', uw: tail[1] === 'w', ux: ['x','s','t'].includes(tail[2]),
      gr: tail[3] === 'r', gw: tail[4] === 'w', gx: ['x','s','t'].includes(tail[5]),
      or: tail[6] === 'r', ow: tail[7] === 'w', ox: ['x','s','t'].includes(tail[8])
    };
  }

  function permToOctal(p) {
    const u = (p.ur ? 4 : 0) + (p.uw ? 2 : 0) + (p.ux ? 1 : 0);
    const g = (p.gr ? 4 : 0) + (p.gw ? 2 : 0) + (p.gx ? 1 : 0);
    const o = (p.or ? 4 : 0) + (p.ow ? 2 : 0) + (p.ox ? 1 : 0);
    return `0${u}${g}${o}`;
  }

  function permToSymbolic(p, directory) {
    return `${directory ? 'd' : '-'}${p.ur?'r':'-'}${p.uw?'w':'-'}${p.ux?'x':'-'}${p.gr?'r':'-'}${p.gw?'w':'-'}${p.gx?'x':'-'}${p.or?'r':'-'}${p.ow?'w':'-'}${p.ox?'x':'-'}`;
  }

  function setPreset(mode) {
    const d = mode.replace(/^0/, '').split('').map(Number);
    perm = {
      ur: !!(d[0]&4), uw: !!(d[0]&2), ux: !!(d[0]&1),
      gr: !!(d[1]&4), gw: !!(d[1]&2), gx: !!(d[1]&1),
      or: !!(d[2]&4), ow: !!(d[2]&2), ox: !!(d[2]&1)
    };
  }

  async function applyMode() {
    if (!writable || applying) return;
    applying = true; errorText = '';
    try { await onApplyMode(octal); }
    catch (error) { errorText = error?.payload?.error || error?.message || String(error); }
    finally { applying = false; }
  }

  async function calculate(kind) {
    if (!entry || entry.kind !== 'file' || hashBusy) return;
    hashBusy = kind; errorText = '';
    try {
      const result = await getAdminFileHash(entry.path, kind);
      hashes = { ...hashes, [kind]: result.digest || '' };
    } catch (error) {
      errorText = error?.payload?.error || error?.message || String(error);
    } finally { hashBusy = ''; }
  }

  function typeLabel() {
    if (entry?.kind === 'file') return copy.file;
    if (entry?.kind === 'directory') return copy.directory;
    return copy.other;
  }
</script>

<div class="backdrop" onclick={onClose}></div>
<section class="modal" role="dialog" aria-modal="true" aria-label={copy.title}>
  <header>
    <div>
      <strong>{copy.title}</strong>
      <div class="object">
        <span class="icon">{entry?.kind === 'directory' ? '📁' : '📄'}</span>
        <div><b>{entry?.name || ''}</b><small>{entry?.path || ''}</small></div>
      </div>
    </div>
    <button class="close" onclick={onClose}>×</button>
  </header>

  <div class="body">
    <section class="card facts">
      <div><span>{copy.type}</span><b>{typeLabel()}</b></div>
      <div><span>{copy.size}</span><b>{entry?.kind === 'file' ? bytes(entry?.size || 0) : '—'}</b></div>
      <div><span>{copy.modified}</span><b>{entry?.modified_at ? new Date(entry.modified_at).toLocaleString() : '—'}</b></div>
      <div class="path-row"><span>{copy.path}</span><code>{entry?.path || ''}</code></div>
    </section>

    <section class="card">
      <div class="card-title"><span>🛡</span><b>{copy.permissions} (chmod {octal})</b></div>
      {#if !writable}<div class="readonly-banner">{copy.readonly}</div>{/if}

      <div class="perm-grid">
        <span></span><span>{copy.read}</span><span>{copy.write}</span><span>{copy.execute}</span>
        <b>{copy.owner}</b><input type="checkbox" bind:checked={perm.ur} disabled={!writable}/><input type="checkbox" bind:checked={perm.uw} disabled={!writable}/><input type="checkbox" bind:checked={perm.ux} disabled={!writable}/>
        <b>{copy.group}</b><input type="checkbox" bind:checked={perm.gr} disabled={!writable}/><input type="checkbox" bind:checked={perm.gw} disabled={!writable}/><input type="checkbox" bind:checked={perm.gx} disabled={!writable}/>
        <b>{copy.others}</b><input type="checkbox" bind:checked={perm.or} disabled={!writable}/><input type="checkbox" bind:checked={perm.ow} disabled={!writable}/><input type="checkbox" bind:checked={perm.ox} disabled={!writable}/>
      </div>

      <div class="mode-preview"><code>{octal}</code><code>{symbolic}</code></div>
      <div class="preset-title">{copy.presets}</div>
      <div class="presets">
        {#each (entry?.kind === 'directory' ? dirPresets : filePresets) as [mode, label]}
          <button onclick={() => setPreset(mode)} disabled={!writable} class:active={octal === mode}><b>{mode}</b><span>{label}</span></button>
        {/each}
      </div>

      {#if octal === '0000'}<div class="warn">{copy.warningZero}</div>{/if}
      {#if entry?.kind === 'directory' && !(perm.ux || perm.gx || perm.ox)}<div class="warn">{copy.warningDir}</div>{/if}
      <div class="apply-row"><button class="primary" onclick={applyMode} disabled={!writable || applying}>{applying ? '…' : `${copy.apply} (${octal})`}</button></div>
    </section>

    {#if entry?.kind === 'file'}
      <section class="card">
        <div class="card-title"><span>#</span><b>{copy.hashes}</b></div>
        <div class="hash-row"><span>MD5:</span>{#if hashes.md5}<code>{hashes.md5}</code>{:else}<button onclick={() => calculate('md5')} disabled={!!hashBusy}>{hashBusy === 'md5' ? '…' : copy.calculate}</button>{/if}</div>
        <div class="hash-row"><span>SHA256:</span>{#if hashes.sha256}<code>{hashes.sha256}</code>{:else}<button onclick={() => calculate('sha256')} disabled={!!hashBusy}>{hashBusy === 'sha256' ? '…' : copy.calculate}</button>{/if}</div>
      </section>
    {/if}

    {#if errorText}<div class="error">{errorText}</div>{/if}
  </div>

  <footer>
    <div>
      {#if entry?.kind === 'file'}<a class="button" href={adminFileDownloadURL(entry.path)}>⇩ {copy.download}</a><button onclick={onEdit}>▤ {copy.edit}</button>{/if}
    </div>
    <button onclick={onClose}>{copy.close}</button>
  </footer>
</section>

<style>
  .backdrop{position:fixed;inset:0;z-index:1000;background:rgba(0,0,0,.62);backdrop-filter:blur(2px)}
  .modal{position:fixed;z-index:1001;top:50%;left:50%;transform:translate(-50%,-50%);width:min(46rem,calc(100vw - 2rem));max-height:88vh;display:grid;grid-template-rows:auto 1fr auto;background:#0d1117;color:#e6edf3;border:1px solid #30363d;border-radius:.9rem;box-shadow:0 24px 80px rgba(0,0,0,.55);overflow:hidden;font-family:Inter,system-ui,sans-serif}
  header{display:flex;justify-content:space-between;padding:1rem 1rem .85rem;border-bottom:1px solid #252c34;background:#0b0f14}header>div>strong{font-size:.94rem}.close{font-size:1.25rem;width:2rem;height:2rem;padding:0}
  .object{display:flex;gap:.7rem;align-items:center;margin-top:.75rem;padding:.7rem;border:1px solid #28313a;background:#151b22;border-radius:.5rem}.icon{font-size:1.8rem}.object div{min-width:0;display:flex;flex-direction:column;gap:.15rem}.object b{font-size:.83rem}.object small{font:600 .62rem/1.4 "Roboto Mono",monospace;color:#7d91a5;overflow:hidden;text-overflow:ellipsis}
  .body{overflow:auto;padding:.75rem;display:flex;flex-direction:column;gap:.65rem}.card{border:1px solid #29313a;border-radius:.55rem;background:#0f141a;padding:.75rem}.facts{display:grid;grid-template-columns:1fr 1fr;gap:.65rem 1.2rem}.facts div{display:flex;justify-content:space-between;gap:1rem}.facts span,.preset-title{color:#8292a3;font-size:.67rem}.facts b{font-size:.72rem}.path-row{grid-column:1/-1}.path-row code{font-size:.62rem;color:#7bd88f;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .card-title{display:flex;align-items:center;gap:.4rem;margin-bottom:.75rem;font-size:.72rem}.readonly-banner,.warn,.error{padding:.5rem .65rem;border-radius:.4rem;font-size:.65rem}.readonly-banner{background:#231d38;color:#c6a7ff;border:1px solid #584b7d}.warn{margin-top:.55rem;background:#2b2112;color:#ffd866;border:1px solid #6d5829}.error{background:#31181c;color:#ff9da4;border:1px solid #6d363e}
  .perm-grid{display:grid;grid-template-columns:minmax(7rem,1fr) repeat(3,minmax(5rem,7rem));gap:.45rem .75rem;align-items:center}.perm-grid>span{text-align:center;color:#8292a3;font-size:.61rem}.perm-grid>b{font-size:.67rem}.perm-grid input{justify-self:center;width:1rem;height:1rem;accent-color:#5de4c7}
  .mode-preview{display:flex;gap:.45rem;margin-top:.75rem}.mode-preview code{padding:.3rem .45rem;border:1px solid #33414e;border-radius:.35rem;color:#8bc8ff;background:#0a0e13;font-size:.67rem}
  .preset-title{margin-top:.75rem;margin-bottom:.35rem}.presets{display:flex;flex-wrap:wrap;gap:.35rem}.presets button{display:flex;gap:.35rem;align-items:center}.presets button.active{border-color:#5de4c7;background:#12362f}.presets button span{font-size:.58rem;color:#8495a5}
  .apply-row{display:flex;justify-content:flex-end;margin-top:.75rem}.primary{border-color:#3f7f6f;background:#16483c;color:#bff9e7;font-weight:750}
  .hash-row{display:grid;grid-template-columns:5rem minmax(0,1fr);gap:.5rem;align-items:center;margin:.38rem 0}.hash-row>span{font:700 .64rem "Roboto Mono",monospace;color:#9eb0c1}.hash-row code{font-size:.61rem;color:#8bc8ff;overflow-wrap:anywhere}.hash-row button{justify-self:start}
  footer{display:flex;justify-content:space-between;align-items:center;padding:.7rem .8rem;border-top:1px solid #252c34;background:#0b0f14}footer>div{display:flex;gap:.4rem}.button,button{font:inherit;font-size:.67rem;color:#dbe5ee;border:1px solid #33404c;background:#161d25;border-radius:.4rem;padding:.42rem .6rem;text-decoration:none;cursor:pointer}.button:hover,button:hover:not(:disabled){background:#202a34;border-color:#52687a}button:disabled{opacity:.4;cursor:not-allowed}
  @media(max-width:640px){.facts{grid-template-columns:1fr}.path-row{grid-column:auto}.perm-grid{grid-template-columns:5rem repeat(3,1fr)}.perm-grid>span{font-size:.55rem}}
</style>
