/* ============ project.js：项目 CRUD ============ */
var ProjectUI = {
  loadAll: async function () {
    try {
      Store.state.projects = await API.listProjects();
      this.renderList();
      this.renderArchived();
    } catch (e) { UI.toast('加载项目失败：' + e.message, 'error'); }
  },
  getArchived: function () {
    try { return JSON.parse(localStorage.getItem('archivedProjects') || '[]'); } catch (e) { return []; }
  },
  isArchived: function (id) {
    return this.getArchived().indexOf(id) >= 0;
  },
  archive: function (id) {
    var list = this.getArchived();
    if (list.indexOf(id) < 0) list.push(id);
    try { localStorage.setItem('archivedProjects', JSON.stringify(list)); } catch (e) {}
    // 如果归档的是当前项目，清空编辑器
    if (Store.state.currentProject && Store.state.currentProject.id === id) {
      Store.state.currentProject = null;
      Store.state.chapters = [];
      Store.state.currentChapter = null;
      Editor.setContent('');
      document.getElementById('docTitle').textContent = '未选择稿件';
      if (Editor.updateEmptyGuide) Editor.updateEmptyGuide();
    }
    UI.toast('已归档，可在底部「📦 已归档」中恢复', 'info');
    this.renderList();
    this.renderArchived();
  },
  unarchive: function (id) {
    var list = this.getArchived().filter(function (x) { return x !== id; });
    try { localStorage.setItem('archivedProjects', JSON.stringify(list)); } catch (e) {}
    UI.toast('已恢复', 'success');
    this.renderList();
    this.renderArchived();
  },
  renderArchived: function () {
    var box = document.getElementById('archivedBox');
    if (!box) return;
    var ids = this.getArchived();
    var items = (Store.state.projects || []).filter(function (p) { return ids.indexOf(p.id) >= 0; });
    if (!items.length) { box.style.display = 'none'; box.innerHTML = ''; return; }
    box.style.display = '';
    box.innerHTML = '<div class="arch-head" onclick="ProjectUI.toggleArchived()">📦 已归档（' + items.length + '）<span class="arch-arrow">▾</span></div>' +
      '<div class="arch-body" id="archBody">' +
      items.map(function (p) {
        return '<div class="arch-item"><span class="arch-name" title="' + esc(p.name) + '">' + esc(p.name) + '</span>' +
          '<span class="arch-acts"><button class="tool-btn" style="font-size:10px;padding:1px 6px" onclick="event.stopPropagation();ProjectUI.unarchive(\'' + p.id + '\')">恢复</button></span></div>';
      }).join('') +
      '</div>';
  },
  toggleArchived: function () {
    var b = document.getElementById('archBody');
    if (b) b.style.display = b.style.display === 'none' ? '' : 'none';
  },
  renderList: function (filter) {
    var list = document.getElementById('novelList');
    var items = Store.state.projects;
    // 过滤已归档
    var archivedIds = this.getArchived();
    items = items.filter(function (p) { return archivedIds.indexOf(p.id) < 0; });
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
        '<img src="' + coverUrl + '" alt="" onerror="this.style.display=\'none\'" loading="lazy">' +
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
      // 恢复大纲
      if (d.item.outline) {
        Store.state.pipeline.outline = d.item.outline;
        Store.state.composer.outline = d.item.outline;
        var genOutline = document.getElementById('genOutline');
        if (genOutline && !genOutline.value) genOutline.value = d.item.outline;
      }
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
      // 刷新当前路由页面
      if (Router.current === 'characters' && typeof CharacterPage !== 'undefined') CharacterPage.load();
      if (Router.current === 'worldbuilding' && typeof WorldPage !== 'undefined') WorldPage.load();
      if (Router.current === 'outline' && typeof OutlinePage !== 'undefined') OutlinePage.load();
      if (Router.current === 'dashboard' && typeof DashboardPage !== 'undefined') DashboardPage.refresh();
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
        html += '<div class="ch-item" onclick="event.stopPropagation();' + (batchMode ? 'ChapterUI.batchToggle(\'' + c.id + '\')' : 'ChapterUI.selectChapter(Store.state.chapters.find(function(x){return x.id===\'' + c.id + '\'}))') + '" oncontextmenu="event.stopPropagation();return ChapterUI.showContextMenu(event,\'' + c.id + '\')">';
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
    document.getElementById('resProjName').textContent = '📁 ' + (p ? p.name : '') + ' · ' + totalWc.toLocaleString() + '字';
  },
  showCreate: function () {
    var idn = 'n_' + uid();
    var cats = ['玄幻','仙侠','都市','校园','言情','科幻','奇幻','历史','悬疑','轻小说','武侠','同人','短篇','其他'];
    var body = '<div style="text-align:center;margin-bottom:16px">';
    body += '<div style="font-size:40px;margin-bottom:8px">📖</div>';
    body += '<div style="font-size:13px;color:var(--muted)">创建新项目，开始你的创作之旅</div>';
    body += '</div>';
    body += '<div class="form-group"><label>📛 项目名称 <span style="color:var(--danger)">*</span></label><input id="' + idn + '_name" placeholder="例如：风起西陵" autofocus></div>';
    body += '<div class="form-group"><label>📂 作品类型</label><div class="create-cat-grid" id="' + idn + '_catGrid">';
    cats.forEach(function (cat) {
      body += '<span class="create-cat-chip" data-cat="' + cat + '" onclick="ProjectUI.selectCat(this)">' + cat + '</span>';
    });
    body += '</div><input type="hidden" id="' + idn + '_type" value=""></div>';
    body += '<div class="form-group"><label>📝 创作大纲（选填，AI 将协助完善）</label><textarea id="' + idn + '_outline" rows="4" placeholder="输入初始灵感或大纲，留空将由 AI 辅助构思..."></textarea></div>';
    body += '<div style="background:var(--accent-soft);border-radius:8px;padding:8px 10px;font-size:11px;color:var(--muted);margin-top:8px">💡 创建后可在编辑器中与 AI 协作，或前往「章节大纲」面板管理结构。</div>';
    UI.modal({
      title: '✦ 新建项目',
      body: body,
      width: '520px',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '✦ 创建项目', cls: 'btn-primary', onClick: function (m, ov) {
            var name = document.getElementById(idn + '_name').value.trim();
            var type = document.getElementById(idn + '_type').value;
            if (!name) { UI.toast('请输入项目名称', 'warn'); return; }
            var outline = (document.getElementById(idn + '_outline') || {}).value || '';
            ov.remove();
            ProjectUI.create(name, type).then(function () {
              if (outline) {
                Editor.setContent(outline);
                UI.toast('大纲已写入编辑器', 'success');
              }
            });
          }
        }
      ]
    });
  },
  selectCat: function (el) {
    var grid = el.parentElement;
    grid.querySelectorAll('.create-cat-chip').forEach(function (c) { c.classList.remove('active'); });
    el.classList.add('active');
    var input = grid.nextElementSibling;
    if (input) input.value = el.getAttribute('data-cat');
  },
  create: async function (name, type) {
    try {
      var p = await API.createProject(name, type);
      Store.state.projects.unshift(p);
      UI.toast('项目已创建', 'success');
      return this.select(p.id);
    } catch (e) { UI.toast('创建失败：' + e.message, 'error'); }
  },
  ctxMenu: function (e, id) {
    var p = Store.state.projects.find(function (x) { return x.id === id; });
    if (!p) return false;
    return UI.ctxMenu(e, [
      { id: 'open', label: '📖 打开', onClick: function () { ProjectUI.select(id); } },
      { id: 'rename', label: '✏️ 重命名', onClick: function () { ProjectUI.rename(id); } },
      { id: 'duplicate', label: '📋 复制项目', onClick: function () { ProjectUI.duplicate(id); } },
      { id: 'cover', label: '🎨 生成封面', onClick: function () { ProjectUI.generateCover(id); } },
      { divider: true },
      { id: 'archive', label: '📦 归档（隐藏）', onClick: function () { ProjectUI.archive(id); } },
      { id: 'del', label: '🗑 删除', danger: true, onClick: function () { ProjectUI.remove(id); } }
    ]);
  },
  // P3-2：封面加载失败时记录缺失并隐藏图片（保留文字降级）
  coverMissing: function (img, name) {
    img.style.display = 'none';
    try {
      var missing = JSON.parse(sessionStorage.getItem('missingCovers') || '[]');
      if (missing.indexOf(name) < 0) { missing.push(name); sessionStorage.setItem('missingCovers', JSON.stringify(missing)); }
    } catch (e) {}
  },

  generateCover: async function (id) {
    var p = Store.state.projects.find(function (x) { return x.id === id; });
    if (!p) return;
    // R2 修复：防重复点击（Pollinations 生成需 10s+，期间再次点击会重复消耗）
    if (ProjectUI._coverGenerating) { UI.toast('封面正在生成中，请稍候…', 'warn'); return; }
    ProjectUI._coverGenerating = true;
    UI.toast('正在为「' + p.name + '」生成封面（约需 10-20 秒）…', '');
    try {
      var r = await fetch('/api/projects/' + id + '/cover', { method: 'POST' });
      var d = await r.json();
      if (!r.ok) throw new Error(d.error || '生成失败');
      // 刷新封面（加时间戳破缓存）
      var imgs = document.querySelectorAll('.novel-item img[src*="' + encodeURIComponent(p.name).substring(0, 10) + '"]');
      imgs.forEach(function (img) {
        img.src = d.url + '?t=' + Date.now();
        img.style.display = '';
        img.onerror = function () { img.style.display = 'none'; };
      });
      ProjectUI.renderList();
      UI.toast('封面已生成', 'success');
    } catch (e) { UI.toast('封面生成失败：' + e.message, 'error'); }
    finally { ProjectUI._coverGenerating = false; }
  },
  duplicate: async function (id) {
    UI.toast('正在复制项目…', '');
    try {
      await API.duplicateProject(id);
      await ProjectUI.loadAll();
      UI.toast('项目已复制', 'success');
    } catch (e) { UI.toast('复制失败：' + e.message, 'error'); }
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
          if (Editor.updateEmptyGuide) Editor.updateEmptyGuide();
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
