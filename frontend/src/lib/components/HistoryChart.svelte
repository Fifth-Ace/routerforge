<script>
  import { settings } from '$lib/stores/settings.js';
  import { t } from '$lib/i18n/index.js';

  export let history = {};

  const W = 1000;
  const H = 250;
  const pad = { l:42, r:14, t:14, b:46 };
  const plotBottom = H - pad.b - 18;
  const fractions = [0,.25,.5,.75,1];

  $: locale = $settings.locale || 'ru';

  function niceMax(value) {
    if (value <= 5) return Math.max(1, Math.ceil(value));
    const p = 10 ** Math.floor(Math.log10(value));
    const n = value / p;
    return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10) * p;
  }

  function smoothPath(points) {
    if (!points.length) return '';
    if (points.length === 1) return `M ${points[0][0]} ${points[0][1]}`;
    let d = `M ${points[0][0].toFixed(1)} ${points[0][1].toFixed(1)}`;
    for (let i=0; i<points.length-1; i+=1) {
      const p0=points[i-1]||points[i], p1=points[i], p2=points[i+1], p3=points[i+2]||p2;
      const c1x=p1[0]+(p2[0]-p0[0])/12, c1y=p1[1]+(p2[1]-p0[1])/12;
      const c2x=p2[0]-(p3[0]-p1[0])/12, c2y=p2[1]-(p3[1]-p1[1])/12;
      d += ` C ${c1x.toFixed(1)} ${c1y.toFixed(1)}, ${c2x.toFixed(1)} ${c2y.toFixed(1)}, ${p2[0].toFixed(1)} ${p2[1].toFixed(1)}`;
    }
    return d;
  }

  function eventHeight(value) {
    const n=Number(value||0);
    return n>0 ? Math.min(14,3+Math.log2(n+1)*3) : 0;
  }

  $: points=history?.points||[];
  $: sufficient=history?.sufficient!==false;
  $: coverage=Math.round(Number(history?.coverage||0)*100);
  $: maxReq=niceMax(Math.max(1,...points.map((p)=>Number(p.requests||0))));
  $: x=(i)=>pad.l+(W-pad.l-pad.r)*(points.length<=1?0:i/(points.length-1));
  $: y=(v)=>plotBottom-(plotBottom-pad.t)*(Number(v||0)/maxReq);
  $: reqPoints=points.map((p,i)=>[x(i),y(p.requests)]);
  $: line=smoothPath(reqPoints);
  $: area=reqPoints.length>1 ? `${line} L ${reqPoints.at(-1)[0].toFixed(1)} ${plotBottom} L ${reqPoints[0][0].toFixed(1)} ${plotBottom} Z` : '';
  $: labelIndexes=[0,Math.floor((points.length-1)/2),points.length-1].filter((v,i,a)=>v>=0&&a.indexOf(v)===i);
  $: eventBase=plotBottom+15;
</script>

{#if !points.length}
  <div class="empty chart-empty">{t(locale, 'history.empty')}</div>
{:else if !sufficient}
  <div class="empty chart-empty"><strong>{t(locale, 'history.insufficient')}</strong><span>{t(locale, 'history.coverage', { coverage })}</span></div>
{:else}
  <svg class="history-chart" viewBox="0 0 {W} {H}" preserveAspectRatio="none">
    <defs><linearGradient id="reqFill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="var(--accent)" stop-opacity=".24"/><stop offset="100%" stop-color="var(--accent)" stop-opacity="0"/></linearGradient></defs>
    {#each fractions as fraction}
      {@const yy=pad.t+(plotBottom-pad.t)*fraction}
      <line x1={pad.l} y1={yy} x2={W-pad.r} y2={yy} class="chart-grid"/>
      <text x="2" y={yy+3} class="chart-label">{Math.round(maxReq*(1-fraction))}</text>
    {/each}
    {#each labelIndexes as i}
      <text x={x(i)} y={H-5} text-anchor={i===0?'start':i===points.length-1?'end':'middle'} class="chart-label">{new Date(points[i].time).toLocaleTimeString(locale === 'en' ? 'en-GB' : 'ru-RU',{hour:'2-digit',minute:'2-digit'})}</text>
    {/each}
    {#if area}<path d={area} fill="url(#reqFill)"/>{/if}
    <path d={line} fill="none" stroke="var(--accent)" stroke-width="2" vector-effect="non-scaling-stroke"/>
    {#each points as point,i}
      {@const xx=x(i)}
      {@const fh=eventHeight(point.fallbacks)}
      {@const eh=eventHeight(point.errors)}
      {@const th=eventHeight(point.timeouts)}
      {#if fh}<line x1={xx-2} y1={eventBase} x2={xx-2} y2={eventBase-fh} class="chart-event-fallback" stroke-width="2" vector-effect="non-scaling-stroke"/>{/if}
      {#if eh}<line x1={xx} y1={eventBase} x2={xx} y2={eventBase-eh} class="chart-event-error" stroke-width="2" vector-effect="non-scaling-stroke"/>{/if}
      {#if th}<line x1={xx+2} y1={eventBase} x2={xx+2} y2={eventBase-th} class="chart-event-timeout" stroke-width="2" vector-effect="non-scaling-stroke"/>{/if}
    {/each}
    <line x1={pad.l} y1={eventBase} x2={W-pad.r} y2={eventBase} class="chart-grid"/>
  </svg>
{/if}
