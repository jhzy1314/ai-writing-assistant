/* ============ versions.js：版本管理 + 手动保存 + 自动草稿 ============ */
var VersionUI = {
  open: function () {
    if (!Store.state.currentProject) { UI.toast('请先选择项目', 'warn'); return; }
    var list = Store.state.versions;
    var cur = Store.state.latestVersion;
    var body = '<div class="ver-list">';
    if (!list.length) body += '<div class="res-check-empty">暂无版本，AI 生成或手动保存后将自动创建</div>';
    list.forEach(function (v) {
      var isCur = cur && v.id === cur.id;
      var name = Store.get('verName.' + v.id, null);
      var title = name || v.title || ('版本' + v.version);
      body += '<div class="ver-item' + (isCur ? ' current' : '') + '" data-id="' + v.id + '">' +
        '<div class="vh"><span class="vno">V' + v.version + '</span>' +
        '<span class="vt" data-tid="' + v.id + '">' + esc(title) + '</span></div>' +
        '<div class="vm"><span>' + esc(fmtTime(v.created_at)) + '</span><span>' + wordCount(v.content).toLocaleString() + ' 字</span></div>' +
        '<div class="vacts">' +
        '<button class="tool-btn" onclick="VersionUI.rollback(\'' + v.id + '\')">♻ 回退</button>' +
        '<button class="tool-btn" onclick="VersionUI.rename(\'' + v.id + '\')">✏ 重命名</button>' +
        '<button class="tool-btn" onclick="VersionUI.view(\'' + v.id + '\')">👁 查看</button>' +
        '</div></div>';
    });
    body += '</div>';
    UI.modal({ title: '版本历史', sub: '每次 AI 生成完成自动保存版本。回退将载入内容到编辑器（可继续编辑后另存为新版本）。', body: body, wide: '520px' });
  },
  rollback: async function (vid) {
    try {
      var v = await API.getVersion(vid);
      Editor.setContent(v.content || '');
      UI.toast('已回退到 V' + v.version + '（可在编辑后保存为新版本）', 'success');
      document.querySelectorAll('.modal-overlay').forEach(function (o) { o.remove(); });
    } catch (e) { UI.toast('回退失败：' + e.message, 'error'); }
  },
  rename: function (vid, btn) {
    var span = btn.closest('.ver-item').querySelector('.vt[data-tid="' + vid + '"]');
    var old = span.textContent;
    UI.prompt('版本重命名', '版本备注', old, function (name) {
      if (!name) return;
      Store.set('verName.' + vid, name);
      span.textContent = name;
      var v = Store.state.versions.find(function (x) { return x.id === vid; });
      if (v) v.title = name;
      if (Store.state.latestVersion && Store.state.latestVersion.id === vid) Store.state.latestVersion.title = name;
      Sidebar.renderResources();
      UI.toast('已重命名', 'success');
    });
  },
  view: async function (vid) {
    try {
      var v = await API.getVersion(vid);
      UI.modal({ title: 'V' + v.version + ' · ' + (v.title || ''), body: '<div class="result-box">' + esc(v.content || '(空)') + '</div>', wide: '640px' });
    } catch (e) { UI.toast('查看失败：' + e.message, 'error'); }
  }
};

var ManualSave = {
  autosaveTimer: null,
  startAutosave: function () {
    if (this.autosaveTimer) clearInterval(this.autosaveTimer);
    this.autosaveTimer = setInterval(function () {
      var p = Store.state.currentProject;
      if (!p) return;
      var text = Editor.getText();
      if (text && text.trim()) {
        Store.saveDraft(p.id, text);
        var tag = document.getElementById('draftSavedTag');
        tag.style.display = '';
        setTimeout(function () { tag.style.display = 'none'; }, 1500);
      }
    }, 30000); // 30 秒自动存草稿
  },
  saveVersionAuto: async function (text) {
    var p = Store.state.currentProject;
    if (!p || !text) return;
    try {
      var ch = Store.state.currentChapter;
      var title = (ch ? ch.title + ' · ' : '') + Store.state.composer.runMode + ' ' + new Date().toLocaleString('zh-CN', { hour12: false }).slice(0, 16);
      var v = await API.saveVersion(p.id, title, text);
      Store.state.versions.unshift(v);
      Store.state.latestVersion = v;
      Store.clearDraft(p.id);
      ProjectUI.updateMeta();
      Sidebar.renderResources();
    } catch (e) { UI.toast('自动保存版本失败：' + e.message, 'error'); }
  },
  saveDraft: async function () {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('内容为空', 'warn'); return; }
    try {
      var title = '手动保存 ' + new Date().toLocaleString('zh-CN', { hour12: false });
      var v = await API.saveVersion(p.id, title, text);
      Store.state.versions.unshift(v);
      Store.state.latestVersion = v;
      Store.clearDraft(p.id);
      ProjectUI.updateMeta();
      Sidebar.renderResources();
      UI.toast('已保存为 V' + v.version, 'success');
    } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
  },
  restoreDraft: function () {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    var d = Store.getDraft(p.id);
    if (!d || !d.text) { UI.toast('未发现未保存草稿', ''); return; }
    UI.confirm('恢复草稿', '发现未保存草稿（' + new Date(d.ts).toLocaleString('zh-CN', { hour12: false }) + '，' + wordCount(d.text).toLocaleString() + '字）。恢复将覆盖当前编辑器内容。', function () {
      Editor.setContent(d.text);
      UI.toast('已恢复草稿', 'success');
    });
  }
};
