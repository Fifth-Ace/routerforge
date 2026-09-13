(function () {
  'use strict';

  function params() { return new URLSearchParams(window.location.search || ''); }
  function safeHex(value, fallback) { var v=String(value||'').trim(); return /^#[0-9a-f]{6}$/i.test(v)?v.toLowerCase():fallback; }
  function mix(a,b,t){a=safeHex(a,'#000000').slice(1);b=safeHex(b,'#ffffff').slice(1);function p(x){return parseInt(x,16);}return '#'+[0,2,4].map(function(i){return Math.round(p(a.slice(i,i+2))*(1-t)+p(b.slice(i,i+2))*t).toString(16).padStart(2,'0');}).join('');}
  function rgba(hex,alpha){var r=safeHex(hex,'#38bdf8').slice(1);var v=[0,2,4].map(function(i){return parseInt(r.slice(i,i+2),16);});return 'rgba('+v[0]+','+v[1]+','+v[2]+','+alpha+')';}
  function luminance(hex){var c=safeHex(hex,'#000000').slice(1);var w=[.2126,.7152,.0722];return [0,2,4].map(function(i){return parseInt(c.slice(i,i+2),16);}).reduce(function(s,v,i){return s+v*w[i];},0)/255;}
  function applyTheme(){
    var p=params();
    var theme=p.get('theme')||'forge';
    var accent=safeHex(p.get('accent'),'#38bdf8');
    var palettes={forge:{background:'#0b0d10',text:'#f5f7fa',surface:'#12151a',surface2:'#171b21',hover:'#1d2229',muted:'#8d98a4',border:'#29313a',borderStrong:'#36414d'},midnight:{background:'#08111b',text:'#edf5ff',surface:'#0f1824',surface2:'#152131',hover:'#1b2a3c',muted:'#8ca0b5',border:'#25384a',borderStrong:'#34516a'},graphite:{background:'#101113',text:'#f1f2f4',surface:'#17191c',surface2:'#1d2024',hover:'#25292e',muted:'#9299a2',border:'#30353c',borderStrong:'#414850'}};
    var palette=palettes[theme]||palettes.forge;
    if(theme==='custom'){
      var bg=safeHex(p.get('background'),'#0b0d10');var text=safeHex(p.get('text'),'#f5f7fa');var light=luminance(bg)>.55;
      palette={background:bg,text:text,surface:mix(bg,text,light?.045:.055),surface2:mix(bg,text,light?.09:.105),hover:mix(bg,text,light?.13:.15),muted:mix(text,bg,.48),border:mix(bg,text,light?.18:.19),borderStrong:mix(bg,text,light?.27:.29)};
    }
    var s=document.documentElement.style;
    s.setProperty('--rf-bg',palette.background);s.setProperty('--rf-surface',palette.surface);s.setProperty('--rf-surface-2',palette.surface2);s.setProperty('--rf-hover',palette.hover);s.setProperty('--rf-text',palette.text);s.setProperty('--rf-muted',palette.muted);s.setProperty('--rf-border',palette.border);s.setProperty('--rf-border-strong',palette.borderStrong);s.setProperty('--rf-accent',accent);s.setProperty('--rf-accent-soft',rgba(accent,.10));s.setProperty('--rf-accent-hover',rgba(accent,.16));s.setProperty('--rf-accent-border',rgba(accent,.30));
    document.documentElement.dataset.theme=theme;document.documentElement.dataset.density=p.get('density')||'normal';document.documentElement.dataset.radius=p.get('radius')||'default';
  }
  function basePath(){var path=window.location.pathname;var marker='/ui/';var pos=path.indexOf(marker);return pos>=0?path.slice(0,pos):path.replace(/\/ui$/,'');}
  function moduleId(){var match=window.location.pathname.match(/\/api\/modules\/([^/]+)\//);return match?decodeURIComponent(match[1]):'network-tools';}
  function esc(value){return String(value==null?'':value).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');}
  function fmtBytes(value){var n=Number(value||0);if(!Number.isFinite(n)||n<=0)return '0 B';var units=['B','KB','MB','GB','TB'];var i=Math.min(Math.floor(Math.log(n)/Math.log(1024)),units.length-1);return (n/Math.pow(1024,i)).toFixed(i?1:0)+' '+units[i];}
  async function request(path,query){var url=new URL(basePath()+path,window.location.origin);Object.keys(query||{}).forEach(function(key){var value=query[key];if(value!==''&&value!=null)url.searchParams.set(key,value);});var response=await fetch(url.toString(),{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}});var text=await response.text();var data;try{data=text?JSON.parse(text):{};}catch(_){data={error:text||'invalid JSON response'};}if(!response.ok)throw new Error(data.error||('HTTP '+response.status));return data;}
  function badge(text,tone){return '<span class="rf-badge '+esc(tone||'')+'">'+esc(text)+'</span>';}
  var lastPostedHeight=0;
  function measuredContentHeight(){
    var page=document.querySelector('.nt-page');
    if(page){
      var rect=page.getBoundingClientRect();
      return Math.ceil(Math.max(page.scrollHeight||0,page.offsetHeight||0,rect.height||0)+18);
    }
    var body=document.body;
    return Math.ceil(Math.max(body?body.scrollHeight:0,body?body.offsetHeight:0)+18);
  }
  function notifyHeight(){
    try{
      if(window.parent===window)return;
      var height=Math.max(360,Math.min(6000,measuredContentHeight()));
      if(Math.abs(height-lastPostedHeight)<2)return;
      lastPostedHeight=height;
      window.parent.postMessage({type:'routerforge-module-height',moduleId:moduleId(),height:height},window.location.origin);
    }catch(_){}
  }
  function error(target,err){var node=typeof target==='string'?document.querySelector(target):target;if(node)node.innerHTML='<div class="rf-error">'+esc(err&&err.message?err.message:err)+'</div>';notifyHeight();}
  applyTheme();
  window.RFNet={request:request,esc:esc,fmtBytes:fmtBytes,badge:badge,error:error,notifyHeight:notifyHeight,params:params};
  window.addEventListener('load',notifyHeight);
  window.addEventListener('resize',notifyHeight);
  if(typeof ResizeObserver==='function'){
    var observed=document.querySelector('.nt-page')||document.body;
    if(observed){var ro=new ResizeObserver(notifyHeight);ro.observe(observed);}
  }else if(typeof MutationObserver==='function'){
    var observedFallback=document.querySelector('.nt-page')||document.body;
    if(observedFallback){var mo=new MutationObserver(notifyHeight);mo.observe(observedFallback,{subtree:true,childList:true,attributes:true});}
  }
}());
