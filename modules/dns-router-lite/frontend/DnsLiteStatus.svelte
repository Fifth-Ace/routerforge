<script>
  // Status text belongs to Svelte. Portals keep the shipped IDs, CSS selectors
  // and accessibility roles stable; no API calls or mutations occur here.
  let {view = {}} = $props();
  function statusTarget(node, spec) {
    const host = document.getElementById(spec.id);
    if (!host) throw Error('DNS_LITE_STATUS_TARGET_MISSING: ' + spec.id);
    host.replaceChildren(node);
    function update(next) {
      if ('tone' in next) host.dataset.tone = next.tone || '';
      if ('panelState' in next) {
        const panel = document.getElementById('systemGatePanel');
        if (panel) panel.dataset.state = next.panelState || 'stock';
      }
    }
    update(spec);
    return {update, destroy() { if (node.parentNode === host) node.remove(); }};
  }
</script>

<span use:statusTarget={{id:'serviceBadge',tone:view.serviceTone}} style="display:contents">{view.serviceBadge}</span>
<span use:statusTarget={{id:'healthUpdated'}} style="display:contents">{view.healthUpdated}</span>
<span use:statusTarget={{id:'healthGate'}} style="display:contents">{view.healthGate}</span>
<span use:statusTarget={{id:'healthServers'}} style="display:contents">{view.healthServers}</span>
<span use:statusTarget={{id:'healthRequests'}} style="display:contents">{view.healthRequests}</span>
<span use:statusTarget={{id:'healthP95'}} style="display:contents">{view.healthP95}</span>
<span use:statusTarget={{id:'nativeDNSHealth'}} style="display:contents">{view.nativeDNSHealth}</span>
<span use:statusTarget={{id:'gateCompactBadge',tone:view.gateTone}} style="display:contents">{view.gateBadge}</span>
<span use:statusTarget={{id:'gateBadge',tone:view.gateTone,panelState:view.gatePanelState}} style="display:contents">{view.gateBadge}</span>
<span use:statusTarget={{id:'listenerSummary'}} style="display:contents">{view.listenerSummary}</span>
<span use:statusTarget={{id:'connectionSummary'}} style="display:contents">{view.connectionSummary}</span>
<span use:statusTarget={{id:'gateAdvice'}} style="display:contents">{view.gateAdvice}</span>
<span use:statusTarget={{id:'runStatus'}} style="display:contents">{view.runStatus}</span>
<span use:statusTarget={{id:'routeAdvice',tone:view.routeTone}} style="display:contents">{view.routeAdvice}</span>
<span use:statusTarget={{id:'clientState'}} style="display:contents">{view.clientState}</span>
<span use:statusTarget={{id:'liveBadge',tone:view.liveTone}} style="display:contents">{view.liveBadge}</span>
<span use:statusTarget={{id:'logUpdated'}} style="display:contents">{view.logUpdated}</span>
<span use:statusTarget={{id:'logGuide',tone:view.routeTone}} style="display:contents">{view.routeAdvice}</span>
<span use:statusTarget={{id:'metricRequests'}} style="display:contents">{view.metricRequests}</span>
<span use:statusTarget={{id:'metricCache'}} style="display:contents">{view.metricCache}</span>
<span use:statusTarget={{id:'metricErrors'}} style="display:contents">{view.metricErrors}</span>
<span use:statusTarget={{id:'metricRunning'}} style="display:contents">{view.metricRunning}</span>
<span use:statusTarget={{id:'logCounter'}} style="display:contents">{view.logCounter}</span>
<span use:statusTarget={{id:'logVisible'}} style="display:contents">{view.logVisible}</span>
