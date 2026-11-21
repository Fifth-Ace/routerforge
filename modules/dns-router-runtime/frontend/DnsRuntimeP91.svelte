<script>
  import { rfUi } from './rf-localization.js';
  import {createP91Flow} from './DnsRuntimeP91Logic.js';
  import {p91Runtime} from './DnsRuntimeCommands.js';
  let transport=$state('doh');
  let endpoint=$state('');
  let ip=$state('');
  let sni=$state('');
  let busy=$state(false);
  let message=$state(rfUi('Адрес ещё не проверялся.',"The address has not been tested yet."));
  let error=$state('');
  let verified=$state(null);
  let autoIP='';
  const bridge=()=>p91Runtime;
  const fields=()=>({transport,endpoint,ip,sni});
  const flow=createP91Flow({
    lookup:body=>bridge().lookup(body),
    readDraft:()=>bridge().readDraft(),
    commit:(action,payload)=>bridge().commit(action,payload),
    notify:text=>{message=text;bridge().notify(text);},
    refresh:()=>bridge().refresh(),
    getFields:fields,
    signal:state=>{
      verified=state.verified;
      if(state.verified?.ip && !ip){ip=state.verified.ip;autoIP=ip;}
    }
  });
  function edit(origin){
    // A discovered IP is tied to the previous endpoint. Preserve a manually entered IP.
    if(origin!=='ip'&&autoIP&&ip===autoIP)ip='';
    autoIP='';flow.invalidate();error='';message=rfUi('Адрес изменён. Выполни проверку заново.',"The address changed. Run the test again.");
  }
  async function inspect(){
    if(busy)return;
    busy=true;error='';message=rfUi('Ищу IP и проверяю DNS…',"Resolving IPs and checking DNS…");
    try{await flow.inspect();}
    catch(e){error=String(e?.message||e);message=rfUi('Проверка не подтверждена: ',"Verification incomplete:")+error;}
    finally{busy=false;}
  }
  function store(){
    if(busy)return;
    error='';
    try{flow.store();transport='doh';endpoint='';ip='';sni='';autoIP='';message=rfUi('DNS добавлен в черновик. Сохрани настройки отдельной кнопкой.',"DNS added to the draft. Save settings separately.");}
    catch(e){error=String(e?.message||e);message=error;}
  }
  let label=$derived(transport==='udp'||transport==='doq'?rfUi('Найти IP',"Find IP"):rfUi('Получить IP и сертификаты',"Fetch IP addresses and certificates"));
  let placeholder=$derived(transport==='doh'?'https://dns.example.com/dns-query':transport==='udp'?'1.1.1.1':'dns.example.com');
</script>
<details class="dr-p90-manual" data-p91-native="true">
  <summary>{rfUi("＋ Добавить DNS-сервер","＋ Add DNS server")}</summary>
  <div class="dr-p90-manual-body">
    <div class="dr-p91-form-row">
      <label class="dr-simple-field"><span>{rfUi("Тип DNS","DNS type")}</span><select id="drP90Transport" bind:value={transport} onchange={()=>edit('transport')}><option value="doh">DoH (HTTPS)</option><option value="dot">DoT (TLS)</option><option value="udp">{rfUi("UDP (обычный)","UDP (unencrypted)")}</option><option value="doq">DoQ (QUIC)</option></select></label>
      <label class="dr-simple-field"><span>{rfUi("Адрес DNS","DNS address")}</span><input id="drP90Endpoint" type="text" maxlength="300" placeholder={placeholder} bind:value={endpoint} oninput={()=>edit('endpoint')} autocomplete="off" spellcheck="false" /></label>
    </div>
    <p class="dr-simple-help">{rfUi("Вставь адрес сервера — IP и сертификат найдём только после явного запроса.","Enter the server address. IP and certificate details are retrieved only when requested.")}</p>
    <div class="dr-simple-actions"><button id="drP90Inspect" type="button" class="button" disabled={busy} onclick={inspect}>{label}</button><button id="drP90Store" type="button" class="button primary" disabled={busy||!verified} onclick={store}>{rfUi("Добавить DNS","Add DNS")}</button></div>
    <p class="dr-simple-state" id="drP90Status" role="status" aria-live="polite">{message}</p>
    <div class="dr-p91-found" id="drP91Found" hidden={!verified}>
      <div><strong>{rfUi("Найденные IP","Discovered IPs")}</strong><span id="drP91IPs">{verified?.ips?.join(', ')||''}</span></div>
      <div id="drP91CertRow" hidden={!verified?.encrypted}><strong>{rfUi("Сертификат","Certificate")}</strong><span id="drP91Cert">{verified?.encrypted?'✓ '+String(verified.certificate?.subject||'')+' · до '+String(verified.certificate?.not_after||''):''}</span></div>
    </div>
    <details class="dr-p91-advanced" id="drP91Advanced">
      <summary>{rfUi("Дополнительно: IP, SNI и сертификат","Advanced: IP, SNI, and certificate")}</summary>
      <label class="dr-simple-field"><span>{rfUi("IP подключения (необязательно)","Connection IP (optional)")}</span><input id="drP90IP" type="text" maxlength="45" placeholder={rfUi("Определится автоматически","Detected automatically")} bind:value={ip} oninput={()=>edit('ip')} autocomplete="off" spellcheck="false" /></label>
      <label class="dr-simple-field"><span>{rfUi("Имя сертификата (SNI, необязательно)","Certificate name (SNI, optional)")}</span><input id="drP91SNI" type="text" maxlength="253" placeholder={rfUi("Совпадает с адресом сервера","Matches server address")} bind:value={sni} oninput={()=>edit('sni')} autocomplete="off" spellcheck="false" /></label>
      <details class="dr-p91-advanced" id="drP91PEMWrap" hidden={!verified?.encrypted}><summary>{rfUi("Публичный сертификат PEM","Public PEM certificate")}</summary><pre id="drP91PEM">{verified?.encrypted?verified.certificate?.pem||'':''}</pre></details>
    </details>
    {#if error}<p role="alert">{error}</p>{/if}
  </div>
</details>
