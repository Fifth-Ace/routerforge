/** UI-only evidence. Polling a trace endpoint is not proof of live DNS traffic. */
export function dnsLiveBadge(status, paused = false) {
  const active = Boolean(status?.active);
  if (paused) return {text:'ПАУЗА · снимок журнала', tone:active?'good':status?'bad':''};
  if (!status) return {text:'LIVE · ожидание', tone:''};
  if (!active) return {text:'LIVE · DNS остановлен', tone:'bad'};
  if (Number(status.requests || 0) > 0) return {text:'LIVE · запросы поступают', tone:'good'};
  return {text:'LIVE · нет запросов', tone:'good'};
}
