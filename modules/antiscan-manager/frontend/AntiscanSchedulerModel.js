// Canonical Antiscan cron task model; automatic retry_load_geo is never mutated.
export const schedulerTaskLabels = {
  read_candidates: ['Обработка кандидатов', 'Проверка накопленных IP-кандидатов и перенос /24 при достижении порога.'],
  read_ndm_ipsets: ['Импорт блокировок Keenetic', 'Перенос системных lockout IP из Keenetic в runtime Antiscan.'],
  save_ipsets: ['Сохранение ipset', 'Периодический экспорт runtime-наборов согласно SAVE_IPSETS.'],
  'update_ipsets geo': ['Обновление Geo', 'Плановое обновление списков подсетей настроенных стран.'],
  retry_load_geo: ['Повтор Geo после ошибки', 'Временная аварийная задача, которую создаёт и удаляет сам upstream Antiscan.']
};

export function managedScheduleTasks(rows) {
  return rows.filter(item => !item.automatic).map(item=>({
    task:item.task,enabled:Boolean(item.enabled),schedule:String(item.schedule||'').trim().replace(/\s+/g,' ')
  }));
}

