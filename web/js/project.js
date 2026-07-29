/* ============ project.js：项目 CRUD ============ */
var ProjectUI = {
  loadAll: async function () {
    try {
      Store.state.projects = await API.listProjects();
      this.renderList();
    } catch (e) { UI.toast('加载项目失败：' + e.message, 'error'); }
  },
  renderList: function (filter) {
    var list = document.getElementById('novelList');
    var items = Store.state.projects;
    if (filter) {
      var f = filter.toLowerCase();
      items = items.filter(function (p) { return (p.name || '').toLowerCase().includes(f); });
    }
    if (!items.length) {
      list.innerHTML = '<div class="side-empty">' + (filter ? '未匹配到项目' : '暂无项目<br>点击上方按钮创建') + '</div>';
      return;
    }
    var cur = Store.state.currentProject;
    list.innerHTML = items.map(function (p) {
      var active = cur && cur.id === p.id;
      var color = projectColor(p.name);
      // 计算该书总字数（chapters可能未加载，用0占位）
      var pwc = 0;
      if (Store.state.currentProject && p.id === Store.state.currentProject.id && Store.state.chapters) {
        pwc = Store.state.chapters.reduce(function (s, c) { return s + (c.word_count || 0); }, 0);
      }
      var wcText = pwc ? pwc.toLocaleString() + '字' : '';
      var coverUrl = '/covers/' + encodeURIComponent(p.name) + '.png';
      var coverHtml = '<div class="novel-cover" style="background:' + color + '">' +
        '<img src="' + coverUrl + '" alt="" onerror="this.style.display=\'none\';this.parentElement.textContent=\'' + esc(p.name.charAt(0)) + '\'" loading="lazy">' +
        '<span class="cover-fallback">' + esc(p.name.charAt(0)) + '</span></div>';
      return '<div class="novel-item' + (active ? ' active expanded' : '') + '" onclick="ProjectUI.select(\'' + p.id + '\')" oncontextmenu="return ProjectUI.ctxMenu(event,\'' + p.id + '\')">' +
        coverHtml +
        '<div class="novel-info"><div class="title">' + esc(p.name) + '</div>' +
        '<div class="meta">' + esc(p.type || '未分类') + (wcText ? ' · ' + wcText : '') + '</div></div>' +
        '<div class="row-del" onclick="event.stopPropagation();ProjectUI.remove(\'' + p.id + '\')" title="删除">✕</div>' +
        '<div class="novel-expand" id="expand-' + p.id + '"></div>' +
        '</div>';
    }).join('');
  },
  select: async function (id) {
    // 防重入：正在加载中禁止重复点击
    if (this._loading === id) return;
    if (SSE.active) { UI.toast('正在进行 AI 生成，请先终止后再切换项目', 'warn'); return; }
    this._loading = id;
    try {
      // 重置批量模式
      ChapterUI.batchMode = false;
      ChapterUI.batchSelected = {};
      // 先清空编辑器，避免残留旧内容造成"数据丢失"错觉
      Editor.setContent('');
      Store.state.currentChapter = null;
      var d = await API.getProject(id);
      Store.state.currentProject = d.item;
      Store.set('lastProjectId', d.item.id);
      Store.loadSelection(id);
      var results = await Promise.all([
        API.listVersions(id),
        API.listCharacters(id),
        API.listWorldSettings(id),
        API.listMaterials(id),
        API.listVolumes(id),
        API.listChapters(id)
      ]);
      Store.state.versions = results[0];
      Store.state.characters = results[1];
      Store.state.worldSettings = results[2];
      Store.state.materials = results[3];
      Store.state.volumes = results[4];
      Store.state.chapters = results[5];
      Store.state.latestVersion = results[0][0] || null;
      // 自动选择第一个章节
      if (Store.state.chapters.length && !Store.state.currentChapter) {
        ChapterUI.selectChapter(Store.state.chapters[0]);
      } else if (!Store.state.chapters.length) {
        Store.state.currentChapter = null;
      }
      this.renderList();
      this.renderExpanded(id);
      ChapterUI.renderTree();
      Sidebar.renderResources();
      if (!Store.state.currentChapter) Editor.loadLatest();
      RightPanel.renderContext();
      PipelineUI.reset();
      document.getElementById('docTitle').textContent = d.item.name;
      this.updateMeta();
      this._loading = null;
    } catch (e) { this._loading = null; UI.toast('加载项目失败：' + e.message, 'error'); }
  },
  renderExpanded: function (id) {
    var el = document.getElementById('expand-' + id);
    if (!el) return;
    var chapters = Store.state.chapters || [];
    var batchMode = ChapterUI.batchMode || false;
    var html = '';
    html += '<div class="ch-actions">';
    html += '<button onclick="event.stopPropagation();ProjectUI.startBatch()">' + (batchMode ? '☑ 批量' : '☐ 批量') + '</button>';
    html += '<button onclick="event.stopPropagation();ChapterUI.addChapter()">＋ 章</button>';
    html += '<button onclick="event.stopPropagation();ChapterUI.addVolume()">＋ 卷</button>';
    html += '<button onclick="event.stopPropagation();ChapterUI.continueNextChapter()">▶ 续写</button>';
    html += '<button onclick="event.stopPropagation();Editor.importFile()">📥 导入</button>';
    html += '<button onclick="event.stopPropagation();Editor.exportFile()">📤 导出</button>';
    html += '</div>';
    if (batchMode && chapters.length) {
      html += '<div class="ch-batch-bar" style="display:flex;gap:6px;padding:2px 0 6px 0;font-size:11px;align-items:center;flex-wrap:wrap">';
      html += '<span onclick="event.stopPropagation();ChapterUI.batchSelectAll()" style="cursor:pointer;color:var(--accent);font-size:11px">全选</span>';
      html += '<span onclick="event.stopPropagation();ChapterUI.batchClear()" style="cursor:pointer;color:var(--muted);font-size:11px">清除</span>';
      html += '<span id="batchCount" style="color:var(--muted);font-size:11px;margin-left:4px">已选 ' + Object.keys(ChapterUI.batchSelected || {}).length + ' 章</span>';
      html += '<span style="flex:1"></span>';
      html += '<button onclick="event.stopPropagation();ChapterUI.batchExportTXT()" class="tool-btn" style="font-size:10px;padding:2px 6px">📤 TXT</button>';
      html += '<button onclick="event.stopPropagation();ChapterUI.batchExportMD()" class="tool-btn" style="font-size:10px;padding:2px 6px">📤 MD</button>';
      html += '<button onclick="event.stopPropagation();ChapterUI.batchDelete()" class="tool-btn danger" style="font-size:10px;padding:2px 6px">🗑 删除</button>';
      html += '</div>';
    }
    if (chapters.length) {
      html += '<div class="ch-list">';
      chapters.forEach(function (c) {
        var checked = batchMode && ChapterUI.batchSelected && ChapterUI.batchSelected[c.id];
        html += '<div class="ch-item" onclick="event.stopPropagation();' + (batchMode ? 'ChapterUI.batchToggle(\'' + c.id + '\')' : 'ChapterUI.selectChapter(Store.state.chapters.find(function(x){return x.id===\'' + c.id + '\'}))') + '">';
        if (batchMode) html += '<span class="ch-cb">' + (checked ? '☑' : '☐') + '</span>';
        html += '📄 ' + esc(c.title || '未命名');
        html += '<span class="wc">' + (c.word_count || 0).toLocaleString() + '字</span>';
        html += '</div>';
      });
      html += '</div>';
    }
    el.innerHTML = html;
  },

  showImport: function () { if (document.querySelector('.topbar .btn-import')) document.querySelector('.topbar .btn-import').click(); },
  showExport: function () { if (document.querySelector('.topbar .btn-export')) document.querySelector('.topbar .btn-export').click(); },
  startBatch: function () {
    ChapterUI.batchMode = !ChapterUI.batchMode;
    ChapterUI.batchSelected = {};
    var p = Store.state.currentProject;
    if (p) this.renderExpanded(p.id);
  },

  updateMeta: function () {
    var p = Store.state.currentProject;
    var v = Store.state.latestVersion;
    var ch = Store.state.currentChapter;
    var wc = ch ? ch.word_count : (v ? wordCount(v.content) : 0);
    // 全书总字数（从章节汇总）
    var totalWc = 0;
    if (Store.state.chapters && Store.state.chapters.length) {
      totalWc = Store.state.chapters.reduce(function (sum, c) { return sum + (c.word_count || 0); }, 0);
    }
    document.getElementById('docMeta').textContent =
      (ch ? ch.title + ' · ' : '') + wc.toLocaleString() + '字' +
      ' | 全书 ' + totalWc.toLocaleString() + '字' +
      ' · ' + Store.state.characters.length + '人物 ' + Store.state.worldSettings.length + '世界观 ' + Store.state.materials.length + '素材' +
      ' · ' + Store.state.chapters.length + '章节';
    document.getElementById('resProjName').textContent = (p ? p.name : '') + ' · ' + totalWc.toLocaleString() + '字';
  },
  showCreate: function () {
    var idn = 'n_' + uid();
    UI.modal({
      title: '新建项目',
      body: '<div class="form-group"><label>项目名称 *</label><input id="' + idn + '_name" placeholder="例如：风起西陵"></div>' +
        '<div class="form-group"><label>类型（可选）</label><select id="' + idn + '_type"><option value="">选择类型</option><option>玄幻</option><option>仙侠</option><option>都市</option><option>校园</option><option>言情</option><option>科幻</option><option>奇幻</option><option>历史</option><option>悬疑</option><option>轻小说</option><option>武侠</option><option>同人</option><option>短篇</option><option>其他</option></select></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '创建', cls: 'btn-primary', onClick: function (m, ov) {
            var name = document.getElementById(idn + '_name').value.trim();
            var type = document.getElementById(idn + '_type').value;
            if (!name) { UI.toast('请输入项目名称', 'warn'); return; }
            ov.remove();
            ProjectUI.create(name, type);
          }
        }
      ]
    });
  },
  create: async function (name, type) {
    try {
      var p = await API.createProject(name, type);
      Store.state.projects.unshift(p);
      UI.toast('项目已创建', 'success');
      this.select(p.id);
    } catch (e) { UI.toast('创建失败：' + e.message, 'error'); }
  },
  ctxMenu: function (e, id) {
    var p = Store.state.projects.find(function (x) { return x.id === id; });
    if (!p) return false;
    return UI.ctxMenu(e, [
      { id: 'open', label: '📖 打开', onClick: function () { ProjectUI.select(id); } },
      { id: 'rename', label: '✏️ 重命名', onClick: function () { ProjectUI.rename(id); } },
      { divider: true },
      { id: 'del', label: '🗑 删除', danger: true, onClick: function () { ProjectUI.remove(id); } }
    ]);
  },
  rename: function (id) {
    var p = Store.state.projects.find(function (x) { return x.id === id; });
    if (!p) return;
    UI.prompt('重命名项目', '项目名称', p.name, async function (name) {
      if (!name) return;
      try {
        var np = await API.updateProject(id, name, p.type || undefined);
        Object.assign(p, np);
        if (Store.state.currentProject && Store.state.currentProject.id === id) {
          Store.state.currentProject = np;
          document.getElementById('docTitle').textContent = np.name;
          ProjectUI.updateMeta();
        }
        ProjectUI.renderList();
        UI.toast('已重命名', 'success');
      } catch (e) { UI.toast('重命名失败：' + e.message, 'error'); }
    });
  },
  remove: function (id) {
    var p = Store.state.projects.find(function (x) { return x.id === id; });
    if (!p) return;
    UI.confirm('删除项目', '确认删除「' + p.name + '」？该项目下全部稿件、人物卡、世界观、素材将一并删除，不可恢复。', async function () {
      try {
        await API.deleteProject(id);
        Store.state.projects = Store.state.projects.filter(function (x) { return x.id !== id; });
        if (Store.state.currentProject && Store.state.currentProject.id === id) {
          Store.state.currentProject = null;
          Store.state.versions = []; Store.state.characters = []; Store.state.worldSettings = []; Store.state.materials = [];
          Store.state.volumes = []; Store.state.chapters = []; Store.state.currentChapter = null;
          Store.state.latestVersion = null;
          Editor.setContent('');
          document.getElementById('docTitle').textContent = '未选择稿件';
          document.getElementById('docMeta').textContent = '';
          document.getElementById('resourceSection').classList.remove('show');
        }
        ProjectUI.renderList();
        UI.toast('项目已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  }
};

// 根据书名生成固定封面颜色
function projectColor(name) {
  var colors = ['#6366f1','#8b5cf6','#0ea5e9','#10b981','#f59e0b','#ef4444','#ec4899','#14b8a6','#f97316','#84cc16'];
  var hash = 0;
  for (var i = 0; i < (name || '').length; i++) { hash = name.charCodeAt(i) + ((hash << 5) - hash); }
  return colors[Math.abs(hash) % colors.length];
}
