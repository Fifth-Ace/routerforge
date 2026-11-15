<script>
  import { onMount } from 'svelte';
  import {
    getRouterforgeSyslogStatus, getRouterforgeSyslogFiles, getRouterforgeSyslogTail,
    applyRouterforgeSyslog, routerforgeSyslogExportURL, entwarePackageAction,
    preflightEntwarePackage, adminServiceAction
  } from '$lib/api.js';
  export let locale = 'ru';
  const tr = (ru,en) => locale === 'en' ? en : ru;
  let status = null;
  let draft = null;
  let files = [];
  let chosen = null;
  let lines = [];
  let filter = '';
  let busy = '';
  let error = '';
  let notice = '';
  let acceptAdoption = false;
  let dirty = false;
  let periodic = null;
  let loadEpoch = 0;
  let tailEpoch = 0;

  $: visibleLines = lines.filter((line) => !filter || line.toLowerCase().includes(filter.toLowerCase()));

  async function refresh({ initial = false } = {}) {
    const epoch = ++loadEpoch;
    try {
      const current = await getRouterforgeSyslogStatus();
      if (epoch !== loadEpoch) return;
      status = current;
      if (initial || !dirty) draft = structuredClone(current.settings);
      if (current.installed && current.managed) {
        const list = await getRouterforgeSyslogFiles();
        if (epoch !== loadEpoch) return;
        files = list.files || [];
        if (!chosen || !files.some((f) => f.date === chosen.date && f.source === chosen.source)) {
          chosen = files[0] || null;
          await reloadTail();
        }
      } else { files=[];chosen=null;lines=[]; }
      error='';
    } catch (e) { if (epoch === loadEpoch) error = getError(e); }
  }
  function getError(e) { return e?.payload?.detail || e?.payload?.error || e?.message || 'Request failed'; }
  async function reloadTail() {
    const epoch=++tailEpoch;
    if (!chosen) {lines=[];return;}
    try {const result=await getRouterforgeSyslogTail(chosen.date,chosen.source,300);if(epoch===tailEpoch) lines=result.lines||[];}
    catch(e){if(epoch===tailEpoch) error=getError(e);}
  }
  function selectFile(file) { chosen=file; void reloadTail(); }
  function mark() {dirty=true;}
  async function install() {
    if (busy) return;
    busy='install'; error='';notice='';
    try {
      const p=await preflightEntwarePackage('syslog-ng','install');
      if (!p.allowed) throw new Error(p.reason || tr('App Center отклонил установку','Installation not permitted by App Center'));
      if (!confirm(tr('Установить пакет syslog-ng из Entware?','Install syslog-ng from Entware?'))) return;
      await entwarePackageAction('syslog-ng','install');
      notice=tr('Пакет установлен. Конфигурация не изменялась.','Package installed. Configuration unchanged.');
      dirty=false;
      await refresh({initial:true});
    } catch(e){error=getError(e);} finally{busy='';}
  }
  async function apply() {
    if (!draft || busy || !status) return;
    if (status.requires_adoption && !acceptAdoption) {error=tr('Сначала разреши принятие существующей конфигурации.','Explicit adoption is required.');return;}
    if (draft.local && !acceptAdoption) {error=tr('Для /dev/log требуется отдельное подтверждение.','Explicit /dev/log opt-in required.');return;}
    if (!confirm(tr('Применить конфигурацию syslog-ng? Существующая будет сохранена в резервную копию. Служба НЕ перезапускается.','Apply syslog-ng config? Existing config will be backed up. Service will NOT restart.'))) return;
    busy='apply';error='';notice='';
    try {
      const result=await applyRouterforgeSyslog({settings:draft,expected_sha256:status.config_sha256,adopt_existing:acceptAdoption,confirm:'APPLY SYSLOG'});
      notice=tr('Настройки сохранены. Для активации вручную перезапусти службу. Резервная копия: ','Saved. Restart service manually to activate. Backup: ')+(result.backup||'—');
      dirty=false;acceptAdoption=false;
      await refresh({initial:true});
    } catch(e){error=getError(e);} finally{busy='';}
  }
  async function serviceAction(action) {
    if (busy || !status?.service_id) return;
    if (!confirm(tr('Выполнить','Execute')+' '+action+' '+status.service_id+'?')) return;
    busy='service';error='';notice='';
    try {await adminServiceAction(status.service_id,action);notice=tr('Действие выполнено','Action complete')+': '+action;await refresh();}
    catch(e){error=getError(e);}finally{busy='';}
  }
  onMount(() => {
    refresh({initial:true});
    periodic=window.setInterval(() => {if (!document.hidden && !busy) {void refresh();if(chosen)void reloadTail();}},10000);
    return () => {loadEpoch++;tailEpoch++;if(periodic)clearInterval(periodic);};
  });
