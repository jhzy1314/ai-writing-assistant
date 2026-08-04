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
        var n = (w.title || w.name || '').toLowerCase();
        var d = (w.content || w.description || '').toLowerCase();
        var cat = (WorldPage.catOf(w) || '').toLowerCase();
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
      html += '<div class="world-card-name">' + esc(w.title || w.name || '未命名') + '</div>';
      var wcat = WorldPage.catOf(w);
      if (wcat) html += '<span class="world-tag">' + esc(wcat) + '</span>';
      var desc = w.content || w.description || '';
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
    var cats = ['地理环境', '历史背景', '社会文化', '校园/职场', '政治势力', '经济体系', '力量/能力体系', '科技', '宗教/信仰', '法律', '其他'];
    var html = '<div class="form-group"><label>设定名称 *</label><input id="worldName" value="' + escAttr(w ? w.title || w.name || '' : '') + '"></div>';
    html += '<div class="form-group"><label>分类</label><select id="worldCategory">';
    cats.forEach(function (cat) {
      html += '<option value="' + cat + '"' + (w && WorldPage.catOf(w) === cat ? ' selected' : '') + '>' + cat + '</option>';
    });
    html += '</select></div>';
    html += '<div class="form-group"><label>设定内容 *</label><textarea id="worldContent" rows="6" placeholder="详细描述世界观设定...">' + esc(w ? w.content || w.description || '' : '') + '</textarea></div>';
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
    var content = (data.category ? '【分类】' + data.category + '\n' : '') + (data.description || '') + (data.notes ? '\n备注：' + data.notes : '');
    var payload = { project_id: p.id, title: data.name, content: content };
    try {
      if (id) { await API.updateWorldSetting(id, payload); }
      else { await API.createWorldSetting(payload); }
      UI.toast(id ? '已保存' : '已创建', 'success');
      this.load();
    } catch (e) { UI.toast('保存失败: ' + e.message, 'error'); }
  },
  /* 从 content 首行解析分类（兼容老数据用 category 字段） */
  catOf: function (w) {
    if (!w) return '';
    if (w.category) return w.category;
    var c = w.content || '';
    var m = c.match(/【分类】(.+)/);
    return m ? m[1].trim() : '';
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
  /* ---- AI 丰富建议：分析现有世界观 → 建议可丰富方向 → 预览 → 一键保存 ---- */
  _suggestBlocks: [],
  suggest: async function () {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    var worlds = Store.state.worldSettings || [];
    if (!worlds.length) { UI.toast('还没有世界观设定，先创建一条或使用「🤖 自动导入」', 'warn'); return; }
    var existing = worlds.map(function (w) {
      return '《' + (w.name || '未命名') + '》' + (w.category ? '（' + w.category + '）' : '') + '\n' + (w.description || w.content || '');
    }).join('\n\n');
    // 风格参考：当前选中的文风样本（最多 2 条，各取前 200 字）
    var styleRef = '';
    try {
      var sel = Store.state.selection && Store.state.selection.styleSamples;
      var samples = Store.state.styleSamples || [];
      if (sel && samples.length) {
        styleRef = samples.filter(function (s) { return sel.has(s.id); }).slice(0, 2)
          .map(function (s) { return s.title + '：' + (s.content || '').slice(0, 200); }).join('\n');
      }
    } catch (e) {}
    if (!styleRef) styleRef = '（未选择文风样本，请根据已有世界观与作品基调推断风格）';
    UI.toast('AI 正在分析世界观并给出丰富建议…', '');
    try {
      var r = await API.post('/api/tools/execute', { tool: 'world_enhance', content: existing, params: { instruction: styleRef } });
      var text = (r && (r.result || r.text)) || '';
      WorldPage._showSuggestions(text);
    } catch (e) { UI.toast('建议生成失败: ' + e.message, 'error'); }
  },
  _showSuggestions: function (text) {
    var blocks = text.split(/【方向\s*\d+】/).map(function (s) { return s.trim(); }).filter(function (s) { return s.length > 0; });
    if (!blocks.length) { UI.toast('AI 未返回有效建议，请重试', 'warn'); return; }
    WorldPage._suggestBlocks = blocks;
    var html = '<div style="max-height:60vh;overflow-y:auto;display:flex;flex-direction:column;gap:10px;padding-right:4px">';
    blocks.forEach(function (b, i) {
      var title = b.split('\n')[0] || ('方向 ' + (i + 1));
      var reason = (b.match(/【理由】\s*([\s\S]*?)(?=【预览样本】|$)/) || [])[1] || '';
      var sample = (b.match(/【预览样本】\s*([\s\S]*)$/) || [])[1] || '';
      if (!sample) sample = b.split('\n').slice(1).join('\n');
      html += '<div class="world-suggest-card" id="wsCard' + i + '" style="border:1px solid var(--border2);border-radius:8px;padding:10px;background:var(--panel2)">';
      html += '<div style="font-weight:600;font-size:13px;margin-bottom:4px">✨ ' + esc(title) + '</div>';
      if (reason) html += '<div style="font-size:11.5px;color:var(--muted);margin-bottom:6px">💡 ' + esc(reason) + '</div>';
      html += '<div style="font-size:12px;line-height:1.7;white-space:pre-wrap;background:var(--panel);border-radius:6px;padding:8px;margin-bottom:8px">' + esc(sample) + '</div>';
      html += '<button class="btn btn-primary btn-sm" onclick="WorldPage.saveSuggestion(' + i + ')">💾 保存这条</button> ';
      html += '<button class="btn btn-ghost btn-sm" onclick="WorldPage.discardSuggestion(' + i + ')">忽略</button>';
      html += '</div>';
    });
    html += '</div>';
    UI.modal({
      title: '✨ 世界观丰富建议（点击「保存这条」直接入库）',
      wide: '640px',
      body: html,
      actions: [{ id: 'ok', label: '关闭', cls: 'btn-primary' }]
    });
  },
  saveSuggestion: function (idx) {
    var b = WorldPage._suggestBlocks[idx];
    if (!b) { UI.toast('建议不存在', 'warn'); return; }
    var title = (b.split('\n')[0] || '').trim() || ('世界观设定 ' + (idx + 1));
    var sample = (b.match(/【预览样本】\s*([\s\S]*)$/) || [])[1] || b.split('\n').slice(1).join('\n');
    sample = sample.trim();
    if (!sample) { UI.toast('该建议没有可保存的内容', 'warn'); return; }
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    UI.confirm('保存这条设定', '将保存为世界观条目：<br><b>' + esc(title) + '</b>', async function () {
      try {
        await API.createWorldSetting({ project_id: p.id, title: title, content: sample });
        UI.toast('✅ 已保存', 'success');
        var card = document.getElementById('wsCard' + idx);
        if (card) card.style.display = 'none';
        WorldPage.load();
      } catch (e) { UI.toast('保存失败: ' + e.message, 'error'); }
    });
  },
  discardSuggestion: function (idx) {
    var card = document.getElementById('wsCard' + idx);
    if (card) card.style.display = 'none';
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
  },
  /* ============ 4-tab：设定 / 势力 / 地点 / 时间线（2026-08-05 转型纯作家辅助） ============ */
  curTab: 'setting',
  switchTab: function (tab) {
    this.curTab = tab;
    document.querySelectorAll('#worldTabs .world-tab').forEach(function (el) {
      el.classList.toggle('active', el.getAttribute('data-tab') === tab);
    });
    var show = { setting: 'worldGrid', faction: 'factionGrid', location: 'locationGrid', timeline: 'timelineList' }[tab];
    ['worldGrid', 'factionGrid', 'locationGrid', 'timelineList'].forEach(function (id) {
      document.getElementById(id).style.display = (id === show) ? '' : 'none';
    });
    // 头部按钮随 tab 切换
    var acts = document.getElementById('worldHeadActs');
    var headHtml = '';
    if (tab === 'setting') {
      headHtml = '<button class="btn btn-primary btn-sm" onclick="WorldPage.showCreate()">＋ 新建设定</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="ResourceUI.autoImportSettings()" title="AI 通读全部章节自动提取人物卡/世界观">🤖 自动导入</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="WorldPage.suggest()" title="AI 建议可丰富方向">✨ 丰富建议</button>' +
        '<span class="spacer" style="flex:1"></span>' +
        '<input type="text" class="page-search" id="worldSearch" placeholder="🔍 搜索设定" oninput="WorldPage.render()">';
    } else if (tab === 'faction') {
      headHtml = '<button class="btn btn-primary btn-sm" onclick="WorldPage.editFaction(null)">＋ 新势力</button><span class="spacer" style="flex:1"></span>';
    } else if (tab === 'location') {
      headHtml = '<button class="btn btn-primary btn-sm" onclick="WorldPage.editLocation(null)">＋ 新地点</button><span class="spacer" style="flex:1"></span>';
    } else if (tab === 'timeline') {
      headHtml = '<button class="btn btn-primary btn-sm" onclick="WorldPage.editTimelineEvent(null)">＋ 新事件</button><span class="spacer" style="flex:1"></span>';
    }
    acts.innerHTML = headHtml;
    if (tab === 'faction') this.renderFactions();
    if (tab === 'location') this.renderLocations();
    if (tab === 'timeline') this.renderTimeline();
  },
  load: async function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty('请先在左侧选中一个项目'); return; }
    try {
      Store.state.worldSettings = await API.listWorldSettings(p.id);
      Store.state.factions = await API.listFactions(p.id);
      Store.state.locations = await API.listLocations(p.id);
      Store.state.timelineEvents = await API.listTimeline(p.id);
      this.render();
      if (this.curTab === 'faction') this.renderFactions();
      if (this.curTab === 'location') this.renderLocations();
      if (this.curTab === 'timeline') this.renderTimeline();
    } catch (e) {
      document.getElementById('worldGrid').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>加载失败</div></div>';
    }
  },
  /* ---- 势力 ---- */
  renderFactions: function () {
    var grid = document.getElementById('factionGrid');
    var items = Store.state.factions || [];
    if (!items.length) {
      grid.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚔️</div><div>暂无势力/组织，点击「＋ 新势力」创建</div></div>';
      return;
    }
    var html = '';
    items.forEach(function (f) {
      html += '<div class="world-card" onclick="WorldPage.editFaction(\'' + f.id + '\')">';
      html += '<div class="world-card-icon">⚔️</div><div class="world-card-body">';
      html += '<div class="world-card-name">' + esc(f.name) + '</div>';
      if (f.leader) html += '<span class="world-tag">👑 ' + esc(f.leader) + '</span>';
      var d = f.description || '';
      if (d.length > 100) d = d.substring(0, 100) + '...';
      html += '<div class="world-card-desc">' + esc(d) + '</div>';
      html += '</div><div class="world-card-acts">';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();WorldPage.editFaction(\'' + f.id + '\')" title="编辑">✏️</span>';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();WorldPage.delFaction(\'' + f.id + '\')" title="删除">🗑</span>';
      html += '</div></div>';
    });
    grid.innerHTML = html;
  },
  editFaction: function (id) {
    var f = null;
    (Store.state.factions || []).forEach(function (x) { if (x.id === id) f = x; });
    var isNew = !f;
    var html = '<div class="form-group"><label>名称 *</label><input id="facName" value="' + escAttr(f ? f.name : '') + '"></div>';
    html += '<div class="form-group"><label>首领/负责人</label><input id="facLeader" value="' + escAttr(f ? f.leader : '') + '" placeholder="可选"></div>';
    html += '<div class="form-group"><label>描述</label><textarea id="facDesc" rows="4" placeholder="组织目标、规模、行事风格...">' + esc(f ? f.description : '') + '</textarea></div>';
    html += '<div class="form-group"><label>主要成员</label><input id="facMembers" value="' + escAttr(f ? f.members : '') + '" placeholder="逗号分隔人名"></div>';
    html += '<div class="form-group"><label>与其他势力的关系</label><textarea id="facRelations" rows="2" placeholder="如：与皇族敌对 / 暗中支持主角">' + esc(f ? f.relations : '') + '</textarea></div>';
    var self = this;
    UI.modal({
      title: (isNew ? '新建' : '编辑') + '势力',
      body: html,
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: isNew ? '创建' : '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var name = document.getElementById('facName').value.trim();
          if (!name) { UI.toast('请输入名称', 'warn'); return; }
          ov.remove();
          self.saveFaction(id, {
            name: name, leader: document.getElementById('facLeader').value.trim(),
            description: document.getElementById('facDesc').value.trim(),
            members: document.getElementById('facMembers').value.trim(),
            relations: document.getElementById('facRelations').value.trim()
          });
        }}
      ]
    });
  },
  saveFaction: async function (id, data) {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    try {
      if (id) { await API.updateFaction(id, data); } else { await API.createFaction(Object.assign({ project_id: p.id }, data)); }
      UI.toast(id ? '已保存' : '已创建', 'success');
      Store.state.factions = await API.listFactions(p.id);
      this.renderFactions();
    } catch (e) { UI.toast('保存失败: ' + e.message, 'error'); }
  },
  delFaction: function (id) {
    var self = this;
    UI.confirm('删除势力', '确定删除此势力？', function () {
      API.deleteFaction(id).then(function () {
        UI.toast('已删除', 'success');
        Store.state.factions = [];
        self.load();
      }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  /* ---- 地点 ---- */
  renderLocations: function () {
    var grid = document.getElementById('locationGrid');
    var items = Store.state.locations || [];
    if (!items.length) {
      grid.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">📍</div><div>暂无地点，点击「＋ 新地点」创建</div></div>';
      return;
    }
    var html = '';
    items.forEach(function (l) {
      html += '<div class="world-card" onclick="WorldPage.editLocation(\'' + l.id + '\')">';
      html += '<div class="world-card-icon">📍</div><div class="world-card-body">';
      html += '<div class="world-card-name">' + esc(l.name) + '</div>';
      if (l.type) html += '<span class="world-tag">' + esc(l.type) + '</span>';
      var d = l.description || '';
      if (d.length > 100) d = d.substring(0, 100) + '...';
      html += '<div class="world-card-desc">' + esc(d) + '</div>';
      html += '</div><div class="world-card-acts">';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();WorldPage.editLocation(\'' + l.id + '\')" title="编辑">✏️</span>';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();WorldPage.delLocation(\'' + l.id + '\')" title="删除">🗑</span>';
      html += '</div></div>';
    });
    grid.innerHTML = html;
  },
  editLocation: function (id) {
    var l = null;
    (Store.state.locations || []).forEach(function (x) { if (x.id === id) l = x; });
    var isNew = !l;
    var html = '<div class="form-group"><label>名称 *</label><input id="locName" value="' + escAttr(l ? l.name : '') + '"></div>';
    html += '<div class="form-group"><label>类型</label><input id="locType" value="' + escAttr(l ? l.type : '') + '" placeholder="如：城市/学校/秘境/酒馆"></div>';
    html += '<div class="form-group"><label>描述</label><textarea id="locDesc" rows="4">' + esc(l ? l.description : '') + '</textarea></div>';
    html += '<div class="form-group"><label>关联</label><input id="locRelated" value="' + escAttr(l ? l.related : '') + '" placeholder="关联人物/势力/事件"></div>';
    var self = this;
    UI.modal({
      title: (isNew ? '新建' : '编辑') + '地点',
      body: html,
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: isNew ? '创建' : '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var name = document.getElementById('locName').value.trim();
          if (!name) { UI.toast('请输入名称', 'warn'); return; }
          ov.remove();
          self.saveLocation(id, {
            name: name, type: document.getElementById('locType').value.trim(),
            description: document.getElementById('locDesc').value.trim(),
            related: document.getElementById('locRelated').value.trim()
          });
        }}
      ]
    });
  },
  saveLocation: async function (id, data) {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    try {
      if (id) { await API.updateLocation(id, data); } else { await API.createLocation(Object.assign({ project_id: p.id }, data)); }
      UI.toast(id ? '已保存' : '已创建', 'success');
      Store.state.locations = await API.listLocations(p.id);
      this.renderLocations();
    } catch (e) { UI.toast('保存失败: ' + e.message, 'error'); }
  },
  delLocation: function (id) {
    var self = this;
    UI.confirm('删除地点', '确定删除此地？', function () {
      API.deleteLocation(id).then(function () {
        UI.toast('已删除', 'success');
        self.load();
      }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  /* ---- 时间线事件 ---- */
  renderTimeline: function () {
    var box = document.getElementById('timelineList');
    var items = Store.state.timelineEvents || [];
    if (!items.length) {
      box.innerHTML = '<div class="timeline-empty">暂无时间线事件，点击「＋ 新事件」录入重要剧情节点<br>（可填发生时间/对应章节/出场人物）</div>';
      return;
    }
    var chMap = {};
    (Store.state.chapters || []).forEach(function (c) { chMap[c.id] = c.title; });
    var html = '';
    items.forEach(function (t) {
      html += '<div class="timeline-item">';
      html += '<div class="timeline-time">' + esc(t.event_time || '—') + '</div>';
      html += '<div class="timeline-body">' + esc(t.event) + '</div>';
      html += '<div style="display:flex;flex-direction:column;align-items:flex-end;gap:2px">';
      if (t.chapter_id && chMap[t.chapter_id]) html += '<span class="world-tag">📄 ' + esc(chMap[t.chapter_id]) + '</span>';
      if (t.characters) html += '<span class="timeline-chars">👤 ' + esc(t.characters) + '</span>';
      html += '<div class="timeline-acts">';
      html += '<span class="char-act-btn" onclick="WorldPage.editTimelineEvent(\'' + t.id + '\')" title="编辑">✏️</span>';
      html += '<span class="char-act-btn" onclick="WorldPage.delTimelineEvent(\'' + t.id + '\')" title="删除">🗑</span>';
      html += '</div></div></div>';
    });
    box.innerHTML = html;
  },
  editTimelineEvent: function (id) {
    var t = null;
    (Store.state.timelineEvents || []).forEach(function (x) { if (x.id === id) t = x; });
    var isNew = !t;
    var chs = Store.state.chapters || [];
    var html = '<div class="form-group"><label>事件 *</label><textarea id="tlEvent" rows="3" placeholder="发生了什么事">' + esc(t ? t.event : '') + '</textarea></div>';
    html += '<div class="form-group"><label>发生时间</label><input id="tlTime" value="' + escAttr(t ? t.event_time : '') + '" placeholder="如：第1章 / 高一开学 / 三年前"></div>';
    html += '<div class="form-group"><label>关联章节</label><select id="tlChapter"><option value="">— 不关联 —</option>';
    chs.forEach(function (c) {
      html += '<option value="' + c.id + '"' + (t && t.chapter_id === c.id ? ' selected' : '') + '>' + esc(c.title || '未命名') + '</option>';
    });
    html += '</select></div>';
    html += '<div class="form-group"><label>出场人物</label><input id="tlChars" value="' + escAttr(t ? t.characters : '') + '" placeholder="逗号分隔"></div>';
    var self = this;
    UI.modal({
      title: (isNew ? '新建' : '编辑') + '时间线事件',
      body: html,
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: isNew ? '创建' : '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var ev = document.getElementById('tlEvent').value.trim();
          if (!ev) { UI.toast('请输入事件内容', 'warn'); return; }
          ov.remove();
          self.saveTimelineEvent(id, {
            event: ev,
            event_time: document.getElementById('tlTime').value.trim(),
            chapter_id: document.getElementById('tlChapter').value,
            characters: document.getElementById('tlChars').value.trim()
          });
        }}
      ]
    });
  },
  saveTimelineEvent: async function (id, data) {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    try {
      if (id) { await API.updateTimelineEvent(id, data); } else { await API.createTimelineEvent(Object.assign({ project_id: p.id }, data)); }
      UI.toast(id ? '已保存' : '已创建', 'success');
      Store.state.timelineEvents = await API.listTimeline(p.id);
      this.renderTimeline();
    } catch (e) { UI.toast('保存失败: ' + e.message, 'error'); }
  },
  delTimelineEvent: function (id) {
    var self = this;
    UI.confirm('删除事件', '确定删除此事件？', function () {
      API.deleteTimelineEvent(id).then(function () {
        UI.toast('已删除', 'success');
        self.load();
      }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  }
};
