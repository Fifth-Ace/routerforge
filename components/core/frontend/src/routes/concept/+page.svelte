<script>
  const modules = [
    { id:'overview', label:'Карта концепта', short:'Overview', icon:'◇' },
    { id:'appcenter', label:'Центр приложений', short:'App Center', icon:'▦' },
    { id:'monitoring', label:'Мониторинг', short:'Monitoring', icon:'▥' },
    { id:'dns', label:'DNS', short:'DNS', icon:'◎' },
    { id:'management', label:'Управление', short:'Management', icon:'⌘' },
    { id:'maintenance', label:'Обслуживание', short:'Maintenance', icon:'◫' },
    { id:'network', label:'Сетевые инструменты', short:'Network Tools', icon:'⌁' },
    { id:'integrations', label:'Интеграции', short:'Integrations', icon:'◇' },
    { id:'developer', label:'Инструменты разработчика', short:'Developer Tools', icon:'⌬' }
  ];

  const conceptCards = [
    { id:'appcenter', title:'Центр приложений', note:'Registry, источники, RouterForge-модули, интеграции и Entware.', phase:'P13', tone:'good' },
    { id:'monitoring', title:'Мониторинг', note:'System / Thermal / Storage / Network + Health & Alerts + Incident Timeline.', phase:'P21', tone:'good' },
    { id:'dns', title:'DNS', note:'Резолверы, диагностика, правила и будущий DNS Policy Router.', phase:'P18', tone:'accent' },
    { id:'management', title:'Управление', note:'Процессы, сервисы, пакеты, файлы, терминалы и Service Inspector.', phase:'P19', tone:'accent' },
    { id:'maintenance', title:'Обслуживание', note:'Config Vault, Backup/Recovery, Tasks, Watchdogs, Support Bundle, Storage Doctor.', phase:'P16/P22', tone:'warn' },
    { id:'network', title:'Сетевые инструменты', note:'Network Doctor, Route Inspector, Flow Explorer и Active Probes.', phase:'P17/P20', tone:'warn' },
    { id:'integrations', title:'Интеграции', note:'Обнаружение стороннего ПО и будущий NFQWS2 Manager.', phase:'P23', tone:'neutral' },
    { id:'developer', title:'Инструменты разработчика', note:'Profiling, manifest validation, API Explorer, debug tooling.', phase:'vNext', tone:'neutral' }
  ];

  let active = 'overview';
  let activeTab = {};
  let toast = '';
  let paused = false;

  const stats = [
    ['CPU','4%','−12%'], ['RAM','55%','−8%'], ['Температура','59°C','−3°C'],
    ['Хранилище','18%','22.4 / 128 GB'], ['Процессы','182','+12'], ['Сеть','125 Mbps','98↓ / 27↑']
  ];

  const services = [
    ['routerforge-core','active (running)','good'], ['routerforge-dns','active (running)','good'],
    ['nfqws2','active (running)','good'], ['dnsmasq','active (running)','good'], ['fail2ban','inactive (dead)','bad']
  ];

  const resolvers = [
    ['Cloudflare DoT','1.1.1.1:853','DoT','АКТИВЕН'], ['Google DoT','8.8.8.8:853','DoT','АКТИВЕН'],
    ['Yandex DoT','common.dot.dns.yandex.net:853','DoT','АКТИВЕН'], ['AdGuard Home','https://dns.adguard-home.lan/dns-query','DoH','АКТИВЕН'],
    ['Keenetic / DHCP','192.168.1.1:53','DNS','ДИНАМИЧЕСКИЙ']
  ];

  const events = [
    ['14:28:17','WARNING','Storage','Раздел /data приблизился к порогу 80%'],
    ['13:42:03','INFO','Monitoring','Сервис nfqws2 недоступен'],
    ['11:15:26','INFO','System','Температура CPU вернулась в норму'],
    ['08:03:11','INFO','Network','Интерфейс wlan0 отключён'],
    ['03:21:45','INFO','Updates','Установлены обновления: 3 пакета']
  ];

  const flows = [
    ['14:32:10','192.168.1.100','8.8.8.8','TCP','52344 → 53','ESTABLISHED'],
    ['14:32:08','192.168.1.105','142.250.72.14','TCP','52111 → 443','ESTABLISHED'],
    ['14:32:07','192.168.1.100','1.1.1.1','UDP','60021 → 53','ACTIVE'],
    ['14:32:05','192.168.1.42','93.184.216.34','TCP','49822 → 80','ESTABLISHED']
  ];

  const integrations = [
    ['nfqws Web UI','Веб-интерфейс управления nfqws.','OFFICIAL','Обнаружен','http://192.168.1.1:8080'],
    ['nfqws2','Обновлённая версия nfqws с расширенной диагностикой.','OFFICIAL','Обнаружен','http://192.168.1.1:8081'],
    ['AWG Manager','Управление AmneziaWG, клиентами и ключами.','OFFICIAL','Доступно обновление','—'],
    ['AdGuard Home','Сетевой DNS-фильтр с веб-интерфейсом.','EXTERNAL','Работает','http://192.168.1.1:3000'],
    ['Внешний Web UI','Автоматически обнаруженный локальный интерфейс.','EXTERNAL','Обнаружен','http://192.168.1.1:9090']
  ];

  const logLines = [
    ['14:41:27','INFO','core','RouterForge core started, version concept-r1'],
    ['14:41:28','INFO','dns','Loading resolvers from registry'],
    ['14:41:29','DEBUG','api','GET /api/apps 200 14ms'],
    ['14:41:31','WARN','storage','Disk usage is above 80% (82%)'],
    ['14:41:34','INFO','monitor','Collecting metrics (cpu, mem, net)'],
    ['14:41:36','ERROR','entware','Failed to fetch index: timeout']
  ];

  const appModules = [
    ['Core','Система','v0.7.1','Установлено'], ['DNS','Сеть','v0.7.1','Установлено'],
    ['Management','Управление','v0.7.1','Установлено'], ['Monitoring','Мониторинг','v0.7.1','Установлено']
  ];

  function openModule(id) { active = id; toast = ''; }
  function setTab(scope, value) { activeTab = { ...activeTab, [scope]: value }; }
  function mockAction(label) {
    toast = `CONCEPT R1: «${label}» — визуальный макет, действие не выполняется.`;
    setTimeout(() => { if (toast.includes(label)) toast = ''; }, 3200);
  }
</script>

<svelte:head><title>RouterForge Concept R1</title></svelte:head>

