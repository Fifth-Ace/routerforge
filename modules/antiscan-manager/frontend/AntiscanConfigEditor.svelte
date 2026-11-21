<script>
  import { rfUi } from './rf-localization.js';
  // Svelte owns the editable ascn.conf fields; protected mutation stays in the parent command adapter.
  let {controls = $bindable(), onChanged, onReset, onApply} = $props();
</script>
      <form id="configForm" class={controls["configForm"].className} autocomplete="off" hidden={controls["configForm"].hidden} onsubmit={(event) => event.preventDefault()} oninput={onChanged} onchange={onChanged}>{#if controls["configForm"].textContent !== undefined}{controls["configForm"].textContent}{:else}
        <section class="panel config-header-panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">{rfUi("БЕЗОПАСНАЯ НАСТРОЙКА","SAFE CONFIGURATION")}</span>
              <h2>{rfUi("Настройки ascn.conf","ascn.conf settings")}</h2>
              <p>{rfUi("Только известные параметры Antiscan, без произвольного shell-текста. Комментарии существующего файла сохраняются.","Only supported Antiscan settings are accepted, not arbitrary shell commands. Existing comments are preserved.")}</p>
            </div>
            <div class="config-state-wrap">
              <span id="configState" class={controls["configState"].className} hidden={controls["configState"].hidden}>{#if controls["configState"].textContent !== undefined}{controls["configState"].textContent}{:else}{rfUi("ПРОВЕРКА…","CHECKING…")}{/if}</span>
              <span id="configHash" class={controls["configHash"].className} hidden={controls["configHash"].hidden}>{#if controls["configHash"].textContent !== undefined}{controls["configHash"].textContent}{:else}SHA256 —{/if}</span>
            </div>
          </div>
          <div class="config-transaction-strip">
            <span>{rfUi("проверка","validation")}</span><b>→</b><span>{rfUi("резервная копия","backup")}</span><b>→</b><span>{rfUi("атомарная запись","atomic write")}</span><b>→</b><span>{rfUi("перечитывание Antiscan","Antiscan reload")}</span><b>→</b><span>{rfUi("проверка результата","result verification")}</span><b>→</b><span>{rfUi("откат при ошибке","rollback on failure")}</span>
          </div>
          <div id="configMobileRisk" class={controls["configMobileRisk"].className} hidden={controls["configMobileRisk"].hidden}>{#if controls["configMobileRisk"].textContent !== undefined}{controls["configMobileRisk"].textContent}{:else}{rfUi("Проверяем риск ложной блокировки /24…","Checking for accidental /24 blocks…")}{/if}</div>
        </section>

        <div class="config-grid">
          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">{rfUi("ТРАФИК","TRAFFIC")}</span><h2>{rfUi("Интерфейсы и порты","Interfaces and ports")}</h2></div></div>
            <div class="field-grid">
              <label class="field wide"><span>{rfUi("Интернет-интерфейсы","Internet interfaces")}</span><input id="cfgIspInterfaces" spellcheck="false" placeholder="eth3 ppp0" class={controls["cfgIspInterfaces"].className} hidden={controls["cfgIspInterfaces"].hidden} disabled={controls["cfgIspInterfaces"].disabled} bind:value={controls["cfgIspInterfaces"].value}><small><span class="mono">ISP_INTERFACES</span> {rfUi("· 1–10 имён интерфейсов через пробел.","· 1–10 interface names separated by spaces.")}</small></label>
              <label class="field"><span>{rfUi("Порты роутера","Router ports")}</span><input id="cfgPorts" spellcheck="false" placeholder="22,80,443" class={controls["cfgPorts"].className} hidden={controls["cfgPorts"].hidden} disabled={controls["cfgPorts"].disabled} bind:value={controls["cfgPorts"].value}><small>{rfUi("Порты/диапазоны через запятую.","Ports or ranges, comma-separated.")}</small></label>
              <label class="field"><span>{rfUi("Проброшенные порты","Forwarded ports")}</span><input id="cfgForwardedPorts" spellcheck="false" placeholder="22,80,443" class={controls["cfgForwardedPorts"].className} hidden={controls["cfgForwardedPorts"].hidden} disabled={controls["cfgForwardedPorts"].disabled} bind:value={controls["cfgForwardedPorts"].value}><small><span class="mono">PORTS_FORWARDED</span> {rfUi("· порты перенаправляемых подключений.","· ports of forwarded connections.")}</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">{rfUi("IP И ПОДСЕТИ","IP AND SUBNETS")}</span><h2>{rfUi("IP и /24 защита","IP and /24 protection")}</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgEnableIpsBan" type="checkbox" role="switch" class={controls["cfgEnableIpsBan"].className} hidden={controls["cfgEnableIpsBan"].hidden} disabled={controls["cfgEnableIpsBan"].disabled} bind:checked={controls["cfgEnableIpsBan"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>ENABLE_IPS_BAN</strong><small>{rfUi("Прямые IP и распределённая /24 защита.","Direct IP blocks and distributed /24 protection.")}</small></span></label>
              <label class="field"><span>RULES_MASK</span><input id="cfgRulesMask" spellcheck="false" placeholder="255.255.255.255" class={controls["cfgRulesMask"].className} hidden={controls["cfgRulesMask"].hidden} disabled={controls["cfgRulesMask"].disabled} bind:value={controls["cfgRulesMask"].value}></label>
              <label class="field"><span>{rfUi("Окно новых подключений, сек.","New connection window, sec")}</span><input id="cfgRecentTime" type="number" min="2" max="3600" class={controls["cfgRecentTime"].className} hidden={controls["cfgRecentTime"].hidden} disabled={controls["cfgRecentTime"].disabled} bind:value={controls["cfgRecentTime"].value}><small class="mono">RECENT_CONNECTIONS_TIME</small></label>
              <label class="field"><span>{rfUi("Порог новых подключений","New connection threshold")}</span><input id="cfgRecentHitcount" type="number" min="2" max="100" class={controls["cfgRecentHitcount"].className} hidden={controls["cfgRecentHitcount"].hidden} disabled={controls["cfgRecentHitcount"].disabled} bind:value={controls["cfgRecentHitcount"].value}><small class="mono">RECENT_CONNECTIONS_HITCOUNT</small></label>
              <label class="field"><span>{rfUi("Лимит одновременных соединений","Concurrent connection limit")}</span><input id="cfgRecentLimit" type="number" min="3" max="1000" class={controls["cfgRecentLimit"].className} hidden={controls["cfgRecentLimit"].hidden} disabled={controls["cfgRecentLimit"].disabled} bind:value={controls["cfgRecentLimit"].value}><small class="mono">RECENT_CONNECTIONS_LIMIT</small></label>
              <label class="field"><span>{rfUi("Блокировка IP, сек.","IP block duration, sec")}</span><input id="cfgRecentBanTime" type="number" min="0" max="2147483" class={controls["cfgRecentBanTime"].className} hidden={controls["cfgRecentBanTime"].hidden} disabled={controls["cfgRecentBanTime"].disabled} bind:value={controls["cfgRecentBanTime"].value}><small class="mono">RECENT_CONNECTIONS_BANTIME</small></label>
              <label class="field"><span>{rfUi("Кандидаты /24 живут, сек.","/24 candidate retention, sec")}</span><input id="cfgCandidateStorage" type="number" min="0" max="2147483" class={controls["cfgCandidateStorage"].className} hidden={controls["cfgCandidateStorage"].hidden} disabled={controls["cfgCandidateStorage"].disabled} bind:value={controls["cfgCandidateStorage"].value}></label>
              <label class="field"><span>{rfUi("Порог разных IP в /24","Distinct IP threshold per /24")}</span><input id="cfgDifferentThreshold" type="number" min="2" max="254" class={controls["cfgDifferentThreshold"].className} hidden={controls["cfgDifferentThreshold"].hidden} disabled={controls["cfgDifferentThreshold"].disabled} bind:value={controls["cfgDifferentThreshold"].value}></label>
              <label class="field"><span>{rfUi("Блокировка /24, сек.","/24 block duration, sec")}</span><input id="cfgSubnetBanTime" type="number" min="0" max="2147483" class={controls["cfgSubnetBanTime"].className} hidden={controls["cfgSubnetBanTime"].hidden} disabled={controls["cfgSubnetBanTime"].disabled} bind:value={controls["cfgSubnetBanTime"].value}><small class="mono">SUBNETS_BANTIME</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">{rfUi("ЛОВУШКА","HONEYPOT")}</span><h2>{rfUi("Ловушка","Honeypot")}</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgEnableHoneypot" type="checkbox" role="switch" class={controls["cfgEnableHoneypot"].className} hidden={controls["cfgEnableHoneypot"].hidden} disabled={controls["cfgEnableHoneypot"].disabled} bind:checked={controls["cfgEnableHoneypot"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>ENABLE_HONEYPOT</strong><small>{rfUi("При включении список портов обязателен.","Ports must be specified when enabled.")}</small></span></label>
              <label class="field"><span>{rfUi("Порты ловушки","Honeypot ports")}</span><input id="cfgHoneypotPorts" spellcheck="false" placeholder="23,2323,3389" class={controls["cfgHoneypotPorts"].className} hidden={controls["cfgHoneypotPorts"].hidden} disabled={controls["cfgHoneypotPorts"].disabled} bind:value={controls["cfgHoneypotPorts"].value}><small class="mono">HONEYPOT_PORTS</small></label>
              <label class="field"><span>{rfUi("Время блокировки, сек.","Block duration, sec")}</span><input id="cfgHoneypotBanTime" type="number" min="0" max="2147483" class={controls["cfgHoneypotBanTime"].className} hidden={controls["cfgHoneypotBanTime"].hidden} disabled={controls["cfgHoneypotBanTime"].disabled} bind:value={controls["cfgHoneypotBanTime"].value}><small class="mono">HONEYPOT_BANTIME</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">{rfUi("СПИСКИ И GEO","LISTS AND GEO")}</span><h2>{rfUi("Списки и страны","Lists and countries")}</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgUseCustomExclude" type="checkbox" role="switch" class={controls["cfgUseCustomExclude"].className} hidden={controls["cfgUseCustomExclude"].hidden} disabled={controls["cfgUseCustomExclude"].disabled} bind:checked={controls["cfgUseCustomExclude"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>{rfUi("Пользовательские исключения","User exclusions")}</strong><small><span class="mono">USE_CUSTOM_EXCLUDE_LIST</span> {rfUi("· использовать ascn_custom_exclude.","· use ascn_custom_exclude.")}</small></span></label>
              <label class="field"><span>{rfUi("Режим пользовательского списка","User list mode")}</span><select id="cfgCustomMode" class={controls["cfgCustomMode"].className} hidden={controls["cfgCustomMode"].hidden} disabled={controls["cfgCustomMode"].disabled} bind:value={controls["cfgCustomMode"].value}><option value="0">{rfUi("Выключено","Disabled")}</option><option value="blacklist">{rfUi("Чёрный список","Blocklist")}</option><option value="whitelist">{rfUi("Белый список","Allowlist")}</option></select><small class="mono">CUSTOM_LISTS_BLOCK_MODE</small></label>
              <label class="field"><span>{rfUi("Режим Geo","Geo mode")}</span><select id="cfgGeoMode" class={controls["cfgGeoMode"].className} hidden={controls["cfgGeoMode"].hidden} disabled={controls["cfgGeoMode"].disabled} bind:value={controls["cfgGeoMode"].value}><option value="0">{rfUi("Выключено","Disabled")}</option><option value="blacklist">{rfUi("Чёрный список","Blocklist")}</option><option value="whitelist">{rfUi("Белый список","Allowlist")}</option></select><small class="mono">GEOBLOCK_MODE</small></label>
              <label class="field wide"><span>{rfUi("Страны Geo","Geo countries")}</span><input id="cfgGeoCountries" spellcheck="false" placeholder="RU US DE" class={controls["cfgGeoCountries"].className} hidden={controls["cfgGeoCountries"].hidden} disabled={controls["cfgGeoCountries"].disabled} bind:value={controls["cfgGeoCountries"].value}><small><span class="mono">GEOBLOCK_COUNTRIES</span> {rfUi("· до 8 ISO-кодов, регистр исправляется автоматически.","· up to 8 ISO country codes; capitalization is corrected automatically.")}</small></label>
              <label class="field wide"><span>{rfUi("Страны-исключения Geo","Excluded Geo countries")}</span><input id="cfgGeoExcludeCountries" spellcheck="false" placeholder="FI DE" class={controls["cfgGeoExcludeCountries"].className} hidden={controls["cfgGeoExcludeCountries"].hidden} disabled={controls["cfgGeoExcludeCountries"].disabled} bind:value={controls["cfgGeoExcludeCountries"].value}><small class="mono">GEO_EXCLUDE_COUNTRIES</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">{rfUi("ХРАНЕНИЕ / KEENETIC","STORAGE / KEENETIC")}</span><h2>{rfUi("Хранение и lockout","Storage and lockouts")}</h2></div></div>
            <div class="field-grid">
              <label class="field wide"><span>IPSETS_DIRECTORY</span><input id="cfgIPSetsDirectory" spellcheck="false" placeholder="/tmp/mnt/DISK/antiscan" class={controls["cfgIPSetsDirectory"].className} hidden={controls["cfgIPSetsDirectory"].hidden} disabled={controls["cfgIPSetsDirectory"].disabled} bind:value={controls["cfgIPSetsDirectory"].value}><small>{rfUi("Путь внутри /opt или /tmp. Upstream запрещает только сам /opt/etc; вложенные каталоги, например /opt/etc/antiscan/ipsets, допустимы.","The path must be under /opt or /tmp. The upstream project disallows only /opt/etc itself; subdirectories such as /opt/etc/antiscan/ipsets are allowed.")}</small></label>
              <label class="switch-field"><input id="cfgSaveIPSets" type="checkbox" role="switch" class={controls["cfgSaveIPSets"].className} hidden={controls["cfgSaveIPSets"].hidden} disabled={controls["cfgSaveIPSets"].disabled} bind:checked={controls["cfgSaveIPSets"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>{rfUi("Сохранять наборы","Persist sets")}</strong><small><span class="mono">SAVE_IPSETS</span> {rfUi("· сохранять накопленные ipset.","· persist collected ipset entries.")}</small></span></label>
              <label class="switch-field"><input id="cfgSaveOnExit" type="checkbox" role="switch" class={controls["cfgSaveOnExit"].className} hidden={controls["cfgSaveOnExit"].hidden} disabled={controls["cfgSaveOnExit"].disabled} bind:checked={controls["cfgSaveOnExit"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>{rfUi("Сохранять при остановке","Save on stop")}</strong><small><span class="mono">SAVE_ON_EXIT</span> {rfUi("· экспортировать состояние при остановке.","· export state on shutdown.")}</small></span></label>
              <label class="switch-field"><input id="cfgReadNDM" type="checkbox" role="switch" class={controls["cfgReadNDM"].className} hidden={controls["cfgReadNDM"].hidden} disabled={controls["cfgReadNDM"].disabled} bind:checked={controls["cfgReadNDM"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>{rfUi("Блокировки Keenetic","Keenetic lockouts")}</strong><small><span class="mono">READ_NDM_LOCKOUT_IPSETS</span> {rfUi("· импортировать ip lockout-policy.","· import the IP lockout policy.")}</small></span></label>
              <label class="field"><span>{rfUi("Время блокировки Keenetic, сек.","Keenetic lockout duration, sec")}</span><input id="cfgLockoutBanTime" type="number" min="0" max="2147483" class={controls["cfgLockoutBanTime"].className} hidden={controls["cfgLockoutBanTime"].hidden} disabled={controls["cfgLockoutBanTime"].disabled} bind:value={controls["cfgLockoutBanTime"].value}><small class="mono">LOCKOUT_IPSET_BANTIME</small></label>
            </div>
          </section>
        </div>

        <section class="panel config-apply-panel">
          <div class="config-apply-copy">
            <strong>{rfUi("Применение настроек","Apply settings")}</strong>
            <span id="configHint" class={controls["configHint"].className} hidden={controls["configHint"].hidden}>{#if controls["configHint"].textContent !== undefined}{controls["configHint"].textContent}{:else}{rfUi("Состояние конфигурации загружается…","Loading configuration status…")}{/if}</span>
          </div>
          <div class="config-actions">
            <button id="resetConfig" class={controls["resetConfig"].className} type="button" disabled={controls["resetConfig"].disabled} hidden={controls["resetConfig"].hidden} onclick={onReset}>{#if controls["resetConfig"].textContent !== undefined}{controls["resetConfig"].textContent}{:else}{rfUi("Сбросить форму","Reset form")}{/if}</button>
            <button id="applyConfig" class={controls["applyConfig"].className} type="button" disabled={controls["applyConfig"].disabled} hidden={controls["applyConfig"].hidden} onclick={onApply}>{#if controls["applyConfig"].textContent !== undefined}{controls["applyConfig"].textContent}{:else}{rfUi("Применить конфигурацию","Apply configuration")}{/if}</button>
          </div>
        </section>
      {/if}</form>
