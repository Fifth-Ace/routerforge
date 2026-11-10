// Pure presentation of explicit configuration-evaluation evidence.
// Never sends traffic or reports a successful network-level E2E.
export function presentP22Route(result, mac='', devices=[], english=false) {
  if (!result || result.scope !== 'CONFIGURATION_ONLY') return null;
  const t=(ru,en)=>english?en:ru;
  const decision=result.decision||{};
  const chain=Array.isArray(result.allowed_chain)?result.allowed_chain:[];
  const known=(Array.isArray(devices)?devices:[]).find(device=>device.mac===String(mac||'').toLowerCase());
  const identity=mac?(known?.name||mac):t('Общие правила','Common rules');
  const layer=({device_domain:t('Правило устройства','Device rule'),device:t('DNS устройства','Device DNS'),device_group:t('Группа устройств','Device group'),common_domain:t('Общее доменное правило','Common domain rule'),default:t('DNS по умолчанию','Default DNS'),local_namespace:t('Локальный DNS Keenetic','Keenetic local DNS')})[decision.layer]||decision.layer||t('Правило не определено','No rule matched');
  const primary=chain[0];
  const target=primary?(primary.server_name||primary.address||primary.id||'—'):(decision.layer==='local_namespace'?t('Локальный DNS Keenetic','Keenetic local DNS'):'—');
  const reserve=chain.slice(1).map(item=>item.server_name||item.address||item.id).join(' → ');
  return {
    steps:[t('Устройство: ','Device: ')+identity,t('Правило: ','Rule: ')+layer,t('DNS: ','DNS: ')+target,...(reserve?[t('Резерв: ','Fallback: ')+reserve]:[])],
    note:t('Расчёт по конфигурации, не подтверждённый сетевой E2E.','Configuration evaluation, not a verified network E2E test.')
  };
}
