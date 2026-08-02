/* ============ rightpanel.js：右侧面板切换 + 设定资源勾选区 ============ */
var RightPanel = {
  switch: function (page) {
    // 移动端：右面板是底部弹出 sheet，切到对应页并展开
    var isMobile = !document.body.classList.contains('desktop');
    if (isMobile && typeof MobileUI !== 'undefined') {
      MobileUI.openRightSheet();
      MobileUI.switchRightPage(page);
      // 打开 sheet 后自动渲染对应页内容
      if (page === 'templates' && typeof TemplateUI !== 'undefined') TemplateUI.render();
      if (page === 'models' && typeof ModelSettings !== 'undefined') {
        ModelSettings.loadAll();
        setTimeout(function () {
          var body = document.getElementById('mrsBody');
          var pageEl = body ? body.querySelector('#page-models') : null;
          if (pageEl && typeof SkillsPanel !== 'undefined') {
            var existing = pageEl.querySelector('#skillsPanelContainer');
            if (!existing) {
              var div = document.createElement('div');
              div.id = 'skillsPanelContainer';
              div.style.cssText = 'margin-top:18px;border-top:1px solid var(--border);padding-top:10px';
              pageEl.appendChild(div);
            }
            SkillsPanel.renderTo('skillsPanelContainer');
          }
        }, 300);
      }
      if (page === 'appearance' && typeof Appearance !== 'undefined') Appearance.load();
      if (page === 'forecast' && typeof ForecastPanel !== 'undefined') ForecastPanel.render();
      if (page === 'state' && typeof StateViewer !== 'undefined') StateViewer.render();
      return;
    }
    // 桌面端：用户主动切换右侧面板时自动展开（若当前折叠），避免点了没反应/被拦截的观感
    var rp = document.getElementById('rightPanel');
    if (rp && rp.classList.contains('collapsed')) {
      rp.classList.remove('collapsed');
      document.body.classList.remove('right-hidden');
      Store.set('rightCollapsed', false);
    }
    document.querySelectorAll('.right-tab').forEach(function (t) {
      t.classList.toggle('active', t.dataset.page === page);
    });
    document.querySelectorAll('.right-page').forEach(function (p) {
      p.classList.toggle('show', p.id === 'page-' + page);
    });
    if (page === 'templates') TemplateUI.render();
    if (page === 'models') {
      ModelSettings.loadAll();
      setTimeout(function () {
        var pageEl = document.getElementById('page-models');
        if (pageEl && typeof SkillsPanel !== 'undefined') {
          var existing = document.getElementById('skillsPanelContainer');
          if (!existing) {
            var div = document.createElement('div');
            div.id = 'skillsPanelContainer';
            div.style.cssText = 'margin-top:18px;border-top:1px solid var(--border);padding-top:10px';
            pageEl.appendChild(div);
          }
          SkillsPanel.renderTo('skillsPanelContainer');
        }
      }, 300);
    }
    if (page === 'appearance') Appearance.load();
    if (page === 'forecast') { if (typeof ForecastPanel !== 'undefined') ForecastPanel.render(); }
    if (page === 'state') { if (typeof StateViewer !== 'undefined') StateViewer.render(); }
  },
  renderContext: function () {
    // 人物卡
    var cEl = document.getElementById('ctxCharacters');
    if (!Store.state.characters.length) {
      cEl.innerHTML = '<div class="ghead">人物卡</div><div class="res-check-empty" style="cursor:pointer;padding:12px;border:1px dashed var(--border);border-radius:8px;margin:4px 8px" onclick="ResourceUI.editCharacter()">' +
        '<div style="font-weight:600;margin-bottom:2px">👤 暂无人物卡</div>' +
        '<div style="font-size:10px;color:var(--muted)">点击此处新建人物，或在工具面板使用 AI 提取</div></div>';
    } else {
      cEl.innerHTML = '<div class="ghead">人物卡</div>' + Store.state.characters.map(function (c) {
        var checked = Store.state.selection.characters.has(c.id);
        var f = ResourceUI.unpackChar(c.description);
        var desc = [f.personality, f.background].filter(Boolean).join(' · ');
        return '<div class="check-item' + (checked ? ' checked' : '') + '" onclick="RightPanel.toggle(\'characters\',\'' + c.id + '\')">' +
          '<div class="box">✓</div>' +
          '<div class="txt"><div class="n">' + esc(c.name) + '</div><div class="d">' + esc(desc) + '</div></div>' +
          '</div>';
      }).join('');
    }
    // 世界观
    var wEl = document.getElementById('ctxWorld');
    if (!Store.state.worldSettings.length) {
      wEl.innerHTML = '<div class="ghead">世界观设定</div><div class="res-check-empty" style="cursor:pointer;padding:12px;border:1px dashed var(--border);border-radius:8px;margin:4px 8px" onclick="ResourceUI.editWorld()">' +
        '<div style="font-weight:600;margin-bottom:2px">🌍 暂无世界观</div>' +
        '<div style="font-size:10px;color:var(--muted)">点击此处新建设定，定义你的小说世界规则</div></div>';
    } else {
      wEl.innerHTML = '<div class="ghead">世界观设定</div>' + Store.state.worldSettings.map(function (w) {
        var checked = Store.state.selection.worldSettings.has(w.id);
        return '<div class="check-item' + (checked ? ' checked' : '') + '" onclick="RightPanel.toggle(\'worldSettings\',\'' + w.id + '\')">' +
          '<div class="box">✓</div>' +
          '<div class="txt"><div class="n">' + esc(w.title) + '</div><div class="d">' + esc((w.content || '').slice(0, 60)) + '</div></div>' +
          '</div>';
      }).join('');
    }
    // 素材
    var mEl = document.getElementById('ctxMaterials');
    if (!Store.state.materials.length) {
      mEl.innerHTML = '<div class="ghead">素材文档</div><div class="res-check-empty" style="cursor:pointer" onclick="ResourceUI.uploadMaterial()">暂无素材，点击此处上传</div>';
    } else {
      mEl.innerHTML = '<div class="ghead">素材文档</div>' + Store.state.materials.map(function (m) {
        var checked = Store.state.selection.materials.has(m.id);
        return '<div class="check-item' + (checked ? ' checked' : '') + '" onclick="RightPanel.toggle(\'materials\',\'' + m.id + '\')">' +
          '<div class="box">✓</div>' +
          '<div class="txt"><div class="n">' + esc(m.name) + '</div><div class="d">' + esc((m.content || '').slice(0, 60)) + '</div></div>' +
          '<button class="tool-btn" style="font-size:9px;padding:1px 4px;margin-left:4px" onclick="event.stopPropagation();Editor.insertAtCursor(document.getElementById(\'ctxMaterials\').dataset.content || \'\')" title="插入到编辑器">插入</button>' +
          '</div>';
      }).join('');
    }
  },
  toggle: function (group, id) {
    var s = Store.state.selection[group];
    if (s.has(id)) s.delete(id); else s.add(id);
    if (Store.state.currentProject) Store.saveSelection(Store.state.currentProject.id);
    this.renderContext();
  }
};
