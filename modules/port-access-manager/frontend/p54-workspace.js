(function () {
  'use strict';
  // P54: single workspace. Move original nodes, never clone action controls.
  var main = document.querySelector('main.pa');
  var old = document.querySelector('.rf-pa-workspace');
  if (!main || !old || !document.getElementById('odin-manager')) return;
  function by(id) { return document.getElementById(id); }
  function make(tag, cls, label) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (label !== undefined) n.textContent = label;
    return n;
  }
  var header = main.querySelector('header');
  if (header) {
    var p = header.querySelector('p');
    if (p) p.textContent = 'Выберите метод защиты, управляйте доступом и проверяйте состояние в одном месте.';
  }
  var root = make('div','p54-root');
  old.parentNode.insertBefore(root,old);
  var nav = make('nav','p54-navigation');
  nav.setAttribute('aria-label','Разделы контроля доступа');
  root.appendChild(nav);
  var sections = {};
  var specs = [
    ['protection','Защита','Настройка и применение'],
    ['access','Доступы','Авторизованные IP и черновики'],
    ['engines','Движки','Службы и конфигурации'],
    ['diagnostics','Диагностика','Проверки и журнал']
  ];
  var buttons = {};
  specs.forEach(function(spec){
    var b = make('button','p54-tab');b.type='button';
    b.appendChild(make('strong','',spec[1]));
    b.appendChild(make('small','',spec[2]));
    nav.appendChild(b);buttons[spec[0]]=b;
    var section = make('section','p54-page');section.id='p54-'+spec[0];
    section.setAttribute('aria-label',spec[1]);
    root.appendChild(section);sections[spec[0]]=section;
  });
  function heading(page,title,help){
    var box=make('div','p54-heading');
    box.appendChild(make('h2','',title));
    box.appendChild(make('p','',help));
    sections[page].appendChild(box);
  }
  heading('protection','Настроить защиту','Выберите метод и задайте параметры. Включение защиты — отдельное подтверждаемое действие.');
  heading('access','Доступы и правила','Активные разрешения и сохранённые черновики — разные сущности.');
  heading('engines','Движки Entware','Установка, запуск, остановка и редактирование существующих конфигураций.');
  heading('diagnostics','Диагностика и инструменты','Проверки, предпросмотр, журнал и экспериментальные сценарии.');
  function move(node, page) { if (node) sections[page].appendChild(node); }
  // Existing engine-switch and monitoring handlers are bound to these very nodes.
  move(old.querySelector('.rf-pa-summary'),'protection');
  move(old.querySelector('.rf-pa-selector'),'protection');
  move(old.querySelector('.rf-pa-working'),'protection');
  move(by('odin-live-access'),'access');
  move(by('staging-panel'),'access');
  move(by('recent-live-panel'),'access');
  move(old.querySelector('.rf-pa-advanced'),'diagnostics');
  move(main.querySelector('.rf-pa-operation-log'),'diagnostics');
  // The engine cards and configuration editor retain all their listeners.
  move(by('existing-engine-control'),'engines');
  move(old.querySelector('.rf-pa-compact-advanced'),'engines');
  // If the old compact layer did not wrap the editor, preserve its fallback.
  var editor=by('engine-config-panel');
  if (editor && !sections.engines.contains(editor)) move(editor,'engines');
  // Remove only presentation-only wrappers introduced in P53.
  var previousHero=old.querySelector('.rf-pa-p53-hero');
  if (previousHero) previousHero.remove();
  var workflow=old.querySelector('.rf-pa-p53-workflow');
  if (workflow) workflow.remove();
  var context=old.querySelector('.rf-pa-p53-context');
  if (context) context.remove();
  var oldTabs=old.querySelector('.rf-pa-view-tabs');
  if (oldTabs) { oldTabs.classList.add('p54-internal-switch'); root.appendChild(oldTabs); }
  // The legacy javascript still controls the working/advanced visibility.
  var protectionTab = oldTabs && oldTabs.querySelector('.rf-pa-view-tab:first-child');
  var toolsTab = oldTabs && oldTabs.querySelector('.rf-pa-view-tab:nth-child(2)');
  var active='protection';
  function show(page) {
    if (page==='engines' && document.querySelector('.rf-pa-choice[aria-pressed="true"]') &&
        document.querySelector('.rf-pa-choice[aria-pressed="true"]')===document.querySelector('.rf-pa-choice')) {
      var knock=document.querySelectorAll('.rf-pa-choice')[1];
      if(knock) knock.click();
    }
    if (page==='diagnostics') {
      if (toolsTab) toolsTab.click();
    } else if (protectionTab) protectionTab.click();
    active=page;
    specs.forEach(function(spec){
      var selected=spec[0]===page;
      sections[spec[0]].hidden=!selected;
      buttons[spec[0]].classList.toggle('active',selected);
      buttons[spec[0]].setAttribute('aria-current',selected?'page':'false');
    });
    // On Diagnostics, legacy view-switch exposes advanced. All other sections
    // use their own visibility; legacy tools never leak into the main view.
    if(page==='diagnostics') {
      var adv=old.querySelector('.rf-pa-advanced') || sections.diagnostics.querySelector('.rf-pa-advanced');
      if(adv)adv.hidden=false;
    }
    if(page==='protection') {
      var w=sections.protection.querySelector('.rf-pa-working');
      var chooser=sections.protection.querySelector('.rf-pa-selector');
      if(w)w.hidden=false;if(chooser)chooser.hidden=false;
    }
  }
  specs.forEach(function(spec){buttons[spec[0]].addEventListener('click',function(){show(spec[0]);});});
  // All original markup, action IDs and underlying requests remain intact.
  // No API calls, firewall writes, or alternative action implementations.
  show('protection');
  // Old workspace now contains only unneeded presentation wrappers.
  // Keep it off-screen so no duplicate toolbars or shortcut buttons surface.
  old.hidden=true;
  window.addEventListener('pageshow',function(){show(active);});
})();
