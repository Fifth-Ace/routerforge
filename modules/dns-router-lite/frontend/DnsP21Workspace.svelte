<script>
  import DnsP21Rows from './DnsP21Rows.svelte';
  import DnsP22Visual from './DnsP22Visual.svelte';
  import {normalizeDiscoveredDevices} from './DnsP22Discovery.js';

  // One Svelte owner for P21/P22 workspace, editors, section visibility,
  // discovery presentation and route-result presentation.
  // The parent owns the only API client, CAS revision and draft mutations.
  let {model={},section='devices',busy=false,selection=null,route=null,probeMac='',onAction=()=>{},onRowAction=()=>{}}=$props();
  const english=document.documentElement.lang==='en'||new URLSearchParams(location.search).get('lang')==='en';
  const t=(ru,en)=>english?en:ru;
  function portal(node){
    const target=document.getElementById('rf-dns-lite-svelte-p21-workspace');
    if(!target)throw Error('P21_WORKSPACE_SLOT_MISSING');
    target.replaceChildren(node);
    return {destroy(){if(node.parentNode===target)node.remove();}};
  }
  const validMac=raw=>typeof raw==='string'&&/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(raw)?raw.toLowerCase():'';
  let discovered=$derived(normalizeDiscoveredDevices(model.inventory||[]));
  let targets=$derived(model.targets||[]);
  let groups=$derived(model.groups||[]);

  let deviceMAC,deviceName,deviceTarget,deviceInherit,devicePick;
  let groupID,groupName,groupMembers,groupTarget,groupInherit,memberSelect;
  let ruleMAC,ruleDomain,ruleMatch,ruleTarget,rulePriority,rulePick;
  let routeMAC,routeDomain,routeApplied,routePick;
  let membersValue=$state('');
  let chips=$derived([...new Set(membersValue.split(/[\s,]+/).map(validMac).filter(Boolean))]);
  let ruleIsPicked=$state(false);
  let groupChoice=$state('');
  let memberName=$derived(Object.fromEntries(discovered.map(d=>[d.mac,d.name||d.mac])));

  $effect(()=>{
    const item=selection;
    if(!item?.generation||!deviceMAC)return;
    if(item.kind==='device'){
      deviceMAC.value=item.mac||'';
      deviceName.value=item.name||'';
      deviceTarget.value=item.target||'';
      deviceInherit.checked=!!item.inherit;
      ruleMAC.value=item.mac||'';
      routeMAC.value=item.mac||'';
    } else if(item.kind==='rule'){
      ruleMAC.value=item.mac||'';
      ruleDomain.value=item.domain||'';
      ruleMatch.value=item.match||'suffix';
      ruleTarget.value=item.target||'';
      rulePriority.value=String(item.priority??100);
      if(rulePick){rulePick.value='';ruleIsPicked=false;}
    }
  });

  function submitDevice(){onAction('device',{mac:deviceMAC.value,name:deviceName.value,target:deviceTarget.value,inherit_domains:deviceInherit.checked});}
  function submitGroup(){onAction('group',{id:groupID.value,name:groupName.value,members:groupMembers.value,target:groupTarget.value,inherit_domains:groupInherit.checked});}
  function submitRule(){onAction('rule',{mac:ruleMAC.value,domain:ruleDomain.value,match:ruleMatch.value,target:ruleTarget.value,priority:rulePriority.value});}
  function selectDevice(){
    const row=discovered.find(x=>x.mac===devicePick.value);
    if(!row)return;
    deviceMAC.value=row.mac;
    if(row.name)deviceName.value=row.name;
    ruleMAC.value=row.mac;routeMAC.value=row.mac;
    rulePick.value=row.mac;routePick.value=row.mac;
  }
  function selectGroup(){
    groupChoice=groupID.value;
    const group=groups.find(g=>g.id===groupChoice);
    groupName.value=group?.name||'';
    membersValue=(group?.macs||[]).join('\n');
    groupMembers.value=membersValue;
    groupTarget.value=group?.targetValue||'';
    groupInherit.checked=!!group?.inherit_domains;
  }
  function addMember(){
    if(!memberSelect.value)return;
    membersValue=[...new Set([...chips,memberSelect.value])].join('\n');
    groupMembers.value=membersValue;memberSelect.value='';
  }
  function removeMember(mac){membersValue=chips.filter(x=>x!==mac).join('\n');groupMembers.value=membersValue;}
  function selectRuleDevice(){if(rulePick.value)ruleMAC.value=rulePick.value;ruleIsPicked=!!rulePick.value;}
  function selectRouteDevice(){if(routePick.value)routeMAC.value=routePick.value;}
