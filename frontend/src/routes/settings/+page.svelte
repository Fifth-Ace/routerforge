<script>
  import { onMount } from 'svelte';
  import { snapshot } from '$lib/stores/snapshot.js';
  import { settings, themes } from '$lib/stores/settings.js';
  import { getSystem } from '$lib/api.js';
  import { fmtInt, fmtDuration, fmtAgo } from '$lib/utils.js';

  let system={};
  const themeCards=[['console','DNSM · Console','Текущий инфраструктурный стиль проекта.'],['legacy','DNSM · Legacy','Глубокие тёмно-синие оттенки.'],['neo','DNSM · Neo','Высокая контрастность и яркий акцент.'],['mint','DNSM · Mint','Мягкая серо-синяя палитра.'],['custom','DNSM · Custom','Три базовых цвета, остальные вычисляются.']];
  const update=(key,value)=>settings.update((current)=>({...current,[key]:value}));
  onMount(async()=>{try{system=await getSystem();}catch{}});
</script>

<svelte:head><title>DNS Monitor — Настройки</title></svelte:head>

<div class="page">
  <div class="page-head"><div><h1>Настройки</h1><p>Интерфейс, частота live-обновлений и визуальная тема.</p></div><span class="page-kicker mono">LOCAL UI SETTINGS</span></div>
  <div class="settings-layout">
    <aside class="system-card panel"><div class="panel-head"><div><strong>Система</strong><span>DNS Monitor</span></div><span class="state-chip {$snapshot.capture_error?'error':'good'}">{$snapshot.capture_error?'ERROR':'OK'}</span></div><div class="system-row"><span>DNS Monitor</span><strong>v{$snapshot.version||'—'}</strong></div><div class="system-row"><span>Статус</span><strong>{$snapshot.active_down?'ERROR':$snapshot.active_degraded?'DEGRADED':'OK'}</strong></div><div class="system-row"><span>Uptime</span><strong>{fmtDuration($snapshot.uptime_seconds)}</strong></div><div class="system-row"><span>Upstream</span><strong>{fmtInt($snapshot.upstream_count)}</strong></div><div class="system-row"><span>Запросы</span><strong>{fmtInt($snapshot.total_requests)}</strong></div><div class="system-row"><span>Discovery</span><strong>{fmtAgo($snapshot.last_discovery)}</strong></div><div class="system-row"><span>RAM</span><strong>{system?.rss_kb?`${(system.rss_kb/1024).toFixed(1)} MB`:'—'}</strong></div></aside>

    <div class="settings-stack">
      <section class="settings-section panel"><div class="settings-title">Общие</div><div class="setting-row"><div><strong>Уровень интерфейса</strong><span>Скрывает технические поля при обычном использовании.</span></div><select value={$settings.uiLevel} onchange={(e)=>update('uiLevel',e.currentTarget.value)}><option value="normal">Обычный</option><option value="expert">Экспертный</option></select></div><div class="setting-row"><div><strong>Ширина интерфейса</strong><span>Компактный режим ограничивает рабочую область.</span></div><select value={$settings.compact?'compact':'classic'} onchange={(e)=>update('compact',e.currentTarget.value==='compact')}><option value="classic">Классическая</option><option value="compact">Компактная</option></select></div><div class="setting-row"><div><strong>Live update</strong><span>Частота SSE snapshot. При проблемах SSE автоматически включается polling.</span></div><select value={$settings.refreshMs} onchange={(e)=>update('refreshMs',Number(e.currentTarget.value))}><option value="1000">1 сек</option><option value="2000">2 сек</option><option value="5000">5 сек</option><option value="10000">10 сек</option></select></div></section>

      <section class="settings-section panel"><div class="settings-title">Внешний вид</div><div class="theme-grid">{#each themeCards as [id,title,description]}{@const palette=themes[id]||{accent:$settings.accent,background:$settings.background,text:$settings.text,secondary:'#101010',tertiary:'#202020'}}<button class="theme-card" class:active={$settings.theme===id} onclick={()=>update('theme',id)}><h3>{title}</h3><p>{description}</p><div class="theme-preview" style={`background:${palette.background};color:${palette.text}`}><i style={`width:25%;background:${palette.accent}`}></i><i style={`width:80%;background:${palette.text};opacity:.14`}></i><i style={`background:${palette.secondary};border:1px solid ${palette.tertiary}`}></i></div></button>{/each}</div>{#if $settings.theme==='custom'}<div class="color-grid"><label><span>Акцент</span><input type="color" value={$settings.accent} oninput={(e)=>update('accent',e.currentTarget.value)}/><code>{$settings.accent}</code></label><label><span>Фон</span><input type="color" value={$settings.background} oninput={(e)=>update('background',e.currentTarget.value)}/><code>{$settings.background}</code></label><label><span>Текст</span><input type="color" value={$settings.text} oninput={(e)=>update('text',e.currentTarget.value)}/><code>{$settings.text}</code></label></div>{/if}</section>

      <section class="settings-section panel"><div class="settings-title">Мониторинг</div><div class="setting-row static"><div><strong>Discovery</strong><span>Автоматически перечитывает DoT/DoH и Policy из Keenetic.</span></div><code>60 сек</code></div><div class="setting-row static"><div><strong>Health-check</strong><span>Проверка каждого локального DNS upstream через реальный proxy.</span></div><code>30 сек</code></div><div class="setting-row static"><div><strong>История</strong><span>Minute buckets хранятся только в RAM.</span></div><code>24 часа</code></div><div class="setting-row static"><div><strong>Веб-порт</strong><span>Панель DNS Monitor.</span></div><code>2233</code></div></section>
    </div>
  </div>
</div>
