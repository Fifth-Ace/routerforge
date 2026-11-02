'use strict';
/* RouterForge P22: UI-only visual parity with approved Advanced Mode mockup.
 * The P21 module remains the single owner of policy edits and apply operations.
 * This enhancer only reads the existing device API and forwards user input into
 * the already-tested P21 form controls. No Keenetic, Gate or runtime mutation.
 */
(() => {
  const $ = id => document.getElementById(id);
  const app = $('dnsLiteApp');
  if (!app || !$('p21Page') || !$('p21Mode')) return;
  const english = document.documentElement.lang === 'en' ||
    new URLSearchParams(location.search).get('lang') === 'en';
  const t = (ru,en) => english ? en : ru;
  const byMac = new Map();
  let fetching = false;
  let devices = [];
  const nativeSelect = $('p21Mode');
  const basicButton = $('p22ModeBasic');
  const advancedButton = $('p22ModeAdvanced');
  const cardDevices = $('p22DevicesCard');
  const cardGroups = $('p22GroupsCard');
  const cardRules = $('p22RulesCard');
  const cards = {devices:cardDevices,groups:cardGroups,rules:cardRules};
  const tabs = {
    devices:$('p21Nav'),
    groups:$('p22NavGroups'),
    rules:$('p22NavRules')
  };

  function option(value, label) {
    const o = document.createElement('option');
    o.value = value;
    o.textContent = label;
    return o;
  }
  function cleanMAC(raw) {
    return typeof raw === 'string' && /^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(raw)
      ? raw.toLowerCase() : '';
  }
  function setMode(mode) {
    const advanced = mode === 'advanced';
    basicButton.setAttribute('aria-pressed',String(!advanced));
    advancedButton.setAttribute('aria-pressed',String(advanced));
    basicButton.classList.toggle('selected',!advanced);
    advancedButton.classList.toggle('selected',advanced);
  }
  function chooseMode(mode) {
    nativeSelect.value = mode;
    nativeSelect.dispatchEvent(new Event('change',{bubbles:true}));
    setMode(mode);
    if (mode === 'advanced') refreshDevices();
  }
  basicButton.addEventListener('click',()=>chooseMode('basic'));
  advancedButton.addEventListener('click',()=>chooseMode('advanced'));
  nativeSelect.addEventListener('change',()=>{
    setMode(nativeSelect.value);
    // Advanced means the Advanced workspace, not the legacy DNS overview.
    if (nativeSelect.value === 'advanced') tabs.devices.click();
  });
  const modeObserver = new MutationObserver(()=>setMode(app.dataset.dnsMode));
  modeObserver.observe(app,{attributes:true,attributeFilter:['data-dns-mode']});
  setMode(app.dataset.dnsMode);

  // P23: each advanced tab is a separate view, never one long page.
  // P21 still owns page transitions, form state and all mutations.
  function chooseSection(which) {
    if (!Object.hasOwn(cards,which)) return;
    if (app.dataset.dnsMode !== 'advanced') chooseMode('advanced');
    if ($('p21Page').hidden) tabs.devices.click();
    for (const [key,tab] of Object.entries(tabs)) {
      if (!tab) continue;
      const selected = key === which;
      tab.classList.toggle('selected',selected);
      if (selected) tab.setAttribute('aria-current','page');
      else tab.removeAttribute('aria-current');
      cards[key].hidden = !selected;
    }
    // Route explanation is a child of the domain-rules view.
    // Never clear or re-create form inputs when changing tabs.
  }
  tabs.devices.addEventListener('click',()=>chooseSection('devices'));
  tabs.groups.addEventListener('click',()=>chooseSection('groups'));
  tabs.rules.addEventListener('click',()=>chooseSection('rules'));
  for (const id of ['navDns','navLogs']) {
    $(id).addEventListener('click',()=>{
      for (const tab of Object.values(tabs)) {
        tab?.classList.remove('selected');
        tab?.removeAttribute('aria-current');
      }
    });
  }
  // Keep the route diagnostic inside the domain-rules view only.
  const routeCard = $('p22RouteCard');
  if (routeCard && cardRules) {
    cardRules.appendChild(routeCard);
    routeCard.classList.add('p22-inline-route');
  }
  const deviceList = $('p21Devices');
  const groupList = $('p21Groups');
  const ruleList = $('p21Rules');
  const deviceFooter = $('p22DeviceFooter');
  // Compact section headings, consistent with the approved design render.
  tabs.devices.textContent = t('Устройства','Devices');
  tabs.groups.textContent = t('Группы','Groups');
  tabs.rules.textContent = t('Правила домена','Domain rules');
  cardDevices.querySelector('h2').textContent = t('Устройства','Devices');
  cardGroups.querySelector('h2').textContent = t('Группы устройств','Device groups');
  cardRules.querySelector('h2').textContent = t('Правила домена','Domain rules');
  const sectionDescription = (panel,text) => {
    const subtitle = document.createElement('p');
    subtitle.className = 'p22-section-subtitle';
    subtitle.textContent = text;
    panel.querySelector('h2').insertAdjacentElement('afterend',subtitle);
  };
  sectionDescription(cardGroups,t('Объединяй устройства и назначай им общий DNS.','Group devices under one DNS policy.'));
  const oldDescription = cardRules.querySelector('.p22-section-subtitle');
  if (oldDescription && english) oldDescription.textContent = 'Route specific domains through different DNS servers for devices.';
  const helpTitle = cardRules.querySelector('.p22-domain-help strong');
  const helpText = cardRules.querySelector('.p22-domain-help p');
  if (english && helpTitle) helpTitle.textContent = 'What is a domain rule?';
  if (english && helpText) helpText.textContent = 'A rule selects which DNS server handles a website. For example, youtube.com can use one DNS while everything else uses the default. Group defaults are configured under Groups; domain overrides here target devices.';
  const manualMACCaption = $('p21MAC')?.closest('.field')?.querySelector('span');
  if (english && manualMACCaption) manualMACCaption.textContent = 'Or enter MAC manually';
  const identificationHelp = cardDevices.querySelector('[data-p21-i18n="identityHelp"]');
  if (identificationHelp) {
    const d = document.createElement('details');
    d.className = 'p22-identity-details';
    const summary = document.createElement('summary');
    summary.textContent = t('Ограничения определения устройств','Device discovery limitations');
    identificationHelp.replaceWith(d);
    d.append(summary,identificationHelp);
  }
  const groupEditor = cardGroups.querySelector('.p22-group-fields');
  const groupChooser = $('p21GroupID')?.closest('.field');
  if (groupEditor && groupChooser) {
    groupChooser.classList.add('p22-group-picker');
    cardGroups.insertBefore(groupChooser,groupEditor);
  }
  if (routeCard) {
    const description = routeCard.querySelector('[data-p21-i18n="precedence"]');
    if (description) {
      const details = document.createElement('details');
      details.className = 'p22-precedence-details';
      const summary = document.createElement('summary');
      summary.textContent = t('Как выбирается DNS · приоритеты правил','DNS precedence and routing priorities');
      description.replaceWith(details);
      details.append(summary,description);
    }
    const applyCheckbox = $('p21ProbeApplied')?.closest('.check');
    const probeButton = $('p21Probe');
    if (applyCheckbox && probeButton) {
      const footer = document.createElement('div');
      footer.className='p22-route-footer';
      applyCheckbox.before(footer);
      footer.append(applyCheckbox,probeButton);
    }
  }
  if (deviceFooter && cardDevices) {
    // Save/apply/refresh belong to the whole P21 workspace, not to Devices.
    // They must stay accessible on Groups and Domain rules as well.
    const workspace = $('p21Page');
    const toolbar = document.createElement('div');
    toolbar.className = 'p22-draft-toolbar';
    toolbar.id = 'p23DraftToolbar';
    const title = document.createElement('span');
    title.className = 'p22-draft-toolbar-label';
    title.textContent = t('Настройки устройств · черновик', 'Device policies · draft');
    toolbar.appendChild(title);
    const actionButtons = cardDevices.querySelector('.section-title .actions');
    if (actionButtons) toolbar.appendChild(actionButtons);
    workspace.insertBefore(toolbar,cardDevices);
    const cacheState = $('p21Cache');
    if (cacheState) workspace.insertBefore(cacheState,cardDevices);
    const addButton = $('p21SetDevice');
    if (addButton) deviceFooter.insertBefore(addButton,deviceFooter.firstChild);
    if (deviceList) cardDevices.appendChild(deviceList);
  }
  if (groupList && cardGroups) cardGroups.appendChild(groupList);
  if (ruleList && cardRules) cardRules.appendChild(ruleList);

  function deviceTitle(row) {
    const name = (row.name || t('Не определён','Unidentified')).trim();
    const address = row.ip || '—';
    return `${name} · ${address} · ${row.mac}`;
  }
  function updateSelect(id, lead, previous) {
    const control = $(id);
    if (!control) return;
    const value = previous === undefined ? control.value : previous;
    control.replaceChildren(option('',lead));
    for (const d of devices) {
      control.appendChild(option(d.mac,deviceTitle(d) +
        (d.trusted ? '' : t(' · не подтверждено',' · unverified'))));
    }
    control.value = devices.some(d=>d.mac===value) ? value : '';
  }
  function syncDeviceLists() {
    updateSelect('p22DevicePick',t('Выбери устройство из списка Keenetic','Choose a device from Keenetic'));
    updateSelect('p22MemberSelect',t('Добавить устройство из списка','Add a device from the list'));
    updateSelect('p22RulePick',t('Выбери устройство','Choose a device'));
    updateSelect('p22ProbePick',t('Выбери устройство','Choose a device'));
    const status = $('p22DeviceDiscovery');
    if (status) status.textContent = devices.length
      ? t(`Обнаружено устройств: ${devices.length}. Недоступные и неизвестные можно добавить вручную.`,
          `Discovered devices: ${devices.length}. Other devices can be added by MAC.`)
      : t('Нет доступных устройств. Можно указать MAC вручную.','No devices discovered. Enter a MAC manually.');
  }
  async function refreshDevices() {
    if (fetching || app.dataset.dnsMode !== 'advanced') return;
    fetching = true;
    try {
      const response = await fetch('../service/devices',{
        method:'GET',cache:'no-store',credentials:'same-origin',
        headers:{Accept:'application/json'}
      });
      if (!response.ok) throw Error(`HTTP ${response.status}`);
      const result = await response.json();
      const rows = Array.isArray(result.devices) ? result.devices : [];
      const deduped = new Map();
      for (const row of rows) {
        const mac = cleanMAC(row?.mac);
        if (!mac) continue;
        const prior = deduped.get(mac);
        // A verified, fresh association outranks unverified inventory rows.
        if (!prior || (!prior.trusted && row.trusted)) {
          deduped.set(mac,{
            mac, ip:String(row.ip||''),name:String(row.name||''),
            trusted:row.trusted===true
          });
        }
      }
      devices = [...deduped.values()].sort((a,b)=>
        Number(b.trusted)-Number(a.trusted) || (a.name||a.mac).localeCompare(b.name||b.mac));
      byMac.clear();
      for (const d of devices) byMac.set(d.mac,d);
      syncDeviceLists();
    } catch {
      // The module remains usable with manual MAC when discovery is unavailable.
      const status = $('p22DeviceDiscovery');
      if (status) status.textContent = t('Обнаружение сейчас недоступно. Введи MAC вручную.',
        'Discovery is unavailable. Enter the MAC manually.');
    } finally {
      fetching = false;
    }
  }
  const devicePick = $('p22DevicePick');
  devicePick.addEventListener('change',()=>{
    const row = byMac.get(devicePick.value);
    if (!row) return;
    $('p21MAC').value = row.mac;
    if (row.name) $('p21Name').value = row.name;
    $('p21RuleMAC').value = row.mac;
    $('p21ProbeMAC').value = row.mac;
    $('p22RulePick').value = row.mac;
    $('p22ProbePick').value = row.mac;
    updateManualRuleMode();
  });
  $('p21Refresh').addEventListener('click',()=>{
    // Refresh after P21's click has started; both calls are READ-ONLY.
    refreshDevices();
  });

  // Group members are still owned by the P21 textarea. Chips are a visual
  // editor over that same value; Save Group retains P21's validation path.
  const members = $('p21Members');
  const chips = $('p22MemberChips');
  function memberMACs() {
    return [...new Set(members.value.split(/[\s,]+/).map(cleanMAC).filter(Boolean))];
  }
  function refreshChips() {
    chips.replaceChildren();
    for (const mac of memberMACs()) {
      const chip = document.createElement('button');
      chip.type = 'button';
      chip.className = 'p22-chip';
      chip.title = t('Убрать из группы','Remove from group');
      chip.textContent = (byMac.get(mac)?.name || mac) + ' ×';
      chip.addEventListener('click',()=>{
        members.value = memberMACs().filter(x=>x!==mac).join('\n');
        members.dispatchEvent(new Event('input',{bubbles:true}));
      });
      chips.appendChild(chip);
    }
  }
  members.addEventListener('input',refreshChips);
  $('p22MemberAdd').addEventListener('click',()=>{
    const chosen = $('p22MemberSelect').value;
    if (!chosen) return;
    members.value = [...new Set([...memberMACs(),chosen])].join('\n');
    $('p22MemberSelect').value = '';
    members.dispatchEvent(new Event('input',{bubbles:true}));
  });
  $('p21GroupID').addEventListener('change',()=>queueMicrotask(refreshChips));
  $('p21SetGroup').addEventListener('click',()=>queueMicrotask(refreshChips));
  refreshChips();

  const rulePick = $('p22RulePick');
  function updateManualRuleMode() {
    const manual = $('p22RuleManual');
    if (!manual) return;
    manual.hidden = Boolean(rulePick.value);
  }
  rulePick.addEventListener('change',()=>{
    if (rulePick.value) $('p21RuleMAC').value = rulePick.value;
    updateManualRuleMode();
  });
  $('p21RuleMAC').addEventListener('input',()=>{
    if (rulePick.value && cleanMAC($('p21RuleMAC').value)!==rulePick.value) {
      rulePick.value = '';
      updateManualRuleMode();
    }
  });
  const probePick = $('p22ProbePick');
  probePick.addEventListener('change',()=>{
    if (probePick.value) $('p21ProbeMAC').value = probePick.value;
  });
  $('p21ProbeMAC').addEventListener('input',()=>{
    if (probePick.value && cleanMAC($('p21ProbeMAC').value)!==probePick.value) probePick.value='';
  });
  updateManualRuleMode();

  // The API is configuration-only. Display a human-readable explanation, not
  // an invented proof of hardware routing or upstream reachability.
  const rawResult = $('p21Explanation');
  const route = $('p22RouteSummary');
  function chip(label,extraClass='') {
    const el = document.createElement('span');
    el.className = `p22-route-chip ${extraClass}`.trim();
    el.textContent = label;
    return el;
  }
  function explainRoute() {
    route.replaceChildren();
    let payload;
    try { payload = JSON.parse(rawResult.textContent); } catch { return; }
    if (!payload || payload.scope !== 'CONFIGURATION_ONLY') return;
    const d = payload.decision || {};
    const chain = Array.isArray(payload.allowed_chain) ? payload.allowed_chain : [];
    const mac = cleanMAC($('p21ProbeMAC').value);
    const known = byMac.get(mac);
    const identity = mac ? (known?.name || mac) : t('Общие правила','Common rules');
    const layer = ({device_domain:t('Правило устройства','Device rule'),device:t('DNS устройства','Device DNS'),
      device_group:t('Группа устройств','Device group'),common_domain:t('Общее доменное правило','Common domain rule'),
      default:t('DNS по умолчанию','Default DNS'),local_namespace:t('Локальный DNS Keenetic','Keenetic local DNS')})[d.layer] || d.layer || t('Правило не определено','No rule matched');
    const target = chain[0];
    const upstream = target ? (target.server_name || target.address || target.id || '—')
      : (d.layer === 'local_namespace' ? t('Локальный DNS Keenetic','Keenetic local DNS') : '—');
    const reserve = chain.length > 1 ? chain.slice(1).map(u=>u.server_name||u.address||u.id).join(' → ') : '';
    const steps = [
      chip(t('Устройство: ','Device: ')+identity),
      chip(t('Правило: ','Rule: ')+layer),
      chip(t('DNS: ','DNS: ')+upstream),
      ...(reserve?[chip(t('Резерв: ','Fallback: ')+reserve)]:[])
    ];
    steps.forEach((step,index)=>{
      if(index){const arrow=document.createElement('span');arrow.className='p22-route-arrow';arrow.textContent='→';route.appendChild(arrow);}
      route.appendChild(step);
    });
    const note = document.createElement('small');
    note.className='p22-route-disclaimer';
    note.textContent=t('Расчёт по конфигурации, не подтверждённый сетевой E2E.',
      'Configuration evaluation, not a verified network E2E test.');
    route.appendChild(note);
  }
  if (rawResult && route) {
    new MutationObserver(explainRoute).observe(rawResult,{childList:true,characterData:true,subtree:true});
    explainRoute();
  }

  // P21 controls remain authoritative. Section links navigate only; neither
  // discovery nor visual mode selection modifies DNS policy or router state.
  const modeSwitch = document.querySelector('.p22-mode-switch');
  if (modeSwitch) modeSwitch.dataset.label = t('Режим','Mode');
  app.classList.add('p22-ready');
  if (app.dataset.dnsMode === 'advanced') tabs.devices.click();
  refreshDevices();
})();