</script>
<div use:portal style="display:contents">
  <div id="p23DraftToolbar" class="p22-draft-toolbar">
    <span class="p22-draft-toolbar-label">{section==='groups'?t('Группы устройств · черновик','Device groups · draft'):section==='rules'?t('Правила домена · черновик','Domain rules · draft'):section==='settings'?t('Настройки DNS · черновик','DNS settings · draft'):t('Настройки устройств · черновик','Device policies · draft')}</span>
    <div class="actions">
      <button class="button" id="p21Refresh" disabled={busy} onclick={()=>onAction('refresh')}>{t('Обновить обнаружение','Refresh devices')}</button>
      <button class="button rfds-button" id="p21Save" disabled={busy} onclick={()=>onAction('save')}>{t('Сохранить черновик','Save draft')}</button>
      <button class="button primary rfds-button rfds-button--primary" id="p21Apply" disabled={busy} onclick={()=>onAction('apply')}>{t('Сохранить и применить','Save and apply')}</button>
    </div>
  </div>
  <p class="notice" id="p21Cache" role="status">{model.cache||''}</p>
  <section class="panel rfds-card" id="p22DevicesCard" hidden={section!=='devices'}>
    <div class="section-title"><div><h2>{t('Устройства','Devices')}</h2><p>{t('Изменения здесь — черновик. Сначала сохранить, затем явно применить DNS.','Changes remain a draft until explicit Save and Apply.')}</p></div></div>
    <details class="p22-identity-details"><summary>{t('Ограничения определения устройств','Device discovery limitations')}</summary><p class="muted" data-p21-i18n="identityHelp">MAC — не аутентификация. Нужен свежий прямой L2-сосед. DHCP даёт имя и проверку противоречий. Клиенты за NAT, VPN или другим роутером не различаются; неизвестный клиент использует общие правила.</p></details>
    <p id="p21DiscoveryStatus" class="muted">{model.discoveryStatus||''}</p>
    <p id="p22DeviceDiscovery" class="p22-discovery-hint" role="status" aria-live="polite">{discovered.length?t('Обнаружено устройств: '+discovered.length+'. Недоступные и неизвестные можно добавить вручную.','Discovered devices: '+discovered.length+'. Other devices can be added by MAC.'):t('Нет доступных устройств. Можно указать MAC вручную.','No devices discovered. Enter a MAC manually.')}</p>
    <div id="p21Devices" class="p21-device-list"><DnsP21Rows kind="devices" rows={model.rows?.devices||[]} {busy} onAction={onRowAction}/></div>
    <div class="fields two p21-form p22-device-fields">
      <label class="field p22-picker"><span>{t('Устройство на роутере','Device on router')}</span><select id="p22DevicePick" bind:this={devicePick} aria-label="Устройство на роутере" onchange={selectDevice}><option value="">{t('Выбери устройство из списка Keenetic','Choose a device from Keenetic')}</option>{#each discovered as item (item.mac)}<option value={item.mac}>{item.name||t('Не определён','Unidentified')} · {item.ip||'—'} · {item.mac}</option>{/each}</select></label>
      <label class="field"><span>{t('Или укажи MAC вручную','Or enter MAC manually')}</span><input id="p21MAC" bind:this={deviceMAC} maxlength="17" placeholder="02:11:22:33:44:55" spellcheck="false"/></label>
      <label class="field"><span>{t('Название','Name')}</span><input id="p21Name" bind:this={deviceName} maxlength="80"/></label>
      <label class="field"><span>{t('DNS или DNS-группа','DNS or DNS group')}</span><select id="p21Target" bind:this={deviceTarget}><option value="">{t('Общие правила','Common rules')}</option>{#each targets as item (item.value)}<option value={item.value}>{item.label}</option>{/each}</select></label>
      <label class="check"><input type="checkbox" id="p21Inherit" bind:this={deviceInherit}/><span>{t('Наследовать общие доменные правила перед DNS устройства','Inherit common domain rules before device DNS')}</span></label>
    </div>
    <div id="p22DeviceFooter" class="p22-device-footer"><button class="button rfds-button" id="p21SetDevice" disabled={busy} onclick={submitDevice}>{t('Добавить / обновить устройство в черновике','Add / update draft device')}</button></div>
  </section>
  <section class="panel rfds-card" id="p22GroupsCard" hidden={section!=='groups'}>
    <h2>{t('Группы устройств','Device groups')}</h2><p class="p22-section-subtitle">{t('Объединяй устройства и назначай им общий DNS.','Group devices under one DNS policy.')}</p>
    <div id="p21Groups" class="p21-device-list"><DnsP21Rows kind="groups" rows={model.rows?.groups||[]} {busy} onAction={onRowAction}/></div>
    <label class="field p22-group-picker"><span>{t('Редактируемая группа','Group to edit')}</span><select id="p21GroupID" bind:this={groupID} onchange={selectGroup}><option value="">{t('Новая группа','New group')}</option>{#each groups as item (item.id)}<option value={item.id}>{item.name}</option>{/each}</select></label>
    <div class="fields two p21-form p22-group-fields">
      <label class="field"><span>{t('Название','Name')}</span><input id="p21GroupName" bind:this={groupName} maxlength="80"/></label>
      <div class="p22-member-field"><label class="field"><span>{t('MAC участников — по одному в строке','Member MACs — one per line')}</span><textarea id="p21Members" bind:this={groupMembers} rows="3" spellcheck="false" oninput={()=>{membersValue=groupMembers.value;}}></textarea></label>
        <div class="p22-member-controls"><select id="p22MemberSelect" bind:this={memberSelect} aria-label="Добавить устройство в группу"><option value="">{t('Добавить устройство из списка','Add listed device')}</option>{#each discovered as item (item.mac)}<option value={item.mac}>{item.name||item.mac} · {item.mac}</option>{/each}</select><button type="button" class="button" id="p22MemberAdd" onclick={addMember}>+ {t('Добавить','Add')}</button></div>
        <div class="p22-member-chips" id="p22MemberChips" aria-label="Участники группы">{#each chips as mac (mac)}<button type="button" class="p22-chip" title={t('Убрать из группы','Remove from group')} onclick={()=>removeMember(mac)}>{memberName[mac]||mac} ×</button>{/each}</div>
      </div>
      <label class="field"><span>{t('DNS или DNS-группа','DNS or DNS group')}</span><select id="p21GroupTarget" bind:this={groupTarget}><option value="">{t('Общие правила','Common rules')}</option>{#each targets as item (item.value)}<option value={item.value}>{item.label}</option>{/each}</select></label>
    </div>
    <label class="check"><input type="checkbox" id="p21GroupInherit" bind:this={groupInherit}/><span>{t('Наследовать общие доменные правила перед DNS группы','Inherit common domain rules before group DNS')}</span></label>
    <button class="button rfds-button" id="p21SetGroup" disabled={busy} onclick={submitGroup}>{t('Сохранить группу в черновике','Save draft group')}</button>
  </section>
  <section class="panel rfds-card" id="p22RulesCard" hidden={section!=='rules'}>
    <h2>{t('Правила домена для устройства','Device domain rules')}</h2><p class="p22-section-subtitle">{t('Определи, какой DNS использовать для нужного сайта на конкретном устройстве.','Select DNS for specific domains and devices.')}</p>
    <div class="p22-domain-help"><strong>{t('Что такое правила домена?','What is a domain rule?')}</strong><p>{t('Это правила, через какой DNS открывать определённые сайты. Например, youtube.com можно направить через один DNS, а остальные сайты — через общий.','A rule selects which DNS server handles a website. For example, youtube.com can use one DNS while everything else uses the default. Group defaults are configured under Groups; domain overrides here target devices.')}</p></div>
    <div id="p21Rules" class="p21-device-list"><DnsP21Rows kind="rules" rows={model.rows?.rules||[]} {busy} onAction={onRowAction}/></div>
    <div class="fields two p21-form p22-rule-fields">
      <div class="p22-rule-scope"><label class="field"><span>{t('Область применения · устройство','Target device')}</span><select id="p22RulePick" bind:this={rulePick} onchange={selectRuleDevice}><option value="">{t('MAC вручную','Manual MAC')}</option>{#each discovered as item (item.mac)}<option value={item.mac}>{item.name||item.mac} · {item.mac}</option>{/each}</select></label><label class="field p22-rule-mac-manual" id="p22RuleManual" hidden={ruleIsPicked}><span>{t('MAC устройства','Device MAC')}</span><input id="p21RuleMAC" bind:this={ruleMAC} maxlength="17" spellcheck="false" oninput={()=>{if(rulePick&&rulePick.value!==validMac(ruleMAC.value)){rulePick.value='';ruleIsPicked=false;}}}/></label></div>
      <label class="field"><span>{t('Домен','Domain')}</span><input id="p21Domain" bind:this={ruleDomain} maxlength="253" placeholder="example.com" spellcheck="false"/></label>
      <label class="field"><span>{t('Совпадение','Match')}</span><select id="p21Match" bind:this={ruleMatch}><option value="suffix">{t('Домен + поддомены / suffix','Domain + subdomains / suffix')}</option><option value="exact">{t('Только домен / exact','Exact domain')}</option><option value="subdomains">{t('Только поддомены / subdomains','Subdomains only')}</option></select></label>
      <label class="field"><span>{t('DNS или DNS-группа','DNS or DNS group')}</span><select id="p21RuleTarget" bind:this={ruleTarget}><option value="">{t('Общие правила','Common rules')}</option>{#each targets as item (item.value)}<option value={item.value}>{item.label}</option>{/each}</select></label>
      <label class="field"><span>{t('Приоритет при равной точности: меньше — раньше','Priority: lower first')}</span><input id="p21Priority" bind:this={rulePriority} type="number" min="0" max="1000000" value="100"/></label>
    </div><button class="button rfds-button" id="p21SetRule" disabled={busy} onclick={submitRule}>{t('Добавить / обновить правило в черновике','Add / update draft rule')}</button>
    <section class="panel rfds-card p22-inline-route" id="p22RouteCard">
      <h2>{t('Приоритеты и проверка маршрута','Route precedence and explanation')}</h2>
      <details class="p22-precedence-details"><summary>{t('Как выбирается DNS · приоритеты правил','DNS precedence and rule priority')}</summary><p data-p21-i18n="precedence">1. Домен устройства → 2. DNS устройства → 3. Группа устройств → 4. Общие домены → 5. DNS по умолчанию. При включённом наследовании общие домены проверяются перед соответствующим DNS.</p></details>
      <div class="fields two p21-form p22-route-form">
        <label class="field"><span>{t('Устройство на роутере','Device on router')}</span><select id="p22ProbePick" bind:this={routePick} onchange={selectRouteDevice}><option value="">{t('MAC вручную','Manual MAC')}</option>{#each discovered as item (item.mac)}<option value={item.mac}>{item.name||item.mac} · {item.mac}</option>{/each}</select></label>
        <label class="field"><span>{t('MAC устройства','Device MAC')}</span><input id="p21ProbeMAC" bind:this={routeMAC} maxlength="17" spellcheck="false"/></label>
        <label class="field"><span>{t('Домен','Domain')}</span><input id="p21ProbeDomain" bind:this={routeDomain} placeholder="example.com" maxlength="253"/></label>
      </div>
      <div class="p22-route-footer"><label class="check"><input id="p21ProbeApplied" bind:this={routeApplied} type="checkbox" checked/><span>{t('Проверять применённые правила (иначе сохранённый черновик)','Evaluate applied rules (otherwise saved draft)')}</span></label><button class="button" id="p21Probe" disabled={busy} onclick={()=>onAction('probe',{domain:routeDomain.value,mac:routeMAC.value,applied:routeApplied.checked})}>{t('Объяснить маршрут','Explain route')}</button></div>
      <DnsP22Visual result={route} mac={probeMac} devices={discovered}/>
      <details class="p22-technical"><summary>{t('Технические подробности (JSON)','Technical details (JSON)')}</summary><pre id="p21Explanation" class="p21-explanation">{route?JSON.stringify(route,null,2):''}</pre></details>
    </section>
  </section>
</div>
