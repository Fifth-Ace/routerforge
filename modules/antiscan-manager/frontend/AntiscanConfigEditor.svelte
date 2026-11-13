<script>
  // Svelte owns the editable ascn.conf fields; protected mutation stays in the parent command adapter.
  let {controls = $bindable(), onChanged, onReset, onApply} = $props();
</script>
      <form id="configForm" class={controls["configForm"].className} autocomplete="off" hidden={controls["configForm"].hidden} onsubmit={(event) => event.preventDefault()} oninput={onChanged} onchange={onChanged}>{#if controls["configForm"].textContent !== undefined}{controls["configForm"].textContent}{:else}
        <section class="panel config-header-panel">
          <div class="panel-head">
            <div>
              <span class="section-kicker">БЕЗОПАСНАЯ НАСТРОЙКА</span>
              <h2>Настройки ascn.conf</h2>
              <p>Только известные параметры Antiscan, без произвольного shell-текста. Комментарии существующего файла сохраняются.</p>
            </div>
            <div class="config-state-wrap">
              <span id="configState" class={controls["configState"].className} hidden={controls["configState"].hidden}>{#if controls["configState"].textContent !== undefined}{controls["configState"].textContent}{:else}ПРОВЕРКА…{/if}</span>
              <span id="configHash" class={controls["configHash"].className} hidden={controls["configHash"].hidden}>{#if controls["configHash"].textContent !== undefined}{controls["configHash"].textContent}{:else}SHA256 —{/if}</span>
            </div>
          </div>
          <div class="config-transaction-strip">
            <span>проверка</span><b>→</b><span>резервная копия</span><b>→</b><span>атомарная запись</span><b>→</b><span>перечитывание Antiscan</span><b>→</b><span>проверка результата</span><b>→</b><span>откат при ошибке</span>
          </div>
          <div id="configMobileRisk" class={controls["configMobileRisk"].className} hidden={controls["configMobileRisk"].hidden}>{#if controls["configMobileRisk"].textContent !== undefined}{controls["configMobileRisk"].textContent}{:else}Проверяем риск ложной блокировки /24…{/if}</div>
        </section>

        <div class="config-grid">
          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">ТРАФИК</span><h2>Интерфейсы и порты</h2></div></div>
            <div class="field-grid">
              <label class="field wide"><span>Интернет-интерфейсы</span><input id="cfgIspInterfaces" spellcheck="false" placeholder="eth3 ppp0" class={controls["cfgIspInterfaces"].className} hidden={controls["cfgIspInterfaces"].hidden} disabled={controls["cfgIspInterfaces"].disabled} bind:value={controls["cfgIspInterfaces"].value}><small><span class="mono">ISP_INTERFACES</span> · 1–10 имён интерфейсов через пробел.</small></label>
              <label class="field"><span>Порты роутера</span><input id="cfgPorts" spellcheck="false" placeholder="22,80,443" class={controls["cfgPorts"].className} hidden={controls["cfgPorts"].hidden} disabled={controls["cfgPorts"].disabled} bind:value={controls["cfgPorts"].value}><small>Порты/диапазоны через запятую.</small></label>
              <label class="field"><span>Проброшенные порты</span><input id="cfgForwardedPorts" spellcheck="false" placeholder="22,80,443" class={controls["cfgForwardedPorts"].className} hidden={controls["cfgForwardedPorts"].hidden} disabled={controls["cfgForwardedPorts"].disabled} bind:value={controls["cfgForwardedPorts"].value}><small><span class="mono">PORTS_FORWARDED</span> · порты перенаправляемых подключений.</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">IP И ПОДСЕТИ</span><h2>IP и /24 защита</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgEnableIpsBan" type="checkbox" role="switch" class={controls["cfgEnableIpsBan"].className} hidden={controls["cfgEnableIpsBan"].hidden} disabled={controls["cfgEnableIpsBan"].disabled} bind:checked={controls["cfgEnableIpsBan"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>ENABLE_IPS_BAN</strong><small>Прямые IP и распределённая /24 защита.</small></span></label>
              <label class="field"><span>RULES_MASK</span><input id="cfgRulesMask" spellcheck="false" placeholder="255.255.255.255" class={controls["cfgRulesMask"].className} hidden={controls["cfgRulesMask"].hidden} disabled={controls["cfgRulesMask"].disabled} bind:value={controls["cfgRulesMask"].value}></label>
              <label class="field"><span>Окно новых подключений, сек.</span><input id="cfgRecentTime" type="number" min="2" max="3600" class={controls["cfgRecentTime"].className} hidden={controls["cfgRecentTime"].hidden} disabled={controls["cfgRecentTime"].disabled} bind:value={controls["cfgRecentTime"].value}><small class="mono">RECENT_CONNECTIONS_TIME</small></label>
              <label class="field"><span>Порог новых подключений</span><input id="cfgRecentHitcount" type="number" min="2" max="100" class={controls["cfgRecentHitcount"].className} hidden={controls["cfgRecentHitcount"].hidden} disabled={controls["cfgRecentHitcount"].disabled} bind:value={controls["cfgRecentHitcount"].value}><small class="mono">RECENT_CONNECTIONS_HITCOUNT</small></label>
              <label class="field"><span>Лимит одновременных соединений</span><input id="cfgRecentLimit" type="number" min="3" max="1000" class={controls["cfgRecentLimit"].className} hidden={controls["cfgRecentLimit"].hidden} disabled={controls["cfgRecentLimit"].disabled} bind:value={controls["cfgRecentLimit"].value}><small class="mono">RECENT_CONNECTIONS_LIMIT</small></label>
              <label class="field"><span>Блокировка IP, сек.</span><input id="cfgRecentBanTime" type="number" min="0" max="2147483" class={controls["cfgRecentBanTime"].className} hidden={controls["cfgRecentBanTime"].hidden} disabled={controls["cfgRecentBanTime"].disabled} bind:value={controls["cfgRecentBanTime"].value}><small class="mono">RECENT_CONNECTIONS_BANTIME</small></label>
              <label class="field"><span>Кандидаты /24 живут, сек.</span><input id="cfgCandidateStorage" type="number" min="0" max="2147483" class={controls["cfgCandidateStorage"].className} hidden={controls["cfgCandidateStorage"].hidden} disabled={controls["cfgCandidateStorage"].disabled} bind:value={controls["cfgCandidateStorage"].value}></label>
              <label class="field"><span>Порог разных IP в /24</span><input id="cfgDifferentThreshold" type="number" min="2" max="254" class={controls["cfgDifferentThreshold"].className} hidden={controls["cfgDifferentThreshold"].hidden} disabled={controls["cfgDifferentThreshold"].disabled} bind:value={controls["cfgDifferentThreshold"].value}></label>
              <label class="field"><span>Блокировка /24, сек.</span><input id="cfgSubnetBanTime" type="number" min="0" max="2147483" class={controls["cfgSubnetBanTime"].className} hidden={controls["cfgSubnetBanTime"].hidden} disabled={controls["cfgSubnetBanTime"].disabled} bind:value={controls["cfgSubnetBanTime"].value}><small class="mono">SUBNETS_BANTIME</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">ЛОВУШКА</span><h2>Ловушка</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgEnableHoneypot" type="checkbox" role="switch" class={controls["cfgEnableHoneypot"].className} hidden={controls["cfgEnableHoneypot"].hidden} disabled={controls["cfgEnableHoneypot"].disabled} bind:checked={controls["cfgEnableHoneypot"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>ENABLE_HONEYPOT</strong><small>При включении список портов обязателен.</small></span></label>
              <label class="field"><span>Порты ловушки</span><input id="cfgHoneypotPorts" spellcheck="false" placeholder="23,2323,3389" class={controls["cfgHoneypotPorts"].className} hidden={controls["cfgHoneypotPorts"].hidden} disabled={controls["cfgHoneypotPorts"].disabled} bind:value={controls["cfgHoneypotPorts"].value}><small class="mono">HONEYPOT_PORTS</small></label>
              <label class="field"><span>Время блокировки, сек.</span><input id="cfgHoneypotBanTime" type="number" min="0" max="2147483" class={controls["cfgHoneypotBanTime"].className} hidden={controls["cfgHoneypotBanTime"].hidden} disabled={controls["cfgHoneypotBanTime"].disabled} bind:value={controls["cfgHoneypotBanTime"].value}><small class="mono">HONEYPOT_BANTIME</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">СПИСКИ И GEO</span><h2>Списки и страны</h2></div></div>
            <div class="field-grid">
              <label class="switch-field wide"><input id="cfgUseCustomExclude" type="checkbox" role="switch" class={controls["cfgUseCustomExclude"].className} hidden={controls["cfgUseCustomExclude"].hidden} disabled={controls["cfgUseCustomExclude"].disabled} bind:checked={controls["cfgUseCustomExclude"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Пользовательские исключения</strong><small><span class="mono">USE_CUSTOM_EXCLUDE_LIST</span> · использовать ascn_custom_exclude.</small></span></label>
              <label class="field"><span>Режим пользовательского списка</span><select id="cfgCustomMode" class={controls["cfgCustomMode"].className} hidden={controls["cfgCustomMode"].hidden} disabled={controls["cfgCustomMode"].disabled} bind:value={controls["cfgCustomMode"].value}><option value="0">Выключено</option><option value="blacklist">Чёрный список</option><option value="whitelist">Белый список</option></select><small class="mono">CUSTOM_LISTS_BLOCK_MODE</small></label>
              <label class="field"><span>Режим Geo</span><select id="cfgGeoMode" class={controls["cfgGeoMode"].className} hidden={controls["cfgGeoMode"].hidden} disabled={controls["cfgGeoMode"].disabled} bind:value={controls["cfgGeoMode"].value}><option value="0">Выключено</option><option value="blacklist">Чёрный список</option><option value="whitelist">Белый список</option></select><small class="mono">GEOBLOCK_MODE</small></label>
              <label class="field wide"><span>Страны Geo</span><input id="cfgGeoCountries" spellcheck="false" placeholder="RU US DE" class={controls["cfgGeoCountries"].className} hidden={controls["cfgGeoCountries"].hidden} disabled={controls["cfgGeoCountries"].disabled} bind:value={controls["cfgGeoCountries"].value}><small><span class="mono">GEOBLOCK_COUNTRIES</span> · до 8 ISO-кодов, регистр исправляется автоматически.</small></label>
              <label class="field wide"><span>Страны-исключения Geo</span><input id="cfgGeoExcludeCountries" spellcheck="false" placeholder="FI DE" class={controls["cfgGeoExcludeCountries"].className} hidden={controls["cfgGeoExcludeCountries"].hidden} disabled={controls["cfgGeoExcludeCountries"].disabled} bind:value={controls["cfgGeoExcludeCountries"].value}><small class="mono">GEO_EXCLUDE_COUNTRIES</small></label>
            </div>
          </section>

          <section class="panel config-card">
            <div class="panel-head"><div><span class="section-kicker">ХРАНЕНИЕ / KEENETIC</span><h2>Хранение и lockout</h2></div></div>
            <div class="field-grid">
              <label class="field wide"><span>IPSETS_DIRECTORY</span><input id="cfgIPSetsDirectory" spellcheck="false" placeholder="/tmp/mnt/DISK/antiscan" class={controls["cfgIPSetsDirectory"].className} hidden={controls["cfgIPSetsDirectory"].hidden} disabled={controls["cfgIPSetsDirectory"].disabled} bind:value={controls["cfgIPSetsDirectory"].value}><small>Путь внутри /opt или /tmp. Upstream запрещает только сам /opt/etc; вложенные каталоги, например /opt/etc/antiscan/ipsets, допустимы.</small></label>
              <label class="switch-field"><input id="cfgSaveIPSets" type="checkbox" role="switch" class={controls["cfgSaveIPSets"].className} hidden={controls["cfgSaveIPSets"].hidden} disabled={controls["cfgSaveIPSets"].disabled} bind:checked={controls["cfgSaveIPSets"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Сохранять наборы</strong><small><span class="mono">SAVE_IPSETS</span> · сохранять накопленные ipset.</small></span></label>
              <label class="switch-field"><input id="cfgSaveOnExit" type="checkbox" role="switch" class={controls["cfgSaveOnExit"].className} hidden={controls["cfgSaveOnExit"].hidden} disabled={controls["cfgSaveOnExit"].disabled} bind:checked={controls["cfgSaveOnExit"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Сохранять при остановке</strong><small><span class="mono">SAVE_ON_EXIT</span> · экспортировать состояние при остановке.</small></span></label>
              <label class="switch-field"><input id="cfgReadNDM" type="checkbox" role="switch" class={controls["cfgReadNDM"].className} hidden={controls["cfgReadNDM"].hidden} disabled={controls["cfgReadNDM"].disabled} bind:checked={controls["cfgReadNDM"].checked}><span class="rf-switch-status" aria-hidden="true"></span><span class="rf-switch-track" aria-hidden="true"><span></span></span><span><strong>Блокировки Keenetic</strong><small><span class="mono">READ_NDM_LOCKOUT_IPSETS</span> · импортировать ip lockout-policy.</small></span></label>
              <label class="field"><span>Время блокировки Keenetic, сек.</span><input id="cfgLockoutBanTime" type="number" min="0" max="2147483" class={controls["cfgLockoutBanTime"].className} hidden={controls["cfgLockoutBanTime"].hidden} disabled={controls["cfgLockoutBanTime"].disabled} bind:value={controls["cfgLockoutBanTime"].value}><small class="mono">LOCKOUT_IPSET_BANTIME</small></label>
            </div>
          </section>
        </div>

        <section class="panel config-apply-panel">
          <div class="config-apply-copy">
            <strong>Применение настроек</strong>
            <span id="configHint" class={controls["configHint"].className} hidden={controls["configHint"].hidden}>{#if controls["configHint"].textContent !== undefined}{controls["configHint"].textContent}{:else}Состояние конфигурации загружается…{/if}</span>
          </div>
          <div class="config-actions">
            <button id="resetConfig" class={controls["resetConfig"].className} type="button" disabled={controls["resetConfig"].disabled} hidden={controls["resetConfig"].hidden} onclick={onReset}>{#if controls["resetConfig"].textContent !== undefined}{controls["resetConfig"].textContent}{:else}Сбросить форму{/if}</button>
            <button id="applyConfig" class={controls["applyConfig"].className} type="button" disabled={controls["applyConfig"].disabled} hidden={controls["applyConfig"].hidden} onclick={onApply}>{#if controls["applyConfig"].textContent !== undefined}{controls["applyConfig"].textContent}{:else}Применить конфигурацию{/if}</button>
          </div>
        </section>
      {/if}</form>
