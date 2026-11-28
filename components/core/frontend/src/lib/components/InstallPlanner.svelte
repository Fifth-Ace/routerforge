<script>
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';
  import { catalogWebPort, bytes, stateInfo } from '$lib/utils.js';
  import { catalogCategoryLabel } from '$lib/catalogPresentation.js';
  import { displayVersion, integrationDescription } from '$lib/app-center-ux.js';
  import CatalogIcon from './CatalogIcon.svelte';
  export let item;
  export let description='';
  export let warning='';
  export let canInstall=false;
  export let canUpdate=false;
  export let canRemove=false;
  export let disabled=false;
  export let initialTab='about';
  export let onaction=()=>{};
  export let onclose=()=>{};
  let activeTab='about';
  $: if (initialTab) activeTab=initialTab;
  let closeButton;
  let dialog;
  let priorFocus;
  const copy={ru:{about:'Обзор',install:'Установка',tech:'Технические данные',overview:'О приложении',platform:'Категория',state:'Состояние',source:'Источник',requirements:'Требования',manual:'Автоматическая установка недоступна. Откройте проект разработчика для инструкции.',noDescription:'Описание не предоставлено разработчиком.',installed:'Установлено',available:'Доступная версия',remove:'Удалить',update:'Обновить',installNow:'Установить',notes:'Примечания к установке',original:'Оригинальное описание',package:'Пакеты',management:'Управление приложением'},en:{about:'Overview',install:'Installation',tech:'Technical data',overview:'About this app',platform:'Category',state:'Status',source:'Source',requirements:'Requirements',manual:'Automatic installation is unavailable. Open the upstream project for instructions.',noDescription:'No description supplied by the publisher.',installed:'Installed',available:'Available version',remove:'Remove',update:'Update',installNow:'Install',notes:'Installation notes',original:'Original description',package:'Packages',management:'App management'}};
  $: locale=$settings.locale || 'ru';
  $: w=copy[locale] || copy.en;
  $: planAction=item?.installed && item?.update_available && item?.update?.method ? 'update' : 'install';
  $: install=planAction==='update'?(item?.update||{}):(item?.install||{});
  $: hints=item?.compatibility?.hints||[];
  $: packages=install.packages||item?.detection?.packages||[];
  $: notes=install.notes||[];
  $: meta=item?.package_meta||{};
  $: unmanaged=item?.unmanaged_github||null;
  $: webPort=catalogWebPort(item);
  $: about=description || integrationDescription(item,locale);
  $: status=stateInfo(item,locale);
  $: techRows=[
    [t(locale,'marketplace.planner.method'),install.method],
    [t(locale,'marketplace.planner.packages'),packages.join(', ')],
    [t(locale,'marketplace.planner.feed'),install.repository_url],
    [t(locale,'marketplace.planner.installer'),install.installer_url],
    [t(locale,'common.architecture'),meta.architecture || item?.release?.architecture],
    [t(locale,'marketplace.planner.downloadSize'),Number(meta.download_size_bytes||item?.release?.size_bytes||0)>0?bytes(meta.download_size_bytes||item.release.size_bytes):''],
    [t(locale,'marketplace.planner.installedSize'),Number(meta.installed_size_bytes||0)>0?bytes(meta.installed_size_bytes):''],
    [t(locale,'marketplace.planner.minCore'),item?.release?.min_core_version],
    [t(locale,'marketplace.planner.depends'),meta.depends?.join(', ')],
    [t(locale,'marketplace.planner.conflicts'),meta.conflicts?.join(', ')],
    [t(locale,'marketplace.planner.service'),item?.service],
    [t(locale,'marketplace.planner.webUi'),webPort?`:${webPort}`:''],
    [t(locale,'marketplace.planner.manifest'),item?.manifest_sha256],
    [t(locale,'marketplace.planner.githubChannel'),unmanaged?(unmanaged.selected_channel || unmanaged.requested_channel || 'auto'):''],
    [t(locale,'marketplace.planner.githubRelease'),unmanaged?.selected?.tag],
    [t(locale,'marketplace.planner.asset'),unmanaged?.selected?.asset],
    [t(locale,'marketplace.planner.autoInstall'),!unmanaged?.selected?unmanaged?.manual_reason:''],
    [locale==='ru'?'Статус проверки':'Trust',item?.trust?.status],[locale==='ru'?'Возможности':'Capabilities',item?.capabilities?.join(', ')]
  ].filter(row=>row[1]);
  onMount(()=>{priorFocus=document.activeElement;closeButton?.focus();return()=>{if(priorFocus?.isConnected)priorFocus.focus();};});
  function keyboard(event){
    if(event.key==='Escape'){event.stopPropagation();onclose();return;}
    if(event.key!=='Tab' || !dialog)return;
    const focusable=[...dialog.querySelectorAll('button:not(:disabled),a[href],summary,[tabindex="0"]')].filter(el=>el.getClientRects().length);
    const first=focusable[0],last=focusable[focusable.length-1];
    if(!first)return;
    if(event.shiftKey && (document.activeElement===first || !dialog.contains(document.activeElement))){event.preventDefault();last.focus();}
    else if(!event.shiftKey && (document.activeElement===last || !dialog.contains(document.activeElement))){event.preventDefault();first.focus();}
  }
