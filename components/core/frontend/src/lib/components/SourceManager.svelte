<script>
  import { onMount } from 'svelte';
  import {
    getAppSources, previewAppSource, addAppSource, refreshAppSource,
    setAppSourceEnabled, removeAppSource, setAppSourceSecurity, getAppLegal
  } from '$lib/api.js';

  export let locale = 'ru';
  export let onclose = () => {};
  export let onchanged = () => {};

  let state = { sources:[], allow_unverified:false, agreement_version:'', agreement_accepted_at:'' };
  let loading = true;
  let busy = '';
  let error = '';
  let sourceURL = '';
  let sourceKind = 'auto';
  let preview = null;
  let riskOpen = false;
  let riskAccepted = false;
  let legalOpen = false;
  let legalText = '';
  let legalLoading = false;

  const ru = () => locale === 'ru';

  onMount(() => { void load(); });

  async function load() {
    loading = true; error = '';
    try { state = await getAppSources(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { loading = false; }
  }

  async function previewSource() {
    const url = sourceURL.trim();
    if (!url || busy) return;
    busy = 'preview'; error = ''; preview = null;
    try { preview = await previewAppSource({ url, kind:sourceKind }); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function addSource() {
    if (!preview || busy) return;
    busy = 'add'; error = '';
    try {
      await addAppSource({ url:sourceURL.trim(), kind:sourceKind });
      sourceURL = ''; preview = null;
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function refreshSource(source) {
    if (busy || source.read_only) return;
    busy = `refresh:${source.id}`; error = '';
    try { await refreshAppSource(source.id); await load(); await onchanged(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; await load(); }
    finally { busy = ''; }
  }

  async function toggleSource(source) {
    if (busy || source.read_only) return;
    busy = `toggle:${source.id}`; error = '';
    try { await setAppSourceEnabled(source.id, !source.enabled); await load(); await onchanged(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function deleteSource(source) {
    if (busy || source.read_only) return;
    const question = ru()
      ? `Удалить источник «${source.name}»? Уже установленные пакеты удалены не будут.`
      : `Remove source “${source.name}”? Already installed packages will not be removed.`;
    if (!window.confirm(question)) return;
    busy = `delete:${source.id}`; error = '';
    try { await removeAppSource(source.id); await load(); await onchanged(); }
    catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function disableUnsafe() {
    if (busy) return;
    busy = 'security'; error = '';
    try {
      await setAppSourceSecurity({ allow_unverified:false, accepted:false, agreement_version:state.agreement_version });
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function enableUnsafe() {
    if (!riskAccepted || busy) return;
    busy = 'security'; error = '';
    try {
      await setAppSourceSecurity({ allow_unverified:true, accepted:true, agreement_version:state.agreement_version });
      riskOpen = false; riskAccepted = false;
      await load(); await onchanged();
    } catch (e) { error = e?.payload?.error || e?.message || 'error'; }
    finally { busy = ''; }
  }

  async function showLegal() {
    legalOpen = true;
    if (legalText || legalLoading) return;
    legalLoading = true;
    try { const result = await getAppLegal(locale); legalText = result?.text || ''; }
    catch (e) { legalText = e?.payload?.error || e?.message || 'error'; }
    finally { legalLoading = false; }
  }

  function trustLabel(source) {
    const trust = String(source?.trust || 'unsigned').toUpperCase();
    if (trust === 'OFFICIAL') return ru() ? 'ОФИЦИАЛЬНЫЙ' : 'OFFICIAL';
    if (trust === 'VERIFIED') return ru() ? 'ПРОВЕРЕН' : 'VERIFIED';
    return ru() ? 'НЕПОДПИСАН' : 'UNSIGNED';
  }

  function sourceState(source) {
    if (!source.enabled) return { cls:'neutral', text:ru() ? 'ОТКЛЮЧЁН' : 'DISABLED' };
    if (source.online) return { cls:'good', text:'ONLINE' };
    if (source.cached) return { cls:'warn', text:ru() ? 'КЭШ' : 'CACHE' };
    return { cls:'error', text:'ERROR' };
  }
</script>

<div class="source-backdrop" role="presentation" onclick={onclose}>
  <section class="source-modal" role="dialog" aria-modal="true" aria-label={ru() ? 'Источники приложений' : 'Application sources'} onclick={(event) => event.stopPropagation()}>
    <div class="source-head">
      <div>
        <span class="mono eyebrow">ROUTERFORGE / SOURCES</span>
        <h2>{ru() ? 'Источники приложений' : 'Application sources'}</h2>
        <p>{ru() ? 'Официальный каталог, сторонние репозитории и отдельные приложения.' : 'Official catalog, third-party repositories and individual apps.'}</p>
      </div>
      <button class="icon-button" aria-label={ru() ? 'Закрыть' : 'Close'} onclick={onclose}>×</button>
    </div>

    {#if error}<div class="source-error">{error}</div>{/if}

    <section class="risk-panel" class:enabled={state.allow_unverified}>
      <div>
        <strong>{ru() ? 'Установка из непроверенных источников' : 'Install from unverified sources'}</strong>
        <p>{state.allow_unverified
          ? (ru() ? 'Разрешена. Каждая установка всё равно требует отдельного подтверждения.' : 'Enabled. Every install still requires separate confirmation.')
          : (ru() ? 'По умолчанию заблокирована.' : 'Blocked by default.')}</p>
      </div>
      {#if state.allow_unverified}
        <button class="button danger-subtle" disabled={Boolean(busy)} onclick={disableUnsafe}>{ru() ? 'Запретить' : 'Disable'}</button>
      {:else}
        <button class="button danger" disabled={Boolean(busy)} onclick={() => riskOpen = true}>{ru() ? 'Разрешить установку' : 'Allow installation'}</button>
      {/if}
    </section>

    <div class="source-list">
      {#if loading}
        <div class="source-empty">{ru() ? 'Загрузка…' : 'Loading…'}</div>
      {:else}
        {#each state.sources || [] as source (source.id)}
          {@const st = sourceState(source)}
          <article class="source-card">
            <div class="source-card-main">
              <div class="source-title-row">
                <strong>{source.name}</strong>
                <span class="state-chip {source.trust === 'official' ? 'official' : 'neutral'}">{trustLabel(source)}</span>
                <span class="state-chip {st.cls}">{st.text}</span>
              </div>
              <span class="mono source-url">{source.url}</span>
              <div class="source-meta">
                <span>{source.kind === 'app' ? (ru() ? 'Одно приложение' : 'Single app') : (ru() ? 'Репозиторий' : 'Repository')}</span>
                <span>{ru() ? 'Приложений' : 'Apps'}: {source.entry_count || 0}</span>
                {#if source.revision}<span>rev {source.revision}</span>{/if}
                {#if source.last_sync}<span>{ru() ? 'Синхр.' : 'Sync'}: {source.last_sync}</span>{/if}
              </div>
              {#if source.error}<div class="source-inline-error">{source.error}</div>{/if}
            </div>
            <div class="source-actions">
              {#if !source.read_only}
                <button class="button compact" disabled={Boolean(busy)} onclick={() => refreshSource(source)}>{ru() ? 'Проверить' : 'Refresh'}</button>
                <button class="button compact" disabled={Boolean(busy)} onclick={() => toggleSource(source)}>{source.enabled ? (ru() ? 'Отключить' : 'Disable') : (ru() ? 'Включить' : 'Enable')}</button>
                <button class="button danger-subtle compact" disabled={Boolean(busy)} onclick={() => deleteSource(source)}>{ru() ? 'Удалить' : 'Remove'}</button>
              {/if}
            </div>
          </article>
        {/each}
      {/if}
    </div>

    <section class="source-add">
      <div class="source-add-head">
        <strong>{ru() ? 'Добавить источник' : 'Add source'}</strong>
        <p>{ru() ? 'Репозиторий приложений, GitHub-репозиторий одного приложения или прямой HTTPS manifest.' : 'Application repository, a single GitHub app repository, or a direct HTTPS manifest.'}</p>
      </div>
      <div class="source-form">
        <select bind:value={sourceKind}>
          <option value="auto">{ru() ? 'Определить автоматически' : 'Auto-detect'}</option>
          <option value="repository">{ru() ? 'Репозиторий приложений' : 'Application repository'}</option>
          <option value="app">{ru() ? 'Отдельное приложение' : 'Single application'}</option>
        </select>
        <input bind:value={sourceURL} placeholder="https://github.com/owner/repo" />
        <button class="button" disabled={!sourceURL.trim() || Boolean(busy)} onclick={previewSource}>{busy === 'preview' ? (ru() ? 'Проверяем…' : 'Checking…') : (ru() ? 'Проверить' : 'Preview')}</button>
      </div>

      {#if preview}
        <div class="source-preview">
          <div class="source-title-row">
            <strong>{preview.name}</strong>
            <span class="state-chip warn">{ru() ? 'НЕПОДПИСАН' : 'UNSIGNED'}</span>
            <span class="state-chip neutral">{preview.kind === 'app' ? (ru() ? 'ПРИЛОЖЕНИЕ' : 'APP') : (ru() ? 'РЕПОЗИТОРИЙ' : 'REPOSITORY')}</span>
          </div>
          <div class="source-meta">
            <span>{ru() ? 'Приложений' : 'Apps'}: {preview.entry_count}</span>
            {#if preview.registry_id}<span>ID: {preview.registry_id}</span>{/if}
            {#if preview.revision}<span>rev {preview.revision}</span>{/if}
          </div>
          <span class="mono source-url">{preview.resolved_url}</span>
          <div class="source-preview-apps">
            {#each (preview.entries || []).slice(0, 8) as entry (entry.id)}
              <span>{entry.name}{#if entry.category} · {entry.category}{/if}{#if entry.package} · <code>{entry.package}</code>{/if}</span>
            {/each}
            {#if Number(preview.entry_count || 0) > 8}<span>+{Number(preview.entry_count) - 8}</span>{/if}
          </div>
          <div class="source-preview-warning">{ru()
            ? 'Источник не проверен RouterForge. R2 разрешает только безопасный opkg/manual lifecycle; произвольные install-скрипты и structured steps блокируются.'
            : 'This source is not verified by RouterForge. R2 only allows safe opkg/manual lifecycle; arbitrary install scripts and structured steps are blocked.'}</div>
          <button class="button primary" disabled={Boolean(busy)} onclick={addSource}>{busy === 'add' ? (ru() ? 'Добавляем…' : 'Adding…') : (ru() ? 'Добавить источник' : 'Add source')}</button>
        </div>
      {/if}
    </section>

    <footer class="source-footer">
      <a class="button compact" href="https://github.com/Fifth-Ace/routerforge" target="_blank" rel="noopener noreferrer">{ru() ? 'Исходный код' : 'Source code'}</a>
      <button class="legal-badge" onclick={showLegal}>{ru() ? 'Пользовательское соглашение · MIT · AS IS' : 'User Agreement · MIT · AS IS'}</button>
    </footer>
  </section>
</div>

{#if riskOpen}
  <div class="risk-backdrop" role="presentation" onclick={() => riskOpen = false}>
    <section class="risk-modal" role="alertdialog" aria-modal="true" onclick={(event) => event.stopPropagation()}>
      <span class="risk-kicker">{ru() ? 'ВНИМАНИЕ' : 'WARNING'}</span>
      <h3>{ru() ? 'Выполнение стороннего кода' : 'Executing third-party code'}</h3>
      <p>{ru()
        ? 'Непроверенное приложение может изменять конфигурацию роутера, файлы, службы и сетевые настройки. RouterForge не проверяет исходный код каждого неподписанного приложения и не гарантирует его безопасность или совместимость.'
        : 'An unverified application may modify router configuration, files, services and network settings. RouterForge does not review every unsigned application and cannot guarantee its security or compatibility.'}</p>
      <p>{ru()
        ? 'Это разрешение не отключает обязательные проверки RouterForge: подмена официального namespace, запрещённый lifecycle и некорректные manifests останутся заблокированы.'
        : 'This permission does not disable RouterForge hard safety checks: official namespace spoofing, forbidden lifecycle methods and invalid manifests remain blocked.'}</p>
      <label class="risk-check">
        <input type="checkbox" bind:checked={riskAccepted} />
        <span>{ru() ? 'Я прочитал предупреждение и принимаю риски установки стороннего кода.' : 'I have read the warning and accept the risks of installing third-party code.'}</span>
      </label>
      <div class="risk-actions">
        <button class="button" onclick={() => riskOpen = false}>{ru() ? 'Отмена' : 'Cancel'}</button>
        <button class="button danger" disabled={!riskAccepted || Boolean(busy)} onclick={enableUnsafe}>{ru() ? 'РАЗРЕШИТЬ УСТАНОВКУ' : 'ALLOW INSTALLATION'}</button>
      </div>
    </section>
  </div>
{/if}

{#if legalOpen}
  <div class="risk-backdrop" role="presentation" onclick={() => legalOpen = false}>
    <section class="legal-modal" role="dialog" aria-modal="true" onclick={(event) => event.stopPropagation()}>
      <div class="source-head">
        <div><span class="mono eyebrow">ROUTERFORGE / LEGAL</span><h3>{ru() ? 'Пользовательское соглашение' : 'User Agreement'}</h3></div>
        <button class="icon-button" onclick={() => legalOpen = false}>×</button>
      </div>
      <pre>{legalLoading ? (ru() ? 'Загрузка…' : 'Loading…') : legalText}</pre>
    </section>
  </div>
{/if}

<style>
  .source-backdrop,.risk-backdrop{position:fixed;inset:0;z-index:150;background:rgba(0,0,0,.84);display:grid;place-items:center;padding:1.2rem;backdrop-filter:none}
  .source-modal{width:min(1040px,96vw);max-height:92vh;overflow:auto;background:#0d1117;border:1px solid var(--rf-border,var(--border));border-radius:1rem;padding:1rem;box-shadow:0 24px 80px rgba(0,0,0,.62);backdrop-filter:none}
  .source-head{display:flex;justify-content:space-between;gap:1rem;align-items:flex-start;border-bottom:1px solid var(--rf-border,var(--border));padding-bottom:.8rem;margin-bottom:.9rem}
  .source-head h2,.source-head h3{margin:.15rem 0 .25rem}.source-head p{margin:0;color:var(--rf-muted,var(--muted))}
  .eyebrow{font-size:.7rem;color:var(--rf-good,#52d273)}
  .risk-panel{display:flex;justify-content:space-between;align-items:center;gap:1rem;padding:.8rem;border:1px solid rgba(247,185,85,.34);background:rgba(247,185,85,.06);border-radius:.75rem;margin-bottom:.9rem}
  .risk-panel.enabled{border-color:rgba(255,78,78,.45);background:rgba(255,78,78,.07)}
  .risk-panel p{margin:.2rem 0 0;color:var(--rf-muted,var(--muted))}
  .source-list{display:grid;gap:.55rem}
  .source-card{display:flex;justify-content:space-between;gap:1rem;align-items:center;border:1px solid var(--rf-border,var(--border));border-radius:.75rem;padding:.75rem;background:rgba(255,255,255,.018)}
  .source-card-main{min-width:0;display:grid;gap:.32rem}.source-title-row{display:flex;align-items:center;gap:.45rem;flex-wrap:wrap}
  .source-url{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--rf-muted,var(--muted));font-size:.75rem}
  .source-meta{display:flex;gap:.65rem;flex-wrap:wrap;color:var(--rf-muted,var(--muted));font-size:.75rem}
  .source-actions{display:flex;gap:.35rem;flex-wrap:wrap;justify-content:flex-end}
  .source-inline-error,.source-error{color:#ff8b8b;font-size:.78rem}.source-error{border:1px solid rgba(255,78,78,.35);padding:.65rem;border-radius:.6rem;margin-bottom:.8rem}
  .source-add{margin-top:1rem;border-top:1px solid var(--rf-border,var(--border));padding-top:1rem}.source-add-head p{margin:.2rem 0 .7rem;color:var(--rf-muted,var(--muted))}
  .source-form{display:grid;grid-template-columns:minmax(12rem,.55fr) minmax(16rem,1.5fr) auto;gap:.5rem}.source-form input,.source-form select{min-width:0}
  .source-preview{margin-top:.7rem;border:1px solid rgba(82,210,115,.25);border-radius:.75rem;padding:.8rem;display:grid;gap:.55rem}
  .source-preview-apps{display:flex;gap:.35rem;flex-wrap:wrap}.source-preview-apps span{font-size:.74rem;padding:.25rem .42rem;border:1px solid var(--rf-border,var(--border));border-radius:.45rem}
  .source-preview-warning{font-size:.78rem;color:var(--rf-muted,var(--muted))}
  .source-footer{display:flex;flex-direction:column;align-items:flex-start;gap:.32rem;margin-top:1rem;padding-top:.9rem;border-top:1px solid var(--rf-border,var(--border))}
  .legal-badge{border:0;background:transparent;color:var(--rf-muted,var(--muted));font:inherit;font-size:.7rem;padding:0;cursor:pointer;text-decoration:underline;text-decoration-style:dotted}
  .risk-modal,.legal-modal{width:min(700px,94vw);max-height:88vh;overflow:auto;background:#0d1117;border:1px solid rgba(255,78,78,.45);border-radius:1rem;padding:1rem;box-shadow:0 24px 80px rgba(0,0,0,.68);backdrop-filter:none}
  .risk-kicker{font-size:.7rem;font-weight:700;color:#ff6868;letter-spacing:.12em}.risk-modal h3{margin:.25rem 0 .7rem}.risk-modal p{color:var(--rf-muted,var(--muted));line-height:1.48}
  .risk-check{display:flex;gap:.55rem;align-items:flex-start;margin:1rem 0;padding:.7rem;border:1px solid rgba(255,78,78,.3);border-radius:.65rem}.risk-check input{margin-top:.2rem}
  .risk-actions{display:flex;justify-content:flex-end;gap:.5rem}.legal-modal{border-color:var(--rf-border,var(--border))}.legal-modal pre{white-space:pre-wrap;font:inherit;font-size:.82rem;line-height:1.5;color:var(--rf-text,var(--text))}
  .button.danger{border-color:rgba(255,78,78,.55);background:rgba(255,78,78,.15);color:#ff8585}.source-empty{padding:1rem;color:var(--rf-muted,var(--muted))}
  @media(max-width:760px){.source-card{align-items:flex-start;flex-direction:column}.source-actions{justify-content:flex-start}.source-form{grid-template-columns:1fr}.source-backdrop{padding:.4rem}.source-modal{width:100%;max-height:96vh}}
</style>
