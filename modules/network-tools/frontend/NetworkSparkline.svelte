<script>
  let { values = [], bad = false } = $props();
  let points = $derived.by(() => {
    const numbers = (Array.isArray(values) ? values : []).filter(n => Number.isFinite(Number(n))).map(Number);
    if (!numbers.length) return '';
    const items = numbers.length === 1 ? [numbers[0], numbers[0]] : numbers;
    let low = Math.min(...items), high = Math.max(...items);
    if (low === high) { low -= 1; high += 1; }
    return items.map((n, i) => `${(i / (items.length - 1) * 100).toFixed(1)},${(28 - (n - low) / (high - low) * 22).toFixed(1)}`).join(' ');
  });
</script>
<svg class:bad class="spark" viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden="true">
  {#if points}<polygon class="area" points={`0,32 ${points} 100,32`}></polygon><polyline class="line" points={points}></polyline>{/if}
</svg>
