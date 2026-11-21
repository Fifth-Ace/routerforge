<script>
  import { settings } from '$lib/stores/settings.js';
  import { routerForgeServerPreview } from '$lib/routerforge-server-preview.js';

  $: locale = $settings.locale === 'en' ? 'en' : 'ru';
  const tr = (ru, en) => locale === 'en' ? en : ru;
</script>

<svelte:head><title>RouterForge Server — {tr('В разработке', 'In development')}</title></svelte:head>
<div class="page rf-server-hub-page">
  <div class="page-head">
    <div>
      <span class="page-kicker">ROUTERFORGE ECOSYSTEM / SERVER</span>
      <h1>RouterForge Server <span class="rf-server-stage">{tr('В разработке', 'In development')}</span></h1>
      <p>{tr('Будущий модуль связи RouterForge с независимым консольным менеджером Linux-серверов.', 'A planned RouterForge integration with the independent Linux server CLI manager.')}</p>
    </div>
  </div>
  <div class="rf-server-grid">
    <section class="panel rf-server-block">
      <h2>{tr('Новый серверный проект уже доступен', 'The standalone server project is available')}</h2>
      <p>{tr('RF Server помогает проводить диагностику Ubuntu/VPS, работать с отчётами, TLS-сканером, обновлениями, SSH и UFW. Действия, меняющие систему, выполняются только после подтверждения.', 'RF Server provides Ubuntu/VPS diagnostics, reports, TLS scanning, updates, SSH and UFW tooling. System-changing actions require confirmation.')}</p>
      <div class="rf-server-meta"><span>Linux / VPS</span><span>CLI v{routerForgeServerPreview.independentVersion}</span><span>{tr('Отдельная установка', 'Standalone installation')}</span></div>
      <a class="button" href={routerForgeServerPreview.repository} target="_blank" rel="noopener noreferrer">{tr('Открыть проект на GitHub', 'Open the project on GitHub')}</a>
    </section>
    <section class="panel rf-server-block">
      <h2>{tr('Скоро в RouterForge', 'Coming to RouterForge')}</h2>
      <ul>
        <li>{tr('Добровольная привязка серверов после подтверждения владения', 'Opt-in server pairing after ownership confirmation')}</li>
        <li>{tr('Состояние и мониторинг серверов с нескольких площадок', 'Status and monitoring across server locations')}</li>
        <li>{tr('Безопасные API-контракты и журнал действий', 'Guarded API contracts and action history')}</li>
        <li>{tr('В дальнейшем — интеграция с RouterForge Portal', 'Future RouterForge Portal integration')}</li>
      </ul>
      <p class="rf-server-warning">{tr('Пока это предварительный экран: подключение серверов, управление и установка модуля недоступны. Ничего не отправляется на VPS и не меняет роутер.', 'This is a preview only: server pairing, remote management and module installation are not available. No VPS or router settings are changed.')}</p>
    </section>
  </div>
  <a class="button rf-server-back" href="/apps?tab=routerforge">{tr('Вернуться в Центр приложений', 'Back to App Center')}</a>
</div>

<style>
  .rf-server-hub-page {min-width:0;}
  .rf-server-hub-page .page-head p {max-width:820px;color:var(--rf-muted,var(--muted));}
  .rf-server-stage {display:inline-block;margin-left:10px;font-size:12px;font-weight:650;line-height:1.4;padding:3px 7px;border:1px solid var(--rf-border,#334155);border-radius:6px;vertical-align:middle;color:var(--rf-accent,#38bdf8);}
  .rf-server-grid {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;}
  .rf-server-block {padding:20px;min-width:0;}
  .rf-server-block h2 {margin:0 0 12px;font-size:19px;}
  .rf-server-block p,.rf-server-block li {font-size:14px;line-height:1.6;}
  .rf-server-block ul {padding-left:20px;}
  .rf-server-meta {display:flex;gap:7px;flex-wrap:wrap;margin:15px 0;}
  .rf-server-meta span {padding:5px 8px;font-size:11px;border:1px solid var(--rf-border,#334155);border-radius:7px;}
  .rf-server-warning {color:var(--rf-muted,var(--muted));}
  .rf-server-back {margin-top:16px;display:inline-flex;}
  @media (max-width:860px) {.rf-server-grid {grid-template-columns:minmax(0,1fr);} .rf-server-block {padding:15px;}}
</style>
