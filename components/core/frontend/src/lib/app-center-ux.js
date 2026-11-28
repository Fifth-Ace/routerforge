// Presentation only: no changes to package admission, execution or registry.
export function displayVersion(value) {
  const v=String(value || '').trim().replace(/^v+(?=\d)/i,'');
  return v ? `v${v}` : '—';
}
export function updateIndicatorState({running=false,launching=false,slot=null,error='',now=Date.now(),dismissed=''}={}) {
  if (running || launching) return 'active';
  const key=slot?.plan?.id || (error?'launch-error':'');
  if (key && dismissed===key) return '';
  if (error || ['failed','cancelled','recovery_required'].includes(slot?.state)) return 'error';
  const ended=Number(slot?.journal?.finished_at || 0)*1000;
  return slot?.state==='completed' && ended>0 && now>=ended && now-ended<5000 ? 'success' : '';
}
// Translations of known catalog entries from the bundled registry/ecosystem.
// Unrecognized entries retain their original description; no inferred features.
const ruDescriptions={
 'adguardhome-keenetic':'DNS-фильтрация для всей сети. На Keenetic можно использовать пакет adguardhome-go из Entware.',
 'bypass-keenetic':'Выборочная маршрутизация через VPN, Shadowsocks или Tor с управлением через Telegram-бота.',
 'chur-keenetic':'Веб-менеджер VPN-интерфейсов Keenetic и Entware с опциональной поддержкой AmneziaWG.',
 'keen-pbr':'Демон маршрутизации по политикам для Keenetic и OpenWrt. Полная версия включает REST API и веб-интерфейс.',
 'keenetic-entware-extras':'Географическая маршрутизация, интеграция SmartDNS, перенаправление DNS, сетевая диагностика и опциональная веб-панель.',
 'keenetic-sing-box-ui':'Веб-интерфейс sing-box для ARM64 с выборочной маршрутизацией TProxy/REDIRECT, диагностикой и авторизацией.',
 kvas:'Выборочная маршрутизация трафика по доменам через VPN с использованием ipset, dnsmasq, dnscrypt или AdGuard Home.',
 skeen:'Набор инструментов TProxy и Redirect на основе sing-box для Keenetic и Netcraze со встроенной веб-панелью.',
 'traffic-via-vpn':'Набор скриптов Entware для перенаправления выбранных доменов, IP-адресов и подсетей через заданный VPN-интерфейс.',
 xkeen:'Установка и обновление Xray на Keenetic, управление базами GeoIP и GeoSite.',
 'usque-keenetic':'Развёртывание SOCKS5 Usque на Keenetic/Entware с определением архитектуры, настройкой службы и опциональной маршрутизацией redsocks.',
 'keenetic-singbox':'Выборочная маршрутизация sing-box для KeeneticOS 5+ с каталогом сервисов, резервированием, kill switch и страницей состояния.',
 'keenetic-cloudflared':'Развёртывание Cloudflare Tunnel на Keenetic/Netcraze с управлением службой и бинарниками ARM64/MIPSEL.',
 'netbird-keenetic':'Развёртывание NetBird на Keenetic/Entware с настройкой firewall, rp_filter и интеграцией NDM.',
 keen2ygg:'Развёртывание IPv6-сети Yggdrasil на Keenetic с интеграцией radvd и ip6tables.',
 'keenetic-zapret2-manager':'Менеджер Zapret2 для Keenetic/Entware с веб-интерфейсом, профилями DPI, blockcheck, IPSET и резервными копиями.',
 'keenetic-aria2-manager':'Менеджер aria2 для Keenetic с интерфейсом AriaNg, уведомлениями Telegram, резервными копиями и интерактивной настройкой.',
 'keenetic-firewall':'Набор инструментов firewall и мониторинга Entware для Keenetic: списки блокировки IPSET, защита VPN от перебора паролей и HTML-панели.',
 'keenetic-traffic-via-vpn':'Выборочная маршрутизация доменов и IP-адресов в существующий VPN-туннель Keenetic через скрипты Entware и списки маршрутов.',
 'nfqws-keenetic':'Классический пакет nfqws для Keenetic/Netcraze и Entware. Для новых возможностей разработчик рекомендует nfqws2.',
 'nfqws2-keenetic':'Пакет nfqws2 для Keenetic/Netcraze и Entware со стратегиями Lua, обработкой трафика NFQUEUE и raw sockets.',
 'nfqws-keenetic-web':'Веб-интерфейс nfqws и nfqws2 для Keenetic/Netcraze с Entware.',
 'keenetic-policy-ui':'Веб-интерфейс Entware для назначения политик подключения и DNS-профилей отдельным устройствам Keenetic/Netcraze.',
 'entware-manager':'Веб-панель управления пакетами, службами, процессами, журналами и файлами Entware, с мониторингом, SMART и терминалом.',
 'zapret-gui':'Веб-интерфейс nfqws2/zapret2, вспомогательных VPN-инструментов и маршрутизации для Keenetic/Entware и OpenWrt.',
 razvilka:'Локальная панель Keenetic/Netcraze для оценки доступных способов сетевой совместимости и подготовки маршрутов выбранных сервисов и устройств.',
 'keenetic-mcp':'MCP/HTTP-сервер мониторинга и управления Keenetic через RCI с предварительным просмотром изменений, правилами наблюдения и резервными копиями.',
 antigoblin:'Панель маршрутизации XKeen/Xray и sing-box для Keenetic/Entware с политиками устройств, подписками и веб-интерфейсом.',
 'xkeen-panel':'Веб-панель XKeen для Keenetic с маршрутизацией, подписками и обновлениями по релизам.',
 keensnap:'Резервное копирование KeeneticOS/Entware с отправкой в Telegram, Google Drive и на подключённый накопитель.',
 sms2gram:'Автоматизация SMS модема Keenetic: Telegram, VK, ntfy, пересылка и управляемые удалённые команды.',
 web4static:'Веб-интерфейс редактирования настроек маршрутизации, DPI, VPN и сетевых инструментов Keenetic/Entware.',
 magitrickle:'Пакет выборочной маршрутизации по доменам для Keenetic/Entware и OpenWrt.',
 'susanin-keenetic':'Адаптивная выборочная VPN-маршрутизация для Keenetic/Entware, определяющая проблемные назначения по поведению соединений.',
 'trusttunnel-keenetic':'Интеграция клиента TrustTunnel для Keenetic/Entware в режимах SOCKS5 или TUN с интерфейсными хуками Keenetic.',
 'tg-ws-proxy-go':'Пакет Telegram MTProto WebSocket-прокси для Keenetic/Entware на Go.',
 'tg-ws-proxy-rs':'Статический Telegram MTProto WebSocket-мост на Rust для Entware/OpenWrt с установщиком по архитектуре и проверкой контрольных сумм релиза.',
 'aiway-manager':'Панель Keenetic для AIWAY DNS/SNI-маршрутизации, VPS-профилей и управления локальным процессом.',
 bird4static:'Инструмент маршрутизации BIRD для Keenetic/Entware с поддержкой antifilter.download, antifilter.network, re:filter и пользовательских источников маршрутов.',
 ipset4static:'Маршрутизация по доменам через ipset и iptables для Keenetic/Entware, самостоятельно или вместе с Bird4Static.',
 'trusttunnel-keenetic-native':'Расширенная интеграция TrustTunnel для Keenetic с нативным режимом OpkgTun и статистикой трафика в панели.',
 'tg-ws-keenetic':'Telegram MTProto WebSocket-прокси на Rust для Keenetic/Entware со встроенной лёгкой веб-панелью.',
 'wireguard-dpi-bypass':'Вспомогательный инструмент Keenetic для восстановления зависших рукопожатий WireGuard/AmneziaWG через проверку и смену локального порта.',
 'magitrickle-badigit':'Вариант MagiTrickle с redir/TPROXY, подписками на правила, перехватом DNS и расширенной маршрутизацией.',
 'magitrickle-larin':'Расширенный MagiTrickle с оптимизированным поиском правил, массовыми операциями, обнаружением конфликтов и обновлениями.',
 'xkeen-ui-fan92':'Однобинарный веб-интерфейс XKeen: настройки, журналы, команды, переключение Xray/Mihomo и опциональное управление AmneziaWG.',
 'dropweb-xkeen':'Развёртывание выборочной маршрутизации Mihomo/XKeen с обновлением подписок, watchdog и локальной веб-панелью.',
 'wdtt-server-entware':'Развёртывание сервера WDTT для Entware с бинарниками по архитектуре, init-службой и интеграцией firewall/NAT.',
 'netcraze-giga-awg3':'Интеграция AmneziaWG v3 на уровне ядра и модулей для Keenetic/Netcraze Giga KN-1012 / MT7981.',
 b4:'Платформа маршрутизации и сетевой совместимости для Keenetic/Entware с наборами правил, прозрачными режимами и транспортом Telegram.',
 'hydra-bridge':'HTTP API с авторизацией для управления настройками, диагностикой, резервными копиями и службами HydraRoute Neo.',
 'ssclash-go':'Самостоятельная панель управления Mihomo для Keenetic/Entware со встроенным интерфейсом, выборочной маршрутизацией и netfilter.',
 broray:'Клиент Xray и менеджер сетевых политик Keenetic с подписками, маршрутизацией, DNS-over-TLS и веб-интерфейсом с авторизацией.',
 'keenetic-auto-setup':'Комплекс развёртывания Mihomo и MagiTrickle на Keenetic/Entware с проверками памяти и диска, маршрутизацией, watchdog и диагностикой.',
 'qeli-keenetic':'Развёртывание клиента Qeli на Keenetic/Entware в режиме TUN-шлюза с опциональной раздельной маршрутизацией и OpkgTun.',
 'xkeen-ui-umarcheh':'Веб-интерфейс XKeen/Xray/Mihomo с подписками, PTY, файловым менеджером, DAT-инструментами, диагностикой и опциональным Android-клиентом.',
 'keen-pbr-headless':'Демон маршрутизации по политикам keen-pbr для Keenetic/Netcraze без API-сервера и веб-интерфейса.',
 wayhop:'Веб-панель роутера для VPN и прокси, резервирования и выборочной маршрутизации с применением и откатом настроек.',
 'qwdtt-keenetic':'Сервер защищённого туннеля для Keenetic с транспортами WireGuard/Raw-IP, профилями, firewall и веб-управлением.',
 'detour-keenetic':'Самостоятельная панель маршрутизации и сетевой совместимости для Keenetic/Entware с sing-box, tpws, цепочками WARP и картами маршрутов.',
 'keenetic-xray-vpn':'Комплекс Xray VPN для Keenetic/Entware с веб-мастером настройки, обновлением, переустановкой и обратимым удалением.',
 'keen-manager':'Единая панель Keenetic для AmneziaWG, Xray и nfqws2 с проверками доступности, резервными цепочками и обратимой настройкой.',
 'keenetic-doq':'Переадресатор DNS-over-QUIC как дополнительный системный DNS-сервер Keenetic без замены порта 53.',
 'keenetic-vpn-xor':'Панель маршрутизации OpenVPN с XOR/scramble для Keenetic/Entware, поддерживающая несколько профилей и веб-управление.',
 'keenetic-xray-auto':'Установщик Xray/VLESS с резервированием для Keenetic, автоматическим выбором Full Go или Minimal Go, восстановлением и диагностикой.',
 'xkeen-smartroute':'Выборочная маршрутизация по доменам, устройствам и IP для XKeen/Xray с подписками, группами серверов и отдельной веб-панелью.'
};
const exactTranslations={
 'Country/region-aware policy routing for Keenetic using GeoIP subnet and domain data.':'Маршрутизация по странам и регионам для Keenetic с данными GeoIP о подсетях и доменах.',
 'Regional SmartDNS configuration with selectable local/international providers and optional tunnel binding.':'Региональная настройка SmartDNS с выбором локальных и международных провайдеров и опциональной привязкой к туннелю.',
 'LAN DNS interception/redirect helper for local SmartDNS, AdGuard Home or Unbound on Keenetic.':'Перехват и перенаправление DNS в локальной сети на SmartDNS, AdGuard Home или Unbound для Keenetic.',
 'Network diagnostic toolkit for egress, DNS, TLS MITM, IPv6 leaks, reachability and interface comparison.':'Диагностика исходящего трафика, DNS, TLS MITM, утечек IPv6, доступности и сравнение интерфейсов.',
 'Web dashboard and configuration editor for the Keenetic Entware Extras package family.':'Веб-панель и редактор конфигурации семейства пакетов Keenetic Entware Extras.'
};
export function integrationDescription(item,locale) {
  const original=String(item?.description || '');
  if (locale!=='ru' || item?.kind!=='integration' || !original || /[А-Яа-яЁё]/.test(original)) return original;
  if (item.source && !['community','project-official','routerforge-official'].includes(item.source)) return original;
  if (Object.hasOwn(exactTranslations,original)) return exactTranslations[original];
  return Object.hasOwn(ruDescriptions,item.id) ? ruDescriptions[item.id] : original;
}
