<script>
  // The outer shipped HTML shell remains a compatibility scaffold while its
  // visibility and status are Svelte-owned. No network/mutation on mount.
  let {page = 'dns', editing = null, config = {}, mode = 'basic', active = false} = $props();
  const source = $derived((config.upstreams || []).find(u => u.id === editing));
  const selectedLabel = $derived(source ? String(source.server_name || source.address || source.id) : 'DNS не выбран');
  const nativeInUse = $derived((config.upstreams || []).some(u =>
    u.transport === 'native' && (config.rules || []).some(r => r.upstream === u.id)));
  function project(node, spec) {
    const host = document.getElementById(spec.id);
    if (!host) throw Error('DNS_LITE_SHELL_TARGET_MISSING: ' + spec.id);
    function refresh(next) {
      if ('hidden' in next) host.hidden = Boolean(next.hidden);
      if ('text' in next) host.textContent = String(next.text);
      if ('mode' in next) host.dataset.dnsMode = next.mode;
      if ('value' in next) host.value = next.value;
    }
    refresh(spec);
    return {update:refresh};
  }
</script>

<span hidden use:project={{id:'pageDns',hidden:page !== 'dns'}}></span>
<span hidden use:project={{id:'pageLogs',hidden:page !== 'logs'}}></span>
<span hidden use:project={{id:'p21Page',hidden:page !== 'devices'}}></span>
<span hidden use:project={{id:'nativeWarning',hidden:!nativeInUse}}></span>
<span hidden use:project={{id:'editorEmpty',hidden:Boolean(editing)}}></span>
<span hidden use:project={{id:'selectedDNSBadge',text:selectedLabel}}></span>
<span hidden use:project={{id:'p21ActiveBadge',hidden:!active}}></span>
<span hidden use:project={{id:'dnsLiteApp',mode}}></span>
<span hidden use:project={{id:'p21Mode',value:mode}}></span>
