/* ============ sidebar.js：左侧栏渲染（项目 + 当前项目资源树 + 章节树） ============ */
var Sidebar = {
  filter: function (q) {
    var self = this;
    if (this._ft) clearTimeout(this._ft);
    this._ft = setTimeout(function () {
      ProjectUI.renderList(q);
      self.renderResources(q);
      ChapterUI.renderTree(q);
    }, 250);
  },
  lock: function () {
    var el = document.getElementById('sidebar');
    if (el) el.style.pointerEvents = 'none';
  },
  unlock: function () {
    var el = document.getElementById('sidebar');
    if (el) el.style.pointerEvents = '';
  },
  renderResources: function (filter) {
    var sec = document.getElementById('resourceSection');
    var p = Store.state.currentProject;
    if (!p) { sec.classList.remove('show'); return; }
    sec.classList.add('show');
    var f = filter ? filter.toLowerCase() : '';
    var match = function (s) { return !f || (s || '').toLowerCase().includes(f); };
    var html = '';
    // 章节树（在 resources 区域之前渲染）
    // 章节树有独立容器 chapterTree，由 ChapterUI.renderTree 负责
    // 稿件版本
    var vers = Store.state.versions.filter(function (v) { return match(v.title || ('版本' + v.version)); });
    html += '<div class="res-group">' +
      '<div class="res-group-head" onclick="Sidebar.toggleGroup(this)"><span class="caret">▾</span>📖 稿件版本<span class="count">' + Store.state.versions.length + '</span></div>' +
      '<div class="res-group-body">';
    if (!Store.state.versions.length) html += '<div class="res-check-empty">暂无版本</div>';
    vers.slice(0, 12).forEach(function (v) {
      html += '<div class="res-item" onclick="VersionUI.open()" title="' + esc(v.title) + '">' +
        '<span class="icon">V' + v.version + '</span><span class="name">' + esc(v.title || ('版本' + v.version)) + '</span></div>';
    });
    html += '</div></div>';
    // 人物卡
    var chars = Store.state.characters.filter(function (c) { return match(c.name); });
    html += '<div class="res-group">' +
      '<div class="res-group-head" onclick="Sidebar.toggleGroup(this)"><span class="caret">▾</span>👥 人物卡<span class="count">' + Store.state.characters.length + '</span>' +
      '<span class="link-btn" onclick="event.stopPropagation();ResourceUI.editCharacter()">＋</span>' +
      '<span class="link-btn" onclick="event.stopPropagation();ResourceUI.summarizeSettings()" title="AI 从正文自动提取人物和世界观">🤖</span></div>' +
      '<div class="res-group-body">';
    if (!Store.state.characters.length) html += '<div class="res-check-empty" style="padding:12px;background:var(--panel3);border-radius:8px;border:1px dashed var(--border);margin:4px 0">' +
      '<div style="font-weight:600;margin-bottom:4px">📝 开始创建人物</div>' +
      '<div style="font-size:10px;color:var(--muted);line-height:1.7">' +
      '方法一：点击上方 <b>＋</b> 按钮手动创建人物卡<br>' +
      '方法二：选中章节后点击 <b>🤖</b> AI 一键从正文提取<br>' +
      '方法三：在「工具」面板使用「自动提取设定」</div></div>';
    chars.forEach(function (c) {
      html += '<div class="res-item" onclick="ResourceUI.editCharacter(\'' + c.id + '\')" title="' + esc(c.name) + '">' +
        '<span class="icon">👤</span><span class="name">' + esc(c.name) + '</span>' +
        '<span class="row-del" onclick="event.stopPropagation();ResourceUI.delCharacter(\'' + c.id + '\')">✕</span></div>';
    });
    html += '</div></div>';
    // 世界观
    var ws = Store.state.worldSettings.filter(function (w) { return match(w.title); });
    html += '<div class="res-group">' +
      '<div class="res-group-head" onclick="Sidebar.toggleGroup(this)"><span class="caret">▾</span>🌍 世界观<span class="count">' + Store.state.worldSettings.length + '</span>' +
      '<span class="link-btn" onclick="event.stopPropagation();ResourceUI.editWorld()">＋</span></div>' +
      '<div class="res-group-body">';
    if (!Store.state.worldSettings.length) html += '<div class="res-check-empty" style="padding:12px;background:var(--panel3);border-radius:8px;border:1px dashed var(--border);margin:4px 0">' +
      '<div style="font-weight:600;margin-bottom:4px">🌍 开始创建世界观</div>' +
      '<div style="font-size:10px;color:var(--muted);line-height:1.7">' +
      '方法一：点击上方 <b>＋</b> 按钮手动创建世界观<br>' +
      '方法二：在「工具」面板使用「自动提取设定」<br>' +
      '方法三：先写正文，再用 AI 从正文提取</div></div>';
    ws.forEach(function (w) {
      html += '<div class="res-item" onclick="ResourceUI.editWorld(\'' + w.id + '\')" title="' + esc(w.title) + '">' +
        '<span class="icon">🌐</span><span class="name">' + esc(w.title) + '</span>' +
        '<span class="row-del" onclick="event.stopPropagation();ResourceUI.delWorld(\'' + w.id + '\')">✕</span></div>';
    });
    html += '</div></div>';
    // 素材
    var mats = Store.state.materials.filter(function (m) { return match(m.name); });
    html += '<div class="res-group">' +
      '<div class="res-group-head" onclick="Sidebar.toggleGroup(this)"><span class="caret">▾</span>📎 素材文档<span class="count">' + Store.state.materials.length + '</span>' +
      '<span class="link-btn" onclick="event.stopPropagation();ResourceUI.uploadMaterial()">＋</span></div>' +
      '<div class="res-group-body">';
    if (!Store.state.materials.length) html += '<div class="res-check-empty">暂无素材<br><small style="color:var(--muted)">点击上方 ＋ 上传 txt/md/docx 参考文档</small></div>';
    mats.forEach(function (m) {
      html += '<div class="res-item" onclick="ResourceUI.previewMaterial(\'' + m.id + '\')" title="' + esc(m.name) + '">' +
        '<span class="icon">📄</span><span class="name">' + esc(m.name) + '</span>' +
        '<span class="row-del" onclick="event.stopPropagation();ResourceUI.delMaterial(\'' + m.id + '\')">✕</span></div>';
    });
    html += '</div></div>';
    document.getElementById('resList').innerHTML = html;
  },
  toggleGroup: function (head) { head.parentElement.classList.toggle('collapsed'); },
  toggleResources: function () {
    var sec = document.getElementById('resourceSection');
    if (!sec) return;
    sec.classList.toggle('collapsed');
    var caret = document.getElementById('resCaret');
    if (caret) caret.textContent = sec.classList.contains('collapsed') ? '▸' : '▾';
    Store.set('resourcesCollapsed', sec.classList.contains('collapsed'));
  },
  restoreResourcesState: function () {
    var sec = document.getElementById('resourceSection');
    if (!sec) return;
    if (Store.get('resourcesCollapsed', false)) {
      sec.classList.add('collapsed');
      var caret = document.getElementById('resCaret');
      if (caret) caret.textContent = '▸';
    }
  }
};