</script>

<div class="rf-syslog">
  <section class="panel">
    <div class="panel-head">
      <div><strong>Syslog Manager</strong><span>{tr('Управление syslog-ng из Entware · без собственной службы RouterForge','Entware syslog-ng integration · no additional RouterForge service')}</span></div>
      <button class="button" onclick={() => refresh()} disabled={!!busy}>↻ {tr('Обновить','Refresh')}</button>
    </div>
    {#if error}<div class="rf-syslog-msg rf-error" role="alert">{error}</div>{/if}
    {#if notice}<div class="rf-syslog-msg rf-ok" role="status">{notice}</div>{/if}
    {#if !status}
      <div class="rf-syslog-empty">{tr('Получаем состояние...','Loading status...')}</div>
    {:else}
      <div class="rf-syslog-status">
        <div><small>{tr('Пакет','Package')}</small><strong>{status.installed ? 'syslog-ng '+(status.version||'') : tr('Не установлен','Not installed')}</strong></div>
        <div><small>{tr('Служба','Service')}</small><strong>{status.running ? tr('Работает','Running') : tr('Остановлена','Stopped')}</strong></div>
        <div><small>{tr('Конфигурация','Config')}</small><strong>{status.managed ? 'RouterForge' : status.config_exists ? tr('Сторонняя','Existing / unmanaged') : tr('Нет','Missing')}</strong></div>
        <div><small>{tr('Хранилище','Storage')}</small><strong>{status.storage_ready ? 'READY' : 'UNAVAILABLE'}</strong></div>
      </div>
    {/if}
  </section>

  {#if status && !status.installed}
    <section class="panel rf-syslog-space">
      <strong>{tr('Требуется пакет Entware syslog-ng','Entware syslog-ng required')}</strong>
      <p>{tr('RouterForge не устанавливает пакет автоматически. Для работы нужен системный сборщик syslog-ng.','RouterForge does not automatically install packages. syslog-ng is required.')}</p>
      <button class="button primary" onclick={install} disabled={!!busy}>{busy==='install'?'…':tr('Установить syslog-ng','Install syslog-ng')}</button>
      <span class="rf-hint">{tr('Операция выполняется через App Center и его действующую политику безопасности.','Uses App Center with its existing safety policy.')}</span>
      <a href="/apps">{tr('Открыть App Center','Open App Center')}</a>
    </section>
  {:else if status && draft}
    <section class="panel rf-syslog-space">
      <div class="rf-syslog-heading"><div><strong>{tr('Управление службой','Service controls')}</strong><span>{status.service_id||tr('Скрипт службы не обнаружен','Service script not found')}</span></div>
        <div class="rf-actions"><button class="button" disabled={!!busy||!status.service_id} onclick={() => serviceAction('start')}>START</button><button class="button" disabled={!!busy||!status.service_id} onclick={() => serviceAction('restart')}>RESTART</button><button class="button" disabled={!!busy||!status.service_id} onclick={() => serviceAction('stop')}>STOP</button></div>
      </div>
      <p class="rf-hint">{tr('Все действия выполняются только по нажатию. Обновление RouterForge не включает сбор автоматически.','All actions require explicit confirmation. RouterForge upgrades never enable collection.')}</p>
    </section>

    <section class="panel rf-syslog-space">
      <div class="rf-syslog-heading"><div><strong>{tr('Настройка сбора','Collector configuration')}</strong><span>{status.config_path}</span></div><span class="rf-chip">{status.managed?'MANAGED':'UNMANAGED'}</span></div>
      {#if status.requires_adoption}
        <p class="rf-syslog-msg rf-warning">{tr('Обнаружен существующий чужой syslog-ng.conf. Он не изменится, пока ты не разрешишь его принятие. Перед заменой делается резервная копия.','Existing third-party syslog-ng.conf detected. It will not be modified without explicit adoption and a backup.')}</p>
      {/if}
      <div class="rf-fields">
        <label>{tr('Адрес прослушивания','Listen address')}<select bind:value={draft.bind} onchange={mark}><option value="0.0.0.0">0.0.0.0 (LAN)</option><option value="127.0.0.1">127.0.0.1 (local)</option></select></label>
        <label>{tr('Порт','Port')}<input type="number" min="1024" max="65535" bind:value={draft.port} oninput={mark} /></label>
        <label>{tr('Хранение, дней','Retention, days')}<input type="number" min="1" max="365" bind:value={draft.keep_days} oninput={mark} /></label>
        <label>{tr('Общий лимит, МБ','Total limit, MB')}<input type="number" min="10" max="10240" bind:value={draft.max_mb} oninput={mark} /></label>
      </div>
      <div class="rf-checks">
        <label><input type="checkbox" bind:checked={draft.udp} onchange={mark}/> UDP</label>
        <label><input type="checkbox" bind:checked={draft.tcp} onchange={mark}/> TCP</label>
        <label><input type="checkbox" bind:checked={draft.local} onchange={mark}/> {tr('Локальные события /dev/log (риск конфликта)','Local /dev/log (conflict risk)')}</label>
      </div>
      <label class="rf-block">{tr('Разрешённые сети (IPv4 CIDR, через запятую)','Allowed networks (IPv4 CIDRs, comma separated)')}
        <input value={draft.allowed.join(', ')} oninput={(e)=>{draft.allowed=e.currentTarget.value.split(',').map(x=>x.trim()).filter(Boolean);mark();}} placeholder="192.168.0.0/16, 10.0.0.0/8" />
      </label>
      <label class="rf-block">{tr('Каталог журналов','Log directory')}<input bind:value={draft.root} oninput={mark} placeholder="/opt/var/log/routerforge-syslog" /></label>
      <p class="rf-hint">{tr('Доступны отдельный каталог Entware и /opt/mnt/УСТРОЙСТВО/routerforge-syslog. Хранилище должно быть примонтировано; запись во внутреннюю память при исчезновении USB запрещена.','Use the dedicated Entware directory or /opt/mnt/DEVICE/routerforge-syslog. Storage must be mounted; no fallback writes to internal flash.')}</p>
      {#if !status.storage_ready}<p class="rf-syslog-msg rf-warning">{status.storage_error || tr('Накопитель недоступен','Storage unavailable')}</p>{/if}
      {#if status.requires_adoption || draft.local}
        <label class="rf-danger"><input type="checkbox" bind:checked={acceptAdoption}/> {tr('Я разрешаю принять/заменить конфигурацию и понимаю возможный конфликт /dev/log','I explicitly authorize adoption / replacement and understand /dev/log conflicts')}</label>
      {/if}
      <div class="rf-actions"><button class="button primary" onclick={apply} disabled={!!busy||!status.binary_ready||!dirty||(status.requires_adoption&&!acceptAdoption)||(draft.local&&!acceptAdoption)}>{busy==='apply'?'…':tr('Проверить и сохранить','Validate & save')}</button></div>
      <p class="rf-hint">{tr('Синтаксис проверяется бинарником syslog-ng. Конфиг публикуется атомарно, служба не перезапускается. Для применения нажми RESTART отдельно.','The syslog-ng binary validates syntax. Atomic publish; service restart is a separate action.')}</p>
    </section>

    {#if status.managed}
      <section class="panel rf-syslog-space">
        <div class="rf-syslog-heading"><div><strong>{tr('Журналы','Journals')}</strong><span>{files.length} {tr('файлов (последние 250)','files (latest 250)')}</span></div><button class="button" onclick={() => refresh()} disabled={!!busy}>↻</button></div>
        <div class="rf-log-layout">
          <div class="rf-log-files">
            {#each files as file (`${file.date}/${file.filename}`)}
              <button class:selected={chosen?.date===file.date&&chosen?.filename===file.filename} onclick={() => selectFile(file)}>
                <strong>{file.source}</strong><small>{file.date} · {(file.size/1024).toFixed(1)} KB</small>
              </button>
            {/each}
            {#if !files.length}<div class="rf-syslog-empty">{tr('Журналы пока пусты','No journals yet')}</div>{/if}
          </div>
          <div class="rf-log-reader">
            <div class="rf-actions rf-log-toolbar"><input type="search" bind:value={filter} placeholder={tr('Поиск по строкам','Search lines')} /><button class="button" onclick={reloadTail} disabled={!chosen}>↻</button>
              {#if chosen}<a class="button" href={routerforgeSyslogExportURL(chosen.date,chosen.source)}>{tr('Скачать','Export')}</a>{/if}
            </div>
            <div class="rf-log-lines">{#each visibleLines as line,i (i)}<div>{line}</div>{/each}{#if !visibleLines.length}<span>{tr('Нет записей','No entries')}</span>{/if}</div>
          </div>
        </div>
        <p class="rf-hint">{tr('Очистка управляемых журналов выполняется ежечасно по сроку и лимиту. Текущие и чужие файлы не удаляются.','Managed journals are cleaned hourly by retention and size limits. Current-day and foreign files are never removed.')}</p>
      </section>
    {/if}
  {/if}
</div>

<style>
.rf-syslog{display:grid;gap:12px;min-width:0}.rf-syslog-space{padding:14px;display:grid;gap:12px}.rf-syslog-space p{margin:0}.rf-syslog-status{padding:12px;display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:9px}.rf-syslog-status>div{display:grid;gap:4px;border:1px solid var(--rf-border,#333);border-radius:7px;padding:10px;min-width:0}.rf-syslog-status small,.rf-hint,.rf-syslog-heading span{color:var(--rf-muted,#a2adb8);font-size:11px}.rf-syslog-status strong{font-size:13px;overflow-wrap:anywhere}.rf-syslog-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.rf-syslog-heading>div{display:grid;gap:4px}.rf-fields{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.rf-fields label,.rf-block{min-width:0;display:grid;gap:5px;font-size:12px;color:var(--rf-muted,#aaa)}.rf-fields input,.rf-fields select,.rf-block input,.rf-log-toolbar input{min-width:0;width:100%;box-sizing:border-box;border:1px solid var(--rf-border,#333);background:var(--rf-surface-2,#15191e);border-radius:6px;color:var(--rf-text,#fff);padding:8px}.rf-checks,.rf-actions{display:flex;gap:9px;flex-wrap:wrap;align-items:center}.rf-checks label,.rf-danger{display:flex;gap:6px;align-items:center;font-size:12px}.rf-danger{color:var(--warn,#e9b459)}.rf-syslog-msg{padding:10px;border:1px solid var(--rf-border,#333);border-radius:6px;overflow-wrap:anywhere;font-size:12px}.rf-error{color:var(--error,#ff6565)}.rf-ok{color:var(--good,#7ad49e)}.rf-warning{color:var(--warn,#e9b459)}.rf-syslog-empty{color:var(--rf-muted,#aaa);padding:16px}.rf-log-layout{display:grid;grid-template-columns:minmax(180px,260px) minmax(0,1fr);min-height:230px;border:1px solid var(--rf-border,#333);border-radius:7px;overflow:hidden}.rf-log-files{overflow:auto;max-height:360px;border-right:1px solid var(--rf-border,#333)}.rf-log-files button{display:grid;width:100%;text-align:left;gap:4px;border:0;border-bottom:1px solid var(--rf-border,#333);background:transparent;color:inherit;padding:10px;cursor:pointer}.rf-log-files button.selected{background:var(--rf-accent-soft,rgba(56,189,248,.15))}.rf-log-files small{color:var(--rf-muted,#aaa)}.rf-log-reader{display:grid;grid-template-rows:auto 1fr;min-width:0}.rf-log-toolbar{padding:8px;border-bottom:1px solid var(--rf-border,#333)}.rf-log-toolbar input{flex:1;width:auto}.rf-log-lines{padding:10px;font:11px/1.55 var(--font-mono,monospace);overflow:auto;max-height:315px;white-space:pre-wrap;word-break:break-word}.rf-chip{font-size:10px}.rf-actions a.button{text-decoration:none}@media(max-width:900px){.rf-fields{grid-template-columns:repeat(2,minmax(0,1fr))}.rf-syslog-status{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:600px){.rf-log-layout{grid-template-columns:1fr}.rf-log-files{max-height:170px;border-right:0;border-bottom:1px solid var(--rf-border,#333)}.rf-fields{grid-template-columns:1fr}}
</style>