</script>
<svelte:window onkeydown={keyboard}/>
{#if item}
  <div class="rf-app-detail-backdrop" role="presentation" onclick={(event)=>{if(event.target===event.currentTarget)onclose();}} transition:fade={{duration:120}}>
    <section bind:this={dialog} class="rf-app-detail" role="dialog" aria-modal="true" aria-label={item.name}>
      <header class="rf-detail-header">
        <div class="rf-detail-heading"><div class="rf-detail-icon"><CatalogIcon {item}/></div><div class="rf-detail-title"><h2>{item.name}</h2><span>{catalogCategoryLabel(item,locale)} · {item.publisher?.name || item.source || '—'}</span></div><button bind:this={closeButton} class="icon-button rf-detail-close" type="button" aria-label={t(locale,'common.close')} onclick={onclose}>×</button></div>
        <div class="rf-detail-status"><span class="state-chip {status.cls}">{status.label}</span>{#if item.version}<span class="mono">{displayVersion(item.version)}</span>{/if}{#if item.release?.version || item.available_version}<span class="muted">{w.available}: {displayVersion(item.release?.version || item.available_version)}</span>{/if}</div>
        <div class="rf-detail-tabs" aria-label={w.overview}>{#each ['about','install','tech'] as tab}<button type="button" class:active={activeTab===tab} aria-pressed={activeTab===tab} onclick={()=>activeTab=tab}>{w[tab]}</button>{/each}</div>
      </header>
      <div class="rf-detail-content">
        {#if activeTab==='about'}
          <h3>{w.overview}</h3><p class="rf-about">{about || w.noDescription}</p>
          <dl class="rf-detail-facts"><div><dt>{w.platform}</dt><dd>{catalogCategoryLabel(item,locale)}</dd></div><div><dt>{t(locale,'common.publisher')}</dt><dd>{item.publisher?.name || item.source || '—'}</dd></div><div><dt>{w.state}</dt><dd>{status.label}</dd></div><div><dt>{w.installed}</dt><dd>{displayVersion(item.version)}</dd></div></dl>
          {#if about && item.description && about!==item.description}<details class="rf-original"><summary>{w.original}</summary><p>{item.description}</p></details>{/if}
        {:else if activeTab==='install'}
          <h3>{w.requirements}</h3>
          {#if hints.length}<ul class="rf-requirements">{#each hints as hint}<li>{hint}</li>{/each}</ul>{:else}<p class="muted">{t(locale,'marketplace.planner.notDeclared')}</p>{/if}
          {#if packages.length}<p>{w.package}: <code>{packages.join(', ')}</code></p>{/if}
          {#if !item.installed && !canInstall}<p class="rf-detail-notice">{item.project_url?w.manual:(locale==='ru'?'Автоматическая установка недоступна.':'Automatic installation is unavailable.')}</p>{/if}
          {#if warning}<p class="rf-detail-notice">{warning}</p>{/if}
          {#if notes.length}<h3>{w.notes}</h3><ul class="rf-requirements">{#each notes as note}<li>{note}</li>{/each}</ul>{/if}
        {:else}
          <dl class="rf-tech-list">{#each techRows as row}<div><dt>{row[0]}</dt><dd class="mono">{row[1]}</dd></div>{/each}</dl>
          {#if item.trust?.note}<p class="muted">{item.trust.note}</p>{/if}
        {/if}
        <div class="rf-detail-management"><slot name="actions"/></div>
      </div>
      <footer class="rf-detail-footer">
        {#if item.project_url}<a class="button" href={item.project_url} target="_blank" rel="noopener noreferrer">{t(locale,'marketplace.planner.openProject')} ↗</a>{/if}
        <div class="rf-detail-primary">
          {#if !item.installed && canInstall}<button class="button primary" type="button" {disabled} onclick={()=>onaction('install')}>{w.installNow}</button>{/if}
          {#if item.installed && canUpdate}<button class="button primary" type="button" {disabled} onclick={()=>onaction('update')}>{w.update}</button>{/if}
          {#if item.installed && canRemove}<button class="button danger-subtle" type="button" {disabled} onclick={()=>onaction('remove')}>{w.remove}</button>{/if}
          <button class="button" type="button" onclick={onclose}>{t(locale,'common.close')}</button>
        </div>
      </footer>
    </section>
  </div>
{/if}
<style>
  .rf-app-detail-backdrop{position:fixed;inset:0;z-index:1100;display:grid;place-items:center;padding:12px;background:rgba(0,0,0,.58)}
  .rf-app-detail{width:min(660px,100%);max-height:calc(100dvh - 24px);display:flex;flex-direction:column;overflow:hidden;border:1px solid var(--rf-border,var(--border));border-radius:14px;background:var(--rf-panel,var(--panel));color:var(--rf-text,var(--text));box-shadow:0 20px 64px #0006;font-family:Inter,system-ui,sans-serif}
  .rf-detail-header{padding:18px 20px 0;border-bottom:1px solid var(--rf-border,var(--border));flex:none}.rf-detail-heading{display:flex;align-items:center;gap:12px}.rf-detail-icon{width:54px;height:54px;flex:none;display:grid;place-items:center;background:var(--rf-hover,var(--hover));color:var(--rf-accent,var(--accent));border:1px solid var(--rf-border,var(--border));border-radius:12px}.rf-detail-icon :global(svg){width:30px;height:30px}.rf-detail-title{min-width:0;flex:1}.rf-detail-title h2{margin:0;font-size:1.25rem;overflow-wrap:anywhere}.rf-detail-title span{display:block;font-size:.78rem;color:var(--rf-muted,var(--muted));margin-top:4px}.rf-detail-close{align-self:flex-start}.rf-detail-status{display:flex;align-items:center;gap:8px;flex-wrap:wrap;font-size:.78rem;margin:12px 0}.rf-detail-tabs{display:flex;flex-wrap:wrap;gap:4px}.rf-detail-tabs button{font:inherit;font-size:.82rem;background:transparent;color:var(--rf-muted,var(--muted));border:0;border-bottom:2px solid transparent;padding:10px;cursor:pointer}.rf-detail-tabs button.active{color:var(--rf-accent,var(--accent));border-color:currentColor;font-weight:600}
  .rf-detail-content{padding:18px 20px;overflow:auto;min-height:0;font-size:.88rem;line-height:1.5}.rf-detail-content h3{font-size:.94rem;margin:0 0 8px}.rf-detail-content p{margin:0 0 14px;white-space:pre-wrap;overflow-wrap:anywhere}.rf-about{line-height:1.6}.rf-detail-facts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;padding:12px 0;border-top:1px solid var(--rf-border,var(--border));margin:0}.rf-detail-facts dt,.rf-tech-list dt{font-size:.75rem;color:var(--rf-muted,var(--muted))}.rf-detail-facts dd,.rf-tech-list dd{margin:3px 0 0;overflow-wrap:anywhere}.rf-tech-list{margin:0}.rf-tech-list>div{display:grid;grid-template-columns:minmax(90px,35%) minmax(0,1fr);gap:10px;border-bottom:1px solid var(--rf-border,var(--border));padding:8px 0}.rf-tech-list dd{font-size:.75rem;margin:0}.rf-requirements{margin:0 0 14px;padding-left:20px}.rf-requirements li{margin:4px 0;overflow-wrap:anywhere}.rf-detail-notice{padding:10px;border-left:2px solid var(--rf-warn,#eab308);background:var(--rf-hover,var(--hover));border-radius:5px}.rf-original summary{font-size:.78rem;cursor:pointer;color:var(--rf-muted,var(--muted))}.rf-original p{margin:8px 0}.rf-detail-management{display:flex;gap:8px;flex-wrap:wrap;margin-top:12px}.rf-detail-footer{padding:12px 20px;border-top:1px solid var(--rf-border,var(--border));display:flex;justify-content:space-between;align-items:center;gap:8px;flex-wrap:wrap;flex:none}.rf-detail-primary{display:flex;gap:6px;flex-wrap:wrap;margin-left:auto}
  @media(max-width:480px){.rf-detail-header,.rf-detail-content,.rf-detail-footer{padding-left:12px;padding-right:12px}.rf-detail-tabs button{padding:8px 6px;font-size:.76rem}.rf-detail-facts{grid-template-columns:1fr}.rf-tech-list>div{grid-template-columns:1fr;gap:2px}}
</style>
