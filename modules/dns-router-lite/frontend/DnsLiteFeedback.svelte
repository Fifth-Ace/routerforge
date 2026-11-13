<script>
  let {text = 'Загружаем настройки…',tone = ''} = $props();
  function portal(node, value) {
    const target = document.getElementById('feedback');
    if (!target) throw Error('DNS_LITE_FEEDBACK_HOST_MISSING');
    // The existing host is only a presentation slot; Svelte owns the text.
    target.replaceChildren(node);
    // Preserve the legacy host status contract for screen readers and tests.
    // Svelte owns both text and tone; the outer HTML does not run a handler.
    target.dataset.tone=String(value||'');
    return {
      update(next){target.dataset.tone=String(next||'');},
      destroy(){if(node.parentNode===target)node.remove();delete target.dataset.tone;}
    };
  }
</script>
<span use:portal={tone} data-tone={tone}>{text}</span>