<div class="concept-page">
  <div class="concept-banner">
    <div><span class="concept-kicker">ROUTERFORGE / CONCEPT R1</span><strong>Визуальный прототип vNext</strong><small>Mock data · no mutations · branch: concept</small></div>
    <span class="concept-chip">SAFE MOCK</span>
  </div>

  <div class="concept-module-nav" role="tablist" aria-label="Concept modules">
    {#each modules as item}
      <button type="button" class:active={active === item.id} onclick={() => openModule(item.id)}>
        <span>{item.icon}</span><b>{item.label}</b>
      </button>
    {/each}
  </div>

  {#if toast}<div class="concept-toast">{toast}</div>{/if}

  {#if active === 'overview'}
    <section class="hero">
      <div>
        <span class="eyebrow">UI FOUNDATION</span>
        <h1>RouterForge vNext — отправная точка интерфейсов</h1>
        <p>Все крупные модули из Master Development Plan собраны здесь как безопасные кликабельные макеты. Это отдельная ветка для визуальной проверки на тестовом роутере, а не новая бизнес-логика.</p>
      </div>
      <div class="hero-status">
        <span>8 экранов</span><span>0 мутаций</span><span>1 общий shell</span>
      </div>
    </section>

    <div class="module-grid">
      {#each conceptCards as card}
        <button class="module-card" type="button" onclick={() => openModule(card.id)}>
          <div class="module-card-head"><strong>{card.title}</strong><span class="phase {card.tone}">{card.phase}</span></div>
          <p>{card.note}</p>
          <span class="open-hint">Открыть макет →</span>
        </button>
      {/each}
    </div>

    <section class="panel architecture">
      <div class="panel-head"><div><strong>Архитектурная граница</strong><span>Концепт следует модульной модели из Master Development Plan</span></div><span class="badge good">CORE MINIMAL</span></div>
      <div class="architecture-row">
        <div class="arch-node core">Core<br/><small>Shell · Auth · App Center · Registry</small></div>
        <div class="arch-arrow">→</div>
        <div class="arch-stack"><span>Monitoring</span><span>DNS</span><span>Management</span><span>Maintenance</span><span>Network Tools</span><span>Integrations</span><span>Developer Tools</span></div>
      </div>
      <div class="engine-row"><span>Policy Objects</span><span>Snapshot / Transaction</span><span>Probe Engine</span><span>Event Engine</span></div>
    </section>

  {:else if active === 'appcenter'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / APP CENTER</span><h1>Центр приложений</h1><p>RouterForge, проверенные интеграции и пакеты Entware в одном техническом каталоге.</p></div><div class="head-actions"><button onclick={() => mockAction('Управление источниками')}>Источники</button><button class="primary" onclick={() => mockAction('Проверить всё')}>Проверить всё</button></div></section>
    <div class="tabs"><button class="active">RouterForge <em>5</em></button><button>Интеграции <em>18</em></button><button>Entware <em>2969</em></button><button>Установлено <em>8</em></button></div>
    <div class="toolbar"><input value="" placeholder="Поиск приложений и пакетов…"/><select><option>Все категории</option></select><select><option>Все статусы</option></select><select><option>Все источники</option></select></div>
    <div class="two-col wide-right">
      <div>
        <h2>Официальные модули RouterForge</h2>
        <div class="app-grid">
          {#each appModules as app}
            <article class="app-card"><div class="app-icon">◆</div><div><strong>{app[0]}</strong><small>{app[1]} · {app[2]}</small><p>Технический модуль RouterForge. Совместимость и состояние проверяются локально.</p><span class="status good">● {app[3]}</span></div><div class="card-actions"><button onclick={() => mockAction(`Открыть ${app[0]}`)}>Открыть</button><button>Подробнее</button></div></article>
          {/each}
        </div>
        <h2>Интеграции</h2>
        <div class="app-grid compact">
          {#each [['AdGuard Home','DNS / фильтрация'],['WireGuard','VPN / сеть'],['OpenVPN','VPN / сеть'],['Mosquitto','IoT / MQTT']] as app}
            <article class="app-card"><div class="app-icon alt">◇</div><div><strong>{app[0]}</strong><small>{app[1]}</small><p>Обнаружение, версия, Web UI и разрешённые операции по manifest contract.</p><span class="status good">● Доступно</span></div><div class="card-actions"><button>Установить</button><button>Подробнее</button></div></article>
          {/each}
        </div>
      </div>
      <aside class="side-stack">
        <section class="panel"><div class="panel-head"><div><strong>Источники и репозитории</strong><span>Registry и provenance</span></div></div><div class="source"><b>RouterForge Registry</b><span class="badge good">DEFAULT</span><small>remote · verified metadata</small></div><div class="source"><b>Entware</b><span class="badge good">ONLINE</span><small>official feed</small></div><div class="source"><b>Local source</b><span class="badge warn">LOW TRUST</span><small>explicit opt-in only</small></div><button class="full">+ Добавить источник</button></section>
        <section class="panel danger-soft"><strong>Сторонние источники</strong><p>Метаданные не дают автоматических прав на выполнение lifecycle-команд. Действия закрыты по умолчанию.</p></section>
      </aside>
    </div>

  {:else if active === 'monitoring'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / MONITORING</span><h1>Мониторинг</h1><p>Состояние системы, метрики, предупреждения и причинная временная шкала.</p></div><div class="head-actions"><select><option>Последние 24 часа</option></select><button onclick={() => paused = !paused}>{paused ? 'Продолжить' : 'Пауза'}</button></div></section>
    <div class="tabs"><button class="active">System</button><button>Thermal</button><button>Storage</button><button>Network</button><button>Health & Alerts</button><button>Incident Timeline</button></div>
    <div class="metric-grid six">{#each stats as s}<article class="metric"><span>{s[0]}</span><strong>{s[1]}</strong><small>{s[2]}</small><div class="spark"><i></i><i></i><i></i><i></i><i></i><i></i></div></article>{/each}</div>
    <section class="health-strip"><span class="health-icon">✓</span><div><strong>Система работает нормально</strong><small>Критических инцидентов нет. Основные сервисы в сети.</small></div><div class="health-modules"><span>● Core <b>В сети</b></span><span>● DNS <b>В сети</b></span><span>● Management <b>В сети</b></span><span>⚠ 2 предупреждения</span></div></section>
    <div class="dashboard-grid three">
      <section class="panel chart"><div class="panel-head"><strong>Загрузка CPU</strong><span>Текущая: 4%</span></div><div class="fake-chart green"></div></section>
      <section class="panel chart"><div class="panel-head"><strong>Температура</strong><span>CPU 59°C</span></div><div class="fake-chart multi"></div></section>
      <section class="panel"><div class="panel-head"><strong>Использование хранилища</strong></div>{#each [['/ (root)','18%'],['/data','42%'],['/var','12%'],['/tmp','3%']] as d}<div class="bar-row"><span>{d[0]}</span><div><i style={`width:${d[1]}`}></i></div><b>{d[1]}</b></div>{/each}</section>
      <section class="panel chart"><div class="panel-head"><strong>Сетевой трафик</strong><span>98 ↓ / 27 ↑ Mbps</span></div><div class="fake-chart blue"></div></section>
      <section class="panel"><div class="panel-head"><strong>Топ хостов по трафику</strong></div>{#each [['192.168.1.100','48.2 Mbps','38%'],['10.0.0.5','22.1 Mbps','17%'],['172.16.0.10','11.4 Mbps','9%'],['8.8.8.8','6.8 Mbps','5%']] as h}<div class="rank-row"><span>{h[0]}</span><b>{h[1]}</b><div><i style={`width:${h[2]}`}></i></div></div>{/each}</section>
      <section class="panel"><div class="panel-head"><strong>Состояние интерфейсов</strong></div>{#each [['eth0','UP','98 / 27 Mbps'],['eth1','UP','5 / 3 Mbps'],['wlan0','DOWN','0 / 0 Mbps'],['tun0','UP','2 / 1 Mbps']] as i}<div class="list-row"><b>{i[0]}</b><span class:bad={i[1] === 'DOWN'} class="good-text">● {i[1]}</span><small>{i[2]}</small></div>{/each}</section>
    </div>
    <section class="panel"><div class="panel-head"><strong>Последние события и инциденты</strong><span class="linkish">Все инциденты →</span></div><table><thead><tr><th>Время</th><th>Уровень</th><th>Источник</th><th>Сообщение</th></tr></thead><tbody>{#each events as e}<tr><td class="mono">{e[0]}</td><td><span class="badge {e[1] === 'WARNING' ? 'warn' : 'info'}">{e[1]}</span></td><td>{e[2]}</td><td>{e[3]}</td></tr>{/each}</tbody></table></section>

  {:else if active === 'dns'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / DNS</span><h1>DNS</h1><p>Резолверы, маршрутизация, диагностика и будущий Policy Router.</p></div><div class="head-actions"><button>Обновить</button><button class="primary" onclick={() => mockAction('Добавить DNS')}>+ Добавить DNS</button></div></section>
    <div class="tabs"><button class="active">Резолверы</button><button>Правила</button><button>Трафик</button><button>Диагностика</button><button>Policy Router <em>CONCEPT</em></button></div>
    <div class="metric-grid six">{#each [['Настроено','5'],['Активные','4'],['Отключённые','0'],['Динамические','1'],['Пресеты каталога','20/20'],['DoT слоты','6/8']] as s}<article class="metric"><span>{s[0]}</span><strong>{s[1]}</strong></article>{/each}</div>
    <div class="resolver-grid">
      {#each resolvers as r}
        <article class="resolver-card"><div><span class="dot"></span><strong>{r[0]}</strong><span class="badge info">{r[2]}</span><small class="mono">{r[1]}</small></div><span class="badge {r[3] === 'АКТИВЕН' ? 'good' : 'neutral'}">{r[3]}</span><div class="resolver-actions"><button onclick={() => mockAction(`Сведения: ${r[0]}`)}>Сведения</button><button onclick={() => mockAction(`Настроить: ${r[0]}`)}>Настроить</button></div></article>
      {/each}
    </div>
    <section class="panel policy-preview"><div class="panel-head"><div><strong>DNS Policy Router — будущая рабочая модель</strong><span>Domain Group → Resolver → Route → Client scope</span></div><span class="badge warn">P18</span></div><div class="policy-flow"><span>Streaming</span><b>→</b><span>Cloudflare DoT</span><b>→</b><span>AWG / tun0</span><b>→</b><span>TV + Console</span></div><div class="policy-columns"><div><strong>Domain Groups</strong><p>Импорт, wildcard, дедупликация, источник.</p></div><div><strong>Explain domain path</strong><p>Показывает совпавшую группу, CNAME/IP, resolver и маршрут.</p></div><div><strong>Dry-run</strong><p>Предварительный просмотр изменений до безопасного apply.</p></div></div></section>

  {:else if active === 'management'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / MANAGEMENT</span><h1>Управление</h1><p>Процессы, сервисы, пакеты, файлы и консоли — в одном операционном workspace.</p></div><div class="head-actions"><button>Действия ▾</button><button>Перезагрузить</button><button class="danger">Выключить</button></div></section>
    <div class="tabs"><button class="active">Процессы</button><button>Сервисы</button><button>Пакеты / Порты</button><button>Файловый менеджер</button><button>Терминал</button><button>NDM Console</button><button>Service Inspector <em>CONCEPT</em></button></div>
    <div class="dashboard-grid three">
      <section class="panel"><div class="panel-head"><strong>Процессы</strong><button>Открыть в топе</button></div><div class="toolbar mini"><input placeholder="Поиск процессов…"/></div><table><thead><tr><th>PID</th><th>USER</th><th>CPU</th><th>RAM</th><th>COMMAND</th></tr></thead><tbody>{#each [['1','root','0.0%','12.1 MB','/sbin/init'],['318','root','0.1%','8.4 MB','/usr/sbin/nginx'],['612','root','0.3%','48.7 MB','/usr/bin/python3'],['720','www-data','1.2%','65.3 MB','/usr/bin/node']] as p}<tr>{#each p as c}<td>{c}</td>{/each}</tr>{/each}</tbody></table></section>
      <section class="panel"><div class="panel-head"><strong>Сервисы</strong><span>guarded actions</span></div>{#each services as s}<div class="service-row"><b>{s[0]}</b><span class="{s[2] === 'good' ? 'good-text' : 'bad-text'}">● {s[1]}</span><div><button>▶</button><button>↻</button><button class="danger">■</button></div></div>{/each}</section>
      <section class="panel"><div class="panel-head"><strong>Пакеты / Порты</strong></div><div class="mini-stat"><span>Установленные пакеты</span><strong>2969</strong></div><div class="mini-stat"><span>Открытые порты</span><strong>8</strong></div><table><tbody>{#each [['22','tcp','ssh'],['53','udp','domain'],['80','tcp','http'],['443','tcp','https'],['51820','udp','wireguard']] as p}<tr><td>{p[0]}</td><td>{p[1]}</td><td>{p[2]}</td><td><span class="badge good">open</span></td></tr>{/each}</tbody></table></section>
    </div>
    <section class="panel file-manager"><div class="panel-head"><strong>Файловый менеджер</strong><span class="mono">/etc/nginx</span><div><button>Создать</button><button>Загрузить</button></div></div><div class="file-body"><aside>{#each ['/','bin','etc','nginx','dnsmasq','ssh','opt','var'] as f}<div class:active={f==='nginx'}>▸ {f}</div>{/each}</aside><div>{#each [['conf.d','—','12 мар 2026'],['sites-available','—','12 мар 2026'],['nginx.conf','2.1 KB','12 мар 2026'],['mime.types','4.5 KB','12 мар 2026'],['proxy_params','1.0 KB','12 мар 2026']] as f}<div class="file-row"><span>▣ {f[0]}</span><span>{f[1]}</span><span>{f[2]}</span></div>{/each}</div></div></section>
    <section class="panel terminal"><div class="panel-head"><strong>Терминал</strong><span class="badge good">Система</span><button>Новая сессия</button></div><pre>root@routerforge:~# uname -a\nLinux routerforge 6.1.0 #325 SMP aarch64 GNU/Linux\nroot@routerforge:~# status nginx\n● nginx.service — active (running)</pre></section>

  {:else if active === 'maintenance'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / MAINTENANCE</span><h1>Обслуживание</h1><p>Диагностика, история конфигурации, восстановление и сервисные операции.</p></div><div class="head-actions"><button>Registry: REMOTE</button><button>Проверить всё</button></div></section>
    <div class="tabs"><button class="active">Логи</button><button>Config Vault</button><button>Backup / Recovery</button><button>Задачи</button><button>Watchdogs</button><button>Support Bundle</button><button>Storage Doctor</button></div>
    <div class="two-col wide-left">
      <section class="panel log-panel"><div class="panel-head"><div><strong>Просмотр логов</strong><span>Журналы системы и сервисов</span></div><div><button>Все</button><button>INFO</button><button>WARN</button><button>ERROR</button></div></div><div class="logs">{#each logLines as l}<div><span>{l[0]}</span><b class={l[1] === 'ERROR' ? 'bad-text' : ''}>{l[1]}</b><span>{l[2]}</span><code>{l[3]}</code></div>{/each}</div></section>
      <section class="panel"><div class="panel-head"><strong>Последние действия</strong><span>Event Engine view</span></div>{#each [['Резервное копирование завершено','14:32'],['Обновление конфигурации DNS','14:21'],['Запуск задачи logrotate','13:00'],['Очистка хранилища','11:48'],['Неудачное разрешение домена','10:17']] as a}<div class="activity-row"><span class="dot"></span><div><b>{a[0]}</b><small>{a[1]}</small></div></div>{/each}</section>
    </div>
    <div class="dashboard-grid three">
      <section class="panel"><div class="panel-head"><div><strong>Config Vault</strong><span>ACTIVE / SAVED / LAST WORKING</span></div><button class="primary">Создать snapshot</button></div>{#each [['LAST WORKING','dns','14:21','verified'],['SAVED','core','13:58','clean'],['SNAPSHOT','nfqws2','11:44','manual']] as v}<div class="vault-row"><span class="badge good">{v[0]}</span><b>{v[1]}</b><span>{v[2]}</span><small>{v[3]}</small><button>Diff</button></div>{/each}</section>
      <section class="panel"><div class="panel-head"><strong>Backup / Recovery</strong><button>Создать бэкап</button></div>{#each [['auto-2026-09-13_0300','245 MB'],['daily-2026-09-12','242 MB'],['pre-upgrade-2026-09-11','240 MB']] as b}<div class="list-row"><b>{b[0]}</b><span>{b[1]}</span><button>Восстановить</button></div>{/each}</section>
      <section class="panel"><div class="panel-head"><strong>Задачи / Watchdogs</strong><button>+ Добавить</button></div>{#each [['daily-backup','0 3 * * *','ON'],['logrotate','0 */6 * * *','ON'],['storage-cleanup','0 4 * * 0','ON'],['health-report','0 8 * * 1','OFF']] as c}<div class="list-row"><b>{c[0]}</b><span class="mono">{c[1]}</span><span class="badge {c[2]==='ON'?'good':'neutral'}">{c[2]}</span></div>{/each}</section>
      <section class="panel"><div class="panel-head"><strong>Storage Doctor</strong><span>82% используется</span></div><div class="usage"><i style="width:82%"></i></div>{#each [['Большие логи','/var/log','12.4 GB'],['Entware cache','/opt/entware/var/cache','1.8 GB'],['Временные файлы','/tmp','1.2 GB']] as s}<div class="list-row"><b>{s[0]}</b><span>{s[1]}</span><strong>{s[2]}</strong><button>Анализ</button></div>{/each}</section>
      <section class="panel"><div class="panel-head"><strong>Support Bundle</strong><button class="primary">Собрать bundle</button></div><p>Выбор коллекторов, preview файлов, детерминированная редактировка секретов и manifest с checksum.</p><label class="check"><input type="checkbox"/> Включить чувствительные идентификаторы</label><div class="bundle">Последний bundle <b>238 MB</b></div></section>
      <section class="panel danger-soft"><div class="panel-head"><strong>Аварийные действия</strong></div><div class="danger-action"><span>Перезагрузить систему</span><button>Перезагрузить</button></div><div class="danger-action"><span>Остановить сервисы</span><button>Остановить</button></div><div class="danger-action"><span>Сбросить конфигурацию</span><button class="danger">Выполнить</button></div></section>
    </div>

  {:else if active === 'network'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / NETWORK TOOLS</span><h1>Сетевые инструменты</h1><p>Причинная диагностика, объяснение маршрутов, потоки и активные проверки.</p></div><div class="head-actions"><button>Сохранить отчёт</button></div></section>
    <div class="tabs"><button class="active">Network Doctor</button><button>Route Inspector</button><button>Flow Explorer</button><button>Active Probes</button></div>
    <div class="metric-grid four">{#each [['Доступность сети','99.8%'],['Средняя задержка','18 ms'],['Потери пакетов','0.2%'],['Активные сессии','1,482']] as s}<article class="metric"><span>{s[0]}</span><strong>{s[1]}</strong><div class="spark"><i></i><i></i><i></i><i></i></div></article>{/each}</div>
    <div class="dashboard-grid two">
      <section class="panel doctor"><div class="panel-head"><div><strong>Быстрая проверка (Network Doctor)</strong><span>Первый доказанный FAIL, а не набор красных лампочек</span></div></div><div class="probe-form"><input value="8.8.8.8"/><select><option>Профиль: Стандартный</option></select><button class="primary" onclick={() => mockAction('Запустить Network Doctor')}>Запустить ▶</button></div>{#each [['Ping','Успешно','RTT 14 ms'],['DNS','Успешно','Время 12 ms'],['HTTP','Пропущено','—'],['TCP','Успешно','15 ms']] as p}<div class="probe-row"><b>{p[0]}</b><span class="badge {p[1]==='Успешно'?'good':'neutral'}">{p[1]}</span><span>{p[2]}</span><span>›</span></div>{/each}</section>
      <section class="panel"><div class="panel-head"><strong>Трассировка маршрута</strong><button>Запустить</button></div><div class="route-hops">{#each [['1','192.168.1.1','0.4 ms','LAN'],['2','10.0.0.1','1.2 ms','ISP edge'],['3','172.16.0.1','4.8 ms','Core'],['4','203.0.113.1','12.1 ms','Transit'],['5','8.8.8.8','14.1 ms','destination']] as h}<div><span class="hop">{h[0]}</span><b>{h[1]}</b><span>{h[2]}</span><small>{h[3]}</small></div>{/each}</div></section>
      <section class="panel"><div class="panel-head"><strong>Route Inspector</strong><button>Показать маршрут</button></div><div class="route-detail"><div>{#each [['Назначение','8.8.8.8/32'],['Следующий хоп','192.168.1.1'],['Интерфейс','eth0 (WAN)'],['Таблица','main'],['Метрика','100'],['Статус','active']] as r}<p><span>{r[0]}</span><b>{r[1]}</b></p>{/each}</div><ol><li>Поиск в таблице main</li><li>Проверка политик PBR</li><li>Выбор следующего хопа</li><li>Установка маршрута</li></ol></div></section>
      <section class="panel"><div class="panel-head"><strong>Flow Explorer</strong><span>metadata only · bounded</span></div><table><thead><tr><th>Время</th><th>Источник</th><th>Назначение</th><th>Протокол</th><th>Порты</th><th>Состояние</th></tr></thead><tbody>{#each flows as f}<tr>{#each f as c}<td>{c}</td>{/each}</tr>{/each}</tbody></table><div class="panel-foot">Всего: 1,482 сессии · <span class="linkish">Показать больше →</span></div></section>
    </div>

  {:else if active === 'integrations'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / INTEGRATIONS</span><h1>Интеграции</h1><p>Обнаружение и управление сторонними интерфейсами и сервисами без превращения RouterForge в магазин.</p></div><div class="head-actions"><button class="primary">Проверить всё</button></div></section>
    <div class="tabs"><button class="active">Все интеграции</button><button>Веб-интерфейсы <em>5</em></button><button>Сетевые сервисы <em>3</em></button><button>Системные утилиты <em>2</em></button><button>NFQWS2 Manager <em>CONCEPT</em></button></div>
    <div class="two-col wide-left"><div><div class="toolbar"><input placeholder="Поиск интеграций…"/><select><option>Все статусы</option></select><select><option>Все источники</option></select><select><option>Все типы</option></select></div><div class="integration-list">{#each integrations as i}<article><div class="integration-icon">◇</div><div class="integration-main"><strong>{i[0]}</strong><p>{i[1]}</p><span class="badge info">{i[2]}</span></div><div class="integration-source"><small>Источник</small><b>{i[2]}</b></div><div class="integration-status"><small>Статус</small><b>{i[3]}</b><span>{i[4]}</span></div><div class="integration-actions"><button class="primary">Открыть</button><button>Проверить</button><button>Подробнее</button></div></article>{/each}</div></div><aside class="side-stack"><section class="panel"><div class="panel-head"><strong>Об интеграциях</strong></div><p>RouterForge обнаруживает локально установленное стороннее ПО и показывает только доказанные возможности.</p><p>Интеграция не означает право на распространение или установку.</p></section><section class="panel"><div class="panel-head"><strong>Обнаружение Web UI</strong></div><button class="primary full">▶ Запустить сканирование</button><div class="list-row"><span>Диапазон</span><b>192.168.1.1/24</b></div><div class="list-row"><span>Найдено</span><b>3 интерфейса</b></div></section><section class="panel"><div class="panel-head"><strong>NFQWS2 Manager</strong><span class="badge warn">P23</span></div><p>Overview · Profiles · Config · Lists · Interfaces · Runtime · Logs · Diagnostics · Snapshots.</p><button class="full">Открыть макет менеджера</button></section></aside></div>

  {:else if active === 'developer'}
    <section class="page-head"><div><span class="eyebrow">ROUTERFORGE / DEVELOPER TOOLS</span><h1>Инструменты разработчика</h1><p>Профилирование, диагностика, валидация manifests и инженерные инструменты.</p></div><div class="head-actions"><span class="badge warn">CONCEPT / DEV ONLY</span></div></section>
    <div class="tabs"><button class="active">Обзор</button><button>Профилирование</button><button>Логи и события</button><button>Валидация манифестов</button><button>API Explorer</button><button>Отладка</button></div>
    <div class="metric-grid five">{#each [['Система','В норме'],['Python','3.11.8'],['Node.js','v20.11.1'],['Docker/Runtime','containerd 1.7'],['SDK','v0.4.2']] as s}<article class="metric"><span>{s[0]}</span><strong>{s[1]}</strong></article>{/each}</div>
    <div class="dashboard-grid two">
      <section class="panel chart"><div class="panel-head"><div><strong>Профилирование и производительность</strong><span>CPU/RAM/RPS по выбранному компоненту</span></div><button class="primary">Запустить профилирование</button></div><div class="fake-chart multi"></div><div class="perf-stats"><span>CPU avg <b>12.4%</b></span><span>RAM avg <b>54.1%</b></span><span>RPS avg <b>128</b></span></div></section>
      <section class="panel log-panel"><div class="panel-head"><div><strong>Логи и поток событий</strong><span>bounded + filters</span></div><label class="check"><input type="checkbox" checked/> автоскролл</label></div><div class="logs">{#each logLines as l}<div><span>{l[0]}</span><b>{l[1]}</b><span>{l[2]}</span><code>{l[3]}</code></div>{/each}</div></section>
      <section class="panel"><div class="panel-head"><div><strong>Валидация манифестов</strong><span>Manifest v1.1 / Registry contract</span></div></div><div class="drop-zone">⇧<strong>Перетащите manifest.yaml сюда</strong><small>или выберите файл · YAML / JSON</small><button>Выбрать файл</button></div><div class="validation">Результат валидации <span>Ошибки: 0 · Предупреждения: 0</span></div></section>
      <section class="panel"><div class="panel-head"><div><strong>API Explorer / Module Socket</strong><span>Тестирование REST и модульных Unix sockets</span></div></div><div class="api-row"><select><option>GET</option></select><input value="/api/apps"/><button class="primary">Отправить</button></div><pre class="json">{`{\n  "total": 18,\n  "apps": [\n    {"name":"entware", "status":"installed"}\n  ]\n}`}</pre></section>
    </div>
    <section class="panel flags"><div class="panel-head"><div><strong>Feature flags</strong><span>Концептуальные переключатели. Production-модули от Developer Tools не зависят.</span></div><span class="badge bad">DEV ONLY</span></div>{#each [['trace.module.socket','Дополнительная трассировка модульного прокси'],['debug.registry.provenance','Расширенные provenance-поля'],['probe.verbose','Подробные результаты Probe Engine'],['events.raw','Показывать raw Event Engine payload']] as f}<label><span><b>{f[0]}</b><small>{f[1]}</small></span><input type="checkbox"/></label>{/each}</section>
  {/if}
</div>

<style>
  :global(body) { background:#090c10; }
  .concept-page { color:var(--rf-text,#e9eef5); padding:0 0 2rem; font-size:13px; }
  button, input, select { font:inherit; }
  button { border:1px solid #303844; border-radius:4px; background:#151a20; color:#e7edf5; padding:.55rem .8rem; cursor:pointer; }
  button:hover { border-color:#4b5868; background:#1b222a; }
  button.primary { border-color:#19d45b; background:rgba(25,212,91,.1); color:#dfffea; }
  button.danger, .danger { border-color:#8f2d31; color:#ff8a8f; }
  .concept-banner { display:flex; align-items:center; justify-content:space-between; gap:1rem; margin-bottom:12px; padding:10px 12px; border:1px solid #24452f; background:linear-gradient(90deg,rgba(23,202,80,.10),rgba(9,12,16,.92)); border-radius:5px; }
  .concept-banner div { display:flex; align-items:baseline; gap:10px; flex-wrap:wrap; }
  .concept-banner strong { font-size:14px; }
  .concept-banner small { color:#7e8997; }
  .concept-kicker,.eyebrow { color:#24df68; font:600 10px/1.2 var(--font-mono,monospace); letter-spacing:.055em; }
  .concept-chip,.badge { display:inline-flex; align-items:center; border:1px solid #35404c; border-radius:999px; padding:3px 7px; font:600 10px/1 var(--font-mono,monospace); white-space:nowrap; }
  .badge.good,.concept-chip { border-color:#165c2c; background:#0e2a17; color:#26df67; }
  .badge.warn { border-color:#6e551d; background:#2b230f; color:#f4bc3a; }
  .badge.info { border-color:#245477; background:#102638; color:#65bff6; }
  .badge.bad { border-color:#713034; background:#2d1518; color:#ff747d; }
  .badge.neutral { color:#a4afbc; }
  .concept-module-nav { display:flex; gap:4px; overflow:auto; padding:0 0 10px; border-bottom:1px solid #242b33; margin-bottom:16px; }
  .concept-module-nav button { display:flex; align-items:center; gap:6px; flex:0 0 auto; border-color:transparent; background:transparent; color:#9ca8b6; border-radius:0; padding:.6rem .75rem; border-bottom:2px solid transparent; }
  .concept-module-nav button.active { color:#27e56c; border-bottom-color:#27e56c; background:rgba(39,229,108,.07); }
  .concept-module-nav b { font-weight:500; }
  .concept-toast { position:fixed; right:20px; bottom:20px; z-index:3000; max-width:420px; padding:10px 12px; border:1px solid #2f6a42; border-radius:5px; background:#101a14; box-shadow:0 18px 50px #0008; color:#c9f8d7; }
  .hero,.page-head { display:flex; align-items:flex-start; justify-content:space-between; gap:24px; margin-bottom:14px; }
  .hero { padding:20px; border:1px solid #2d3540; background:linear-gradient(130deg,#111820,#0b0e12 55%,#0d1a12); border-radius:6px; }
  h1 { margin:5px 0 4px; font-size:25px; line-height:1.1; color:#f5f7fa; }
  h2 { margin:15px 0 7px; font-size:15px; }
  p { margin:5px 0; color:#8e9aaa; line-height:1.45; }
  .hero-status { display:grid; grid-template-columns:repeat(3,auto); gap:8px; }
  .hero-status span { padding:9px 11px; border:1px solid #2d3742; border-radius:4px; background:#0d1116; color:#aeb8c4; white-space:nowrap; }
  .module-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; }
  .module-card { min-height:150px; padding:14px; text-align:left; display:flex; flex-direction:column; background:#11161c; }
  .module-card-head { display:flex; align-items:center; justify-content:space-between; gap:8px; }
  .module-card-head strong { font-size:15px; }
  .module-card p { flex:1; }
  .phase { border-radius:999px; padding:3px 7px; font:600 10px/1 var(--font-mono,monospace); background:#1a2028; color:#b0bcc8; }
  .phase.good,.phase.accent { color:#2ce76f; }.phase.warn { color:#f4bc3a; }
  .open-hint,.linkish { color:#2adf68; }
  .architecture { margin-top:12px; }
  .panel { min-width:0; border:1px solid #2a323c; border-radius:5px; background:#101419; overflow:hidden; }
  .panel-head { min-height:42px; padding:9px 11px; display:flex; align-items:center; justify-content:space-between; gap:10px; border-bottom:1px solid #29313a; }
  .panel-head > div { min-width:0; display:grid; gap:2px; }.panel-head span,.panel-head small { color:#84909f; font-size:11px; }
  .architecture-row { display:flex; align-items:center; gap:12px; padding:18px; }
  .arch-node { padding:12px 15px; border:1px solid #2b7c45; border-radius:5px; background:#102417; }.arch-node small { color:#8db59a; }
  .arch-arrow { color:#2ade68; font-size:24px; }.arch-stack { flex:1; display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:8px; }.arch-stack span,.engine-row span { padding:9px; text-align:center; border:1px solid #333c47; border-radius:4px; background:#151a21; }
  .engine-row { display:grid; grid-template-columns:repeat(4,1fr); gap:8px; padding:0 18px 18px; }.engine-row span { border-color:#534563; color:#cbb9dc; background:#1a1520; }
  .page-head p { margin-top:3px; }.head-actions { display:flex; gap:7px; align-items:center; flex-wrap:wrap; justify-content:flex-end; }
  .tabs { display:flex; gap:3px; overflow:auto; border-bottom:1px solid #2b333d; margin-bottom:12px; }
  .tabs button { flex:0 0 auto; border:0; border-bottom:2px solid transparent; background:transparent; color:#9aa6b5; border-radius:0; padding:.65rem .8rem; }
  .tabs button.active { color:#25e16a; border-bottom-color:#25e16a; background:rgba(37,225,106,.06); }.tabs em { font-style:normal; border-radius:99px; background:#202832; padding:2px 5px; font-size:10px; }
  .toolbar { display:grid; grid-template-columns:minmax(220px,1fr) repeat(3,minmax(120px,auto)); gap:7px; padding:7px; border:1px solid #2c3540; border-radius:5px; background:#11161c; margin-bottom:12px; }.toolbar.mini { grid-template-columns:1fr; border:0; margin:0; padding:7px 0; background:transparent; }
  input,select { min-width:0; border:1px solid #303945; border-radius:3px; background:#161c23; color:#dce4ed; padding:.55rem .65rem; }
  .two-col { display:grid; grid-template-columns:minmax(0,1fr) 320px; gap:12px; }.two-col.wide-left { grid-template-columns:minmax(0,1fr) 360px; }.two-col.wide-right { grid-template-columns:minmax(0,1fr) 300px; }
  .side-stack { display:grid; gap:10px; align-content:start; }.side-stack .panel { padding:0 10px 10px; }.side-stack .panel-head { margin:0 -10px 8px; }
  .app-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:9px; }.app-card { display:grid; grid-template-columns:42px 1fr; gap:10px; padding:10px; border:1px solid #2c343e; border-radius:5px; background:#12171d; }.app-icon { grid-row:1/3; display:grid; place-items:center; width:40px; height:40px; border:1px solid #394553; border-radius:6px; background:#1c242d; color:#eff5fb; font-size:22px; }.app-icon.alt{color:#28e36c}.app-card strong,.integration-main strong { display:block; font-size:14px; }.app-card small,.integration-main p { color:#8d99a8; }.app-card p { min-height:38px; }.status { font-size:11px; }.card-actions { grid-column:1/-1; display:flex; gap:6px; }.card-actions button { flex:1; }.source { display:grid; grid-template-columns:1fr auto; gap:5px; padding:8px 0; border-bottom:1px solid #29313b; }.source small { grid-column:1/-1; color:#7f8a98; }.full { width:100%; margin-top:8px; }
  .danger-soft { border-color:#6c3034; background:linear-gradient(135deg,#201315,#111418); }.danger-soft p { color:#b69093; }
  .metric-grid { display:grid; gap:9px; margin-bottom:12px; }.metric-grid.six { grid-template-columns:repeat(6,1fr); }.metric-grid.five{grid-template-columns:repeat(5,1fr)}.metric-grid.four{grid-template-columns:repeat(4,1fr)}
  .metric { min-width:0; padding:11px 12px; border:1px solid #2c3540; border-radius:5px; background:#11171d; }.metric span { color:#9ba7b5; }.metric strong { display:block; font-size:22px; margin-top:5px; }.metric small{color:#28df69}.spark{display:flex; align-items:flex-end; gap:2px; height:18px; margin-top:4px}.spark i{flex:1;background:#22c85d;height:35%;opacity:.75}.spark i:nth-child(2){height:55%}.spark i:nth-child(3){height:40%}.spark i:nth-child(4){height:75%}.spark i:nth-child(5){height:60%}.spark i:nth-child(6){height:90%}
  .health-strip { display:flex; align-items:center; gap:12px; margin-bottom:12px; padding:12px; border:1px solid #2a3a31; border-radius:5px; background:#101a14; }.health-icon{display:grid;place-items:center;width:34px;height:34px;border-radius:50%;background:#25dd66;color:#06230e;font-size:20px;font-weight:800}.health-strip div:nth-child(2){flex:1;display:grid;gap:2px}.health-strip small{color:#8fa29a}.health-modules{display:flex!important;flex-direction:row;gap:14px!important;flex-wrap:wrap}.health-modules span{color:#94a49a;font-size:11px}.health-modules b{color:#25df68}
  .dashboard-grid { display:grid; gap:10px; margin-bottom:10px; }.dashboard-grid.three{grid-template-columns:repeat(3,minmax(0,1fr))}.dashboard-grid.two{grid-template-columns:repeat(2,minmax(0,1fr))}
  .chart { min-height:210px; }.fake-chart { height:145px; margin:14px; border-left:1px solid #34404b; border-bottom:1px solid #34404b; background:linear-gradient(180deg,transparent 0 40%,rgba(42,219,103,.16)),repeating-linear-gradient(0deg,transparent 0 31px,#202831 32px),repeating-linear-gradient(90deg,transparent 0 70px,#202831 71px); clip-path:polygon(0 80%,8% 73%,13% 78%,20% 60%,26% 67%,35% 52%,43% 59%,52% 42%,62% 54%,70% 30%,79% 49%,87% 25%,94% 35%,100% 18%,100% 100%,0 100%); background-color:#163a23; }.fake-chart.blue{background-color:#153145}.fake-chart.multi{background-color:#443016;clip-path:polygon(0 45%,10% 49%,20% 42%,30% 46%,40% 38%,50% 43%,60% 36%,70% 41%,80% 33%,90% 40%,100% 31%,100% 100%,0 100%)}
  .bar-row,.rank-row,.list-row,.service-row { display:grid; align-items:center; gap:8px; padding:7px 10px; border-bottom:1px solid #252d36; }.bar-row{grid-template-columns:90px 1fr 45px}.bar-row div,.rank-row div{height:8px;border-radius:3px;background:#252d36;overflow:hidden}.bar-row i,.rank-row i,.usage i{display:block;height:100%;background:#37df77}.rank-row{grid-template-columns:110px 90px 1fr}.list-row{grid-template-columns:1fr auto auto}.service-row{grid-template-columns:1fr 1fr auto}.service-row div{display:flex;gap:4px}.service-row button{padding:.25rem .45rem}.good-text{color:#27df68}.bad-text{color:#ff676e}
  table { width:100%; border-collapse:collapse; font-size:11px; }th,td{padding:6px 8px;border-bottom:1px solid #28313a;text-align:left}th{color:#8f9bab;font-weight:500}.mono,code,pre{font-family:var(--font-mono,monospace)}
  .resolver-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:9px; margin-bottom:12px; }.resolver-card{display:grid;grid-template-columns:1fr auto;gap:7px;padding:10px;border:1px solid #2d3741;border-radius:5px;background:#11171d}.resolver-card>div:first-child{min-width:0;display:grid;grid-template-columns:auto auto 1fr;align-items:center;gap:6px}.resolver-card small{grid-column:2/-1;color:#8793a1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.dot{width:7px;height:7px;border-radius:50%;background:#24de67}.resolver-actions{grid-column:1/-1;display:flex;gap:6px;border-top:1px solid #27303a;padding-top:7px}.resolver-actions button{flex:1}
  .policy-preview{padding-bottom:10px}.policy-flow{display:flex;align-items:center;justify-content:center;gap:10px;padding:16px;flex-wrap:wrap}.policy-flow span{padding:8px 12px;border:1px solid #2f7645;border-radius:4px;background:#102317}.policy-flow b{color:#26df69}.policy-columns{display:grid;grid-template-columns:repeat(3,1fr);gap:8px;padding:0 10px}.policy-columns>div{padding:10px;border:1px solid #2b343e;background:#12171d;border-radius:4px}
  .file-manager{margin-bottom:10px}.file-body{display:grid;grid-template-columns:240px 1fr;min-height:230px}.file-body aside{border-right:1px solid #29313a;padding:8px}.file-body aside div{padding:5px 8px;color:#94a0ae}.file-body aside div.active{background:#123d22;color:#a9f4be}.file-row{display:grid;grid-template-columns:1fr 100px 130px;gap:8px;padding:7px 10px;border-bottom:1px solid #27303a}.terminal pre{margin:0;padding:10px;color:#93e9aa;background:#080b0e;min-height:90px}.mini-stat{display:inline-flex;flex-direction:column;margin:12px 10px;padding:9px 11px;border:1px solid #2b343f;border-radius:4px}.mini-stat strong{font-size:20px;color:#2ee872}
  .logs{padding:8px;background:#0a0e12}.logs>div{display:grid;grid-template-columns:78px 50px 60px 1fr;gap:8px;padding:4px 2px;font:11px/1.25 var(--font-mono,monospace)}.logs b{color:#2adf68}.logs code{color:#c8d1db}.activity-row{display:flex;gap:8px;align-items:center;padding:9px;border-bottom:1px solid #29313a}.activity-row div{flex:1;display:flex;justify-content:space-between;gap:8px}.activity-row small{color:#8692a0}.vault-row{display:grid;grid-template-columns:auto 1fr auto auto auto;align-items:center;gap:7px;padding:7px 9px;border-bottom:1px solid #28313a}.usage{height:12px;margin:12px;border-radius:3px;background:#252e37;overflow:hidden}.bundle{margin:10px;padding:9px;border:1px solid #2d3742;background:#0e1217}.check{display:flex;align-items:center;gap:6px;color:#9da9b7}.danger-action{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:10px;border-bottom:1px solid #4a292c}
  .probe-form{display:grid;grid-template-columns:1fr 190px 150px;gap:7px;padding:10px}.probe-row{display:grid;grid-template-columns:120px 100px 1fr auto;gap:8px;padding:8px 10px;border-top:1px solid #28313a}.route-hops>div{display:grid;grid-template-columns:30px 1fr 90px 120px;gap:8px;align-items:center;padding:7px 10px;border-bottom:1px solid #28313a}.hop{display:grid;place-items:center;width:22px;height:22px;border-radius:50%;background:#168c40;color:#fff}.route-detail{display:grid;grid-template-columns:1fr 1fr;gap:10px;padding:10px}.route-detail p{display:flex;justify-content:space-between;border-bottom:1px solid #29313a;padding-bottom:6px}.route-detail ol{margin:0;padding:0;list-style:none;counter-reset:route}.route-detail li{counter-increment:route;padding:9px 8px 9px 38px;position:relative}.route-detail li:before{content:counter(route);position:absolute;left:5px;top:5px;width:24px;height:24px;border-radius:50%;display:grid;place-items:center;background:#15883e}.panel-foot{padding:8px 10px;text-align:right;color:#8a96a5}
  .integration-list{display:grid;gap:8px}.integration-list article{display:grid;grid-template-columns:58px minmax(0,1.8fr) minmax(120px,.7fr) minmax(170px,.9fr) 150px;gap:10px;align-items:center;padding:10px;border:1px solid #2c3540;border-radius:5px;background:#11171d}.integration-icon{display:grid;place-items:center;width:52px;height:52px;border:1px solid #394554;border-radius:7px;background:#1b222a;font-size:24px;color:#32e376}.integration-main p{margin:2px 0 6px}.integration-source,.integration-status{display:grid;gap:4px;border-left:1px solid #28313a;padding-left:10px}.integration-source small,.integration-status small{color:#85919f}.integration-status span{font-size:10px;color:#798493}.integration-actions{display:grid;gap:4px}.integration-actions button{padding:.42rem .55rem}
  .perf-stats{display:flex;gap:20px;padding:0 12px 12px;color:#8995a4}.perf-stats b{color:#eef3f8}.drop-zone{margin:10px;padding:24px;border:1px dashed #52606f;border-radius:5px;display:grid;place-items:center;gap:7px;color:#8e9aa8}.drop-zone strong{color:#eef3f8}.validation{margin:10px;padding:10px;border:1px solid #2e3843;display:flex;justify-content:space-between}.api-row{display:grid;grid-template-columns:85px 1fr 110px;gap:6px;padding:10px}.json{margin:0 10px 10px;padding:10px;background:#090d11;border:1px solid #27303a;color:#b8d7f0;min-height:145px}.flags label{display:flex;align-items:center;justify-content:space-between;padding:9px 10px;border-bottom:1px solid #28313a}.flags label span{display:grid;gap:3px}.flags label small{color:#85919f}
  @media (max-width:1350px){.module-grid{grid-template-columns:repeat(3,1fr)}.metric-grid.six{grid-template-columns:repeat(3,1fr)}.dashboard-grid.three{grid-template-columns:repeat(2,1fr)}.resolver-grid{grid-template-columns:repeat(2,1fr)}.integration-list article{grid-template-columns:52px 1fr 130px 150px}.integration-actions{grid-column:2/-1;grid-template-columns:repeat(3,1fr)}}
  @media (max-width:1000px){.two-col,.two-col.wide-left,.two-col.wide-right{grid-template-columns:1fr}.module-grid{grid-template-columns:repeat(2,1fr)}.app-grid{grid-template-columns:1fr}.metric-grid.five,.metric-grid.four{grid-template-columns:repeat(2,1fr)}.dashboard-grid.two,.dashboard-grid.three{grid-template-columns:1fr}.arch-stack{grid-template-columns:repeat(2,1fr)}.engine-row{grid-template-columns:repeat(2,1fr)}}
  @media (max-width:720px){.hero,.page-head{display:grid}.hero-status{grid-template-columns:1fr}.module-grid,.resolver-grid,.metric-grid.six,.metric-grid.five,.metric-grid.four{grid-template-columns:1fr}.toolbar{grid-template-columns:1fr}.architecture-row{display:grid}.arch-arrow{transform:rotate(90deg);text-align:center}.file-body{grid-template-columns:1fr}.file-body aside{display:none}.probe-form{grid-template-columns:1fr}.policy-columns{grid-template-columns:1fr}.integration-list article{grid-template-columns:45px 1fr}.integration-source,.integration-status,.integration-actions{grid-column:1/-1;border-left:0;padding-left:0}.route-detail{grid-template-columns:1fr}.logs>div{grid-template-columns:65px 45px 55px 1fr;overflow:auto}}
</style>
