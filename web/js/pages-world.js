/* ============ pages-world.js：世界观设定管理页面 ============ */
var WorldPage = {
  init: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    this.load();
  },
  load: async function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty('请先在左侧选中一个项目'); return; }
    try {
      Store.state.worldSettings = await API.listWorldSettings(p.id);
      this.render();
    } catch (e) {
      document.getElementById('worldGrid').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>加载失败</div></div>';
    }
  },
  render: function () {
    var grid = document.getElementById('worldGrid');
    var search = (document.getElementById('worldSearch') || { value: '' }).value.toLowerCase();
    var items = Store.state.worldSettings || [];
    if (search) {
      items = items.filter(function (w) {
        var n = (w.name || '').toLowerCase();
        var d = (w.description || w.content || '').toLowerCase();
        var cat = (w.category || '').toLowerCase();
        return n.indexOf(search) >= 0 || d.indexOf(search) >= 0 || cat.indexOf(search) >= 0;
      });
    }
    if (!items.length) {
      grid.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">🌍</div><div>' + (search ? '未找到匹配设定' : '暂无世界观设定，点击上方按钮创建') + '</div></div>';
      return;
    }
    var html = '';
    items.forEach(function (w) {
      html += '<div class="world-card" onclick="WorldPage.edit(\'' + w.id + '\')">';
      html += '<div class="world-card-icon">' + WorldPage.iconFor(w.category) + '</div>';
      html += '<div class="world-card-body">';
      html += '<div class="world-card-name">' + esc(w.name || '未命名') + '</div>';
      if (w.category) html += '<span class="world-tag">' + esc(w.category) + '</span>';
      var desc = w.description || w.content || '';
      if (desc.length > 100) desc = desc.substring(0, 100) + '...';
      html += '<div class="world-card-desc">' + esc(desc) + '</div>';
      html += '</div>';
      html += '<div class="world-card-acts">';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();WorldPage.edit(\'' + w.id + '\')" title="编辑">✏️</span>';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();WorldPage.del(\'' + w.id + '\')" title="删除">🗑</span>';
      html += '</div>';
      html += '</div>';
    });
    grid.innerHTML = html;
  },
  iconFor: function (cat) {
    var map = { '地理': '🗺', '历史': '📜', '政治': '🏛', '经济': '💰', '文化': '🎭', '种族': '👥', '魔法/力量体系': '✨', '科技': '🔬', '宗教': '🙏', '法律': '⚖' };
    return map[cat] || '📖';
  },
  showCreate: function () {
    this.edit(null);
  },
  edit: function (id) {
    var w = null;
    if (id) {
      var items = Store.state.worldSettings || [];
      for (var i = 0; i < items.length; i++) { if (items[i].id === id) { w = items[i]; break; } }
    }
    var isNew = !w;
    var cats = ['地理', '历史', '政治', '经济', '文化', '种族', '魔法/力量体系', '科技', '宗教', '法律', '其他'];
    var html = '<div class="form-group"><label>设定名称 *</label><input id="worldName" value="' + escAttr(w ? w.name || '' : '') + '"></div>';
    html += '<div class="form-group"><label>分类</label><select id="worldCategory">';
    cats.forEach(function (cat) {
      html += '<option value="' + cat + '"' + (w && w.category === cat ? ' selected' : '') + '>' + cat + '</option>';
    });
    html += '</select></div>';
    html += '<div class="form-group"><label>设定内容 *</label><textarea id="worldContent" rows="6" placeholder="详细描述世界观设定...">' + esc(w ? w.description || w.content || '' : '') + '</textarea></div>';
    html += '<div class="form-group"><label>备注</label><textarea id="worldNotes" rows="2">' + esc(w ? w.notes || '' : '') + '</textarea></div>';

    var self = this;
    UI.modal({
      title: (isNew ? '新建' : '编辑') + '世界观设定',
      body: html,
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: isNew ? '创建' : '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var data = {
            name: document.getElementById('worldName').value.trim(),
            category: document.getElementById('worldCategory').value,
            description: document.getElementById('worldContent').value.trim(),
            notes: document.getElementById('worldNotes').value.trim()
          };
          if (!data.name) { UI.toast('请输入设定名称', 'warn'); return; }
          if (!data.description) { UI.toast('请输入设定内容', 'warn'); return; }
          ov.remove();
          self.save(id, data);
        }}
      ]
    });
  },
  save: async function (id, data) {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先在左侧选择项目', 'warn'); return; }
    data.project_id = p.id;
    try {
      if (id) { await API.updateWorldSetting(id, data); }
      else { await API.createWorldSetting(data); }
      UI.toast(id ? '已保存' : '已创建', 'success');
      this.load();
    } catch (e) { UI.toast('保存失败: ' + e.message, 'error'); }
  },
  del: function (id) {
    var self = this;
    UI.confirm('删除世界观设定', '确定删除此设定？此操作不可撤销！', function () {
      API.deleteWorldSetting(id).then(function () {
        UI.toast('已删除', 'success');
        self.load();
      }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  importFromText: function () {
    var self = this;
    // 自动取当前编辑器内容（无需手动复制）；无内容时提示可粘贴或一键使用全书
    var editorText = '';
    try { editorText = (typeof Editor !== 'undefined' && Editor.getText) ? (Editor.getText() || '') : ''; } catch (e) {}
    var ch = Store.state.currentChapter;
    var chText = ch && ch.content ? ch.content : '';
    var initial = editorText || chText || '';
    if (initial.length > 60000) initial = initial.slice(0, 60000);
    UI.modal({
      title: '从正文提取世界观',
      wide: '620px',
      body: '<p style="font-size:12px;color:var(--muted);margin-bottom:8px">已自动填入当前' + (editorText ? '编辑器内容' : (chText ? '章节内容' : '（编辑器为空，可粘贴或点击下方按钮使用全书）')) + '，AI 将识别并提取世界观设定。</p>' +
        '<div style="margin-bottom:8px;display:flex;gap:6px;flex-wrap:wrap">' +
        '<button class="tool-btn" type="button" onclick="document.getElementById(\'worldImportText\').value=window.__worldAutoSource\'current\';return false" style="font-size:11px">🔄 重新载入当前内容</button>' +
        '<button class="tool-btn" type="button" onclick="WorldPage.useAllChapters(\'worldImportText\')" style="font-size:11px">📚 使用全书内容</button>' +
        '<button class="tool-btn" type="button" onclick="document.getElementById(\'worldImportText\').value=\'\';return false" style="font-size:11px">🗑 清空</button>' +
        '</div>' +
        '<textarea id="worldImportText" rows="10" style="width:100%;font-size:13px">' + esc(initial) + '</textarea>' +
        '<div style="font-size:10px;color:var(--faint);margin-top:4px">提示：可直接编辑上方文本；「使用全书内容」会拼接本项目的全部章节。</div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '提取', cls: 'btn-primary', onClick: function (m, ov) {
          var text = document.getElementById('worldImportText').value.trim();
          if (!text) { UI.toast('请输入正文内容', 'warn'); return; }
          ov.remove();
          self.doImport(text);
        }}
      ]
    });
    try { window.__worldAutoSource = { 'current': initial }; } catch (e) {}
  },
  useAllChapters: function (taId) {
    var chs = Store.state.chapters || [];
    if (!chs.length) { UI.toast('本项目还没有章节', 'warn'); return; }
    var text = chs.map(function (c) { return '【' + (c.title || '未命名') + '】\n' + (c.content || ''); }).join('\n\n');
    if (text.length > 60000) text = text.slice(0, 60000);
    var ta = document.getElementById(taId);
    if (ta) { ta.value = text; UI.toast('已载入全书内容（' + chs.length + ' 章）', 'success'); }
  },
  doImport: async function (text) {
    UI.toast('正在分析世界观...', 'info');
    try {
      var resp = await API.req('POST', '/api/tools/execute', {
        tool: 'extract_worldsetting',
        project_id: Store.state.currentProject.id,
        text: text
      });
      if (resp && resp.result) {
        var self = this;
        UI.modal({
          title: '提取结果',
          body: '<div class="result-box" style="max-height:60vh;overflow-y:auto;white-space:pre-wrap">' + esc(typeof resp.result === 'string' ? resp.result : JSON.stringify(resp.result, null, 2)) + '</div>',
          actions: [
            { id: 'cancel', label: '关闭' },
            { id: 'refresh', label: '刷新列表', cls: 'btn-primary', onClick: function (m, ov) { ov.remove(); self.load(); } }
          ]
        });
      }
    } catch (e) { UI.toast('提取失败: ' + e.message, 'error'); }
  },
  showEmpty: function (msg) {
    document.getElementById('worldGrid').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">🌍</div><div>' + msg + '</div></div>';
  }
};
