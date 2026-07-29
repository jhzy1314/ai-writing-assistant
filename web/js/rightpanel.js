/* ============ rightpanel.js：右侧面板切换 + 设定资源勾选区 ============ */
var RightPanel = {
  switch: function (page) {
    document.querySelectorAll('.right-tab').forEach(function (t) {
      t.classList.toggle('active', t.dataset.page === page);
    });
    document.querySelectorAll('.right-page').forEach(function (p) {
      p.classList.toggle('show', p.id === 'page-' + page);
    });
    if (page === 'templates') TemplateUI.render();
    if (page === 'models') ModelSettings.loadAll();
  },
  renderContext: function () {
    // 人物卡
    var cEl = document.getElementById('ctxCharacters');
    if (!Store.state.characters.length) {
      cEl.innerHTML = '<div class="ghead">人物卡</div><div class="res-check-empty" style="cursor:pointer" onclick="ResourceUI.editCharacter()">暂无人物卡，点击此处创建或使用【工具→自动提取人物卡】</div>';
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
      wEl.innerHTML = '<div class="ghead">世界观设定</div><div class="res-check-empty" style="cursor:pointer" onclick="ResourceUI.editWorld()">暂无世界观，点击此处创建</div>';
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
