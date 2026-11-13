<script>
  let {text = 'Загружаем настройки…',tone = ''} = $props();
  function portal(node) {
    const target = document.getElementById('feedback');
    if (!target) throw Error('DNS_LITE_FEEDBACK_HOST_MISSING');
    // The existing host is only a presentation slot; Svelte owns the text.
    target.replaceChildren(node);
    return {destroy(){if(node.parentNode===target)node.remove();}};
  }
</script>
<span use:portal data-tone={tone}>{text}</span>
