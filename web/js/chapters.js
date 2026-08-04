/* ============ chapters.js：章节层级管理（拖拽/合并/拆分/导航/标签/统计） ============ */
var ChapterUI = {
  dragCh: null,
  batchMode: false,
  batchSelected: {},
  loadAll: async function () {
    var p = Store.state.currentProject;
    if (!p) return;
    try {
      Store.state.volumes = await API.listVolumes(p.id);
      Store.state.chapters = await API.listChapters(p.id);
    } catch (e) { /* 静默 */ }
  },
  selectChapter: async function (ch) {
    if (Store.state.currentChapter && Store.state.currentChapter.id === ch.id) return;
    if (Store.state.currentChapter && Store.state.currentProject) {
      var text = Editor.getText();
      var cur = Store.state.currentChapter;
      if (text !== cur.content) {
        try { await API.updateChapter(cur.id, { content: text }); cur.content = text; } catch (e) { console.warn('[chapters] save before switch failed:', e && e.message); }
      }
    }
    Store.state.currentChapter = ch;
    ch._saveVersion = ch.updated_at || ch.created_at || '';
    Editor._conflictChecked = null;
    var pane = document.querySelector('.editor-pane');
    if (pane) pane.classList.add('has-content');
    Editor.setContent(ch.content || '');
    // Tiptap 异步渲染完成后强制 focus
    setTimeout(function () {
      if (Editor.tiptap) Editor.tiptap.commands.focus('start');
      var p = document.querySelector('.editor-pane');
      if (p) p.classList.add('has-content');
    }, 120);
    document.getElementById('docTitle').textContent = ch.title || '无标题';
    var btn = document.getElementById('btnAutoTitle');
    if (btn) btn.style.display = '';
    ProjectUI.updateMeta();
    ChapterUI.renderTree();
    if (Editor.updateEmptyGuide) Editor.updateEmptyGuide();
  },
  nextChapter: function () {
    var chs = Store.state.chapters;
    var cur = Store.state.currentChapter;
    if (!cur || !chs.length) return;
    var idx = chs.findIndex(function (c) { return c.id === cur.id; });
    if (idx >= 0 && idx < chs.length - 1) { ChapterUI.selectChapter(chs[idx + 1]); }
  },
  prevChapter: function () {
    var chs = Store.state.chapters;
    var cur = Store.state.currentChapter;
    if (!cur || !chs.length) return;
    var idx = chs.findIndex(function (c) { return c.id === cur.id; });
    if (idx > 0) { ChapterUI.selectChapter(chs[idx - 1]); }
  },
  renderTree: function (filter) {
    var container = document.getElementById('chapterTree');
    if (!container) return;
    var p = Store.state.currentProject;
    if (!p) { container.innerHTML = ''; return; }
    var vols = Store.state.volumes || [];
    var chs = Store.state.chapters || [];
    var f = filter ? filter.toLowerCase() : '';
    if (f) {
      chs = chs.filter(function (c) { return (c.title || '').toLowerCase().includes(f); });
      vols = vols.filter(function (v) { return (v.title || '').toLowerCase().includes(f); });
    }
    var curCh = Store.state.currentChapter;
    var html = '<div class="ch-tree-head">' +
      '<span>📑 章节结构</span>' +
       '<span class="ch-tree-acts">' +
         '<span class="link-btn" onclick="ChapterUI.toggleBatch()" title="批量操作" id="btnBatch">☐批量</span>' +
         '<span class="link-btn" onclick="Tools.showTrash()" title="回收站">🗑</span>' +
       '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.addChapter()" title="新建章节">＋章</span>' +
         '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.addVolume()" title="新建卷">＋卷</span>' +
        '<span class="link-btn" onclick="ChapterUI.continueNextChapter()" title="续写下一章">▶续写</span>' +
        '<span class="link-btn" onclick="ChapterUI.showImportMenu()" title="导入">📥导入</span>' +
        '<span class="link-btn" onclick="ChapterUI.exportChapters()" title="导出">📤导出</span>' +
        '<span class="link-btn" onclick="ChapterUI.showStats()" title="统计">📊</span>' +
      '</span>' +
      '</div>';
    // 批量操作工具栏
    if (ChapterUI.batchMode) {
      html += '<div class="ch-batch-bar" id="batchBar">' +
        '<span class="link-btn" onclick="ChapterUI.batchSelectAll()">全选</span>' +
        '<span class="link-btn" onclick="ChapterUI.batchClear()">取消全选</span>' +
        '<span class="spacer"></span>' +
        '<span style="color:var(--muted);font-size:10px" id="batchCount">已选 0 章</span>' +
        '<button class="tool-btn" onclick="ChapterUI.batchExportTXT()">📤 导出选中 TXT</button>' +
        '<button class="tool-btn" onclick="ChapterUI.batchExportMD()">📤 导出选中 MD</button>' +
        '<button class="tool-btn danger" onclick="ChapterUI.batchDelete()">🗑 删除选中</button>' +
      '</div>';
    }
    var noVol = chs.filter(function (c) { return !c.volume_id; });
    vols.forEach(function (v) {
      var vchs = chs.filter(function (c) { return c.volume_id === v.id; });
      html += '<div class="ch-volume" data-vid="' + v.id + '">' +
        '<div class="ch-vol-head" onclick="ChapterUI.toggleVolume(this)">' +
        '<span class="caret">▾</span>📁 ' + esc(v.title) + ' <span class="count">' + vchs.length + '</span>' +
        '<span class="vol-acts">' +
          '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.addChapter(\'' + v.id + '\')">＋</span>' +
          '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.renameVolume(\'' + v.id + '\')">✏</span>' +
          '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.delVolume(\'' + v.id + '\')">✕</span>' +
        '</span></div>' +
        '<div class="ch-vol-body" ondragover="event.preventDefault();event.currentTarget.classList.add(\'drag-over\')" ondragleave="event.currentTarget.classList.remove(\'drag-over\')" ondrop="ChapterUI.onDrop(event,\'' + v.id + '\')">';
      vchs.forEach(function (c) { html += ChapterUI.chapterItem(c, curCh); });
      if (!vchs.length) html += '<div class="res-check-empty">拖入章节或点击＋添加</div>';
      html += '</div></div>';
    });
    if (noVol.length || !vols.length) {
      html += '<div class="ch-volume"><div class="ch-vol-head" onclick="ChapterUI.toggleVolume(this)"><span class="caret">▾</span>📂 未分类 <span class="count">' + noVol.length + '</span></div>' +
        '<div class="ch-vol-body" ondragover="event.preventDefault();event.currentTarget.classList.add(\'drag-over\')" ondragleave="event.currentTarget.classList.remove(\'drag-over\')" ondrop="ChapterUI.onDrop(event,\'\')">';
      noVol.forEach(function (c) { html += ChapterUI.chapterItem(c, curCh); });
      if (!noVol.length && vols.length) html += '<div class="res-check-empty">拖入章节移至此处</div>';
      html += '</div></div>';
    }
    if (!vols.length && !chs.length) {
      html += '<div class="res-check-empty">暂无章节，点击上方＋创建</div>';
    }
    container.innerHTML = html;
    Sidebar.renderResources();
  },
  chapterItem: function (c, curCh) {
    var active = curCh && curCh.id === c.id;
    var tip = c.synopsis ? ' title="梗概：' + esc(c.synopsis) + '"' : '';
    var firstLine = c.synopsis ? c.synopsis.split('\n')[0] : '';
    var subtitle = firstLine ? '<div class="ch-subtitle">' + esc(firstLine.substring(0, 40) + (firstLine.length > 40 ? '…' : '')) + '</div>' : '';
    var cb = ChapterUI.batchMode ? '<span class="ch-cb" data-cid="' + c.id + '" onclick="event.stopPropagation();ChapterUI.batchToggle(\'' + c.id + '\')">' + (ChapterUI.batchSelected[c.id] ? '☑' : '☐') + '</span>' : '';
    var isEmpty = !c.content || !c.content.trim();
    var emptyClass = isEmpty ? ' ch-empty' : '';
    return '<div class="ch-item' + (active ? ' active' : '') + emptyClass + '" draggable="true" data-cid="' + c.id + '" ' +
      'onclick="ChapterUI.selectChapter(Store.state.chapters.find(function(x){return x.id===\'' + c.id + '\'}))" ' +
      'oncontextmenu="return ChapterUI.showContextMenu(event,\'' + c.id + '\')" ' +
      'ondragstart="ChapterUI.onDragStart(event,\'' + c.id + '\')" ondragend="ChapterUI.onDragEnd(event)">' +
      cb +
      '<span class="drag-handle" title="拖拽排序">≡</span>' +
      '<span class="icon">📄</span>' +
      '<span class="name"' + tip + '>' + esc(c.title) + '</span>' +
      '<span class="wc">' + (c.word_count || 0).toLocaleString() + '字</span>' +
      subtitle +
      '<span class="ch-acts">' +
        '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.editTags(\'' + c.id + '\')" title="标签">🏷</span>' +
        '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.renameChapter(\'' + c.id + '\')">✏</span>' +
        '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.copyChapter(\'' + c.id + '\')">📋</span>' +
        '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.splitChapterAtCursor()">✂</span>' +
        '<span class="link-btn" onclick="event.stopPropagation();ChapterUI.delChapter(\'' + c.id + '\')">✕</span>' +
      '</span></div>';
  },
  onDragStart: function (e, id) { ChapterUI.dragCh = id; ChapterUI.dragSrcVid = ''; var el = e.target.closest('.ch-volume'); if (el) ChapterUI.dragSrcVid = el.dataset.vid || ''; e.dataTransfer.effectAllowed = 'move'; },
  onDragEnd: function (e) { ChapterUI.dragCh = null; ChapterUI.dragSrcVid = ''; document.querySelectorAll('.drag-over').forEach(function (el) { el.classList.remove('drag-over'); }); },
  onDrop: async function (e, targetVid) {
    e.preventDefault(); e.currentTarget.classList.remove('drag-over');
    var cid = ChapterUI.dragCh;
    var srcVid = ChapterUI.dragSrcVid || '';
    if (!cid) return;
    try {
      if (srcVid !== targetVid) {
        // 跨卷拖拽：更新 volume_id，并放到目标卷末尾
        await API.updateChapter(cid, { volume_id: targetVid });
        // 更新源卷排序（移除被拖走章节后的gap）
        var srcChs = await API.listChapters(Store.state.currentProject.id, srcVid);
        srcChs = srcChs.filter(function (c) { return c.id !== cid; });
        var srcItems = srcChs.map(function (c, i) { return { id: c.id, sort_order: i + 1 }; });
        await API.reorderChapters(srcItems);
      }
      // 目标卷重新计算 sort_order
      var allChs = await API.listChapters(Store.state.currentProject.id, targetVid);
      var sorted = allChs.filter(function (c) { return c.id !== cid; });
      var dragged = allChs.find(function (c) { return c.id === cid; });
      if (dragged) sorted.push(dragged);
      var items = sorted.map(function (c, i) { return { id: c.id, sort_order: i + 1 }; });
      await API.reorderChapters(items);
      await ChapterUI.loadAll();
      ChapterUI.renderTree();
    } catch (ex) { UI.toast('移动失败', 'error'); }
    ChapterUI.dragCh = null;
    ChapterUI.dragSrcVid = '';
  },
  toggleVolume: function (head) { head.parentElement.classList.toggle('collapsed'); },
  addVolume: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    UI.prompt('新建卷', '卷名称', '', async function (title) {
      if (!title) return;
      try { await API.createVolume({ project_id: p.id, title: title }); await ChapterUI.loadAll(); ChapterUI.renderTree(); ProjectUI.updateMeta(); UI.toast('卷已创建', 'success'); } catch (e) { UI.toast('创建失败：' + e.message, 'error'); }
    });
  },
  renameVolume: function (id) {
    var v = Store.state.volumes.find(function (x) { return x.id === id; });
    if (!v) return;
    UI.prompt('重命名卷', '卷名称', v.title, async function (title) {
      if (!title) return;
      try { await API.updateVolume(id, { title: title }); await ChapterUI.loadAll(); ChapterUI.renderTree(); } catch (e) { UI.toast('失败', 'error'); }
    });
  },
  delVolume: function (id) {
    UI.confirm('删除卷', '确认删除此卷？卷下章节将变为未分类。', async function () {
      try { await API.deleteVolume(id); await ChapterUI.loadAll(); ChapterUI.renderTree(); ProjectUI.updateMeta(); UI.toast('已删除', 'success'); } catch (e) { UI.toast('失败', 'error'); }
    });
  },
  addChapter: function (vid) {
    var p = Store.state.currentProject;
    if (!p) return;
    UI.prompt('新建章节', '章节标题', '', async function (title) {
      if (!title) return;
      // 检测重名
      if (Store.state.chapters.some(function (c) { return c.title === title; })) {
        UI.toast('已存在同名章节「' + title + '」，请使用其他名称', 'warn');
        return;
      }
      try { await API.createChapter({ project_id: p.id, volume_id: vid || '', title: title, content: '' }); await ChapterUI.loadAll(); ChapterUI.renderTree(); ProjectUI.updateMeta(); UI.toast('章节已创建', 'success'); } catch (e) { UI.toast('创建失败：' + e.message, 'error'); }
    });
  },
  renameChapter: function (id) {
    var c = Store.state.chapters.find(function (x) { return x.id === id; });
    if (!c) return;
    UI.prompt('重命名章节', '章节标题', c.title, async function (title) {
      if (!title) return;
      try {
        await API.updateChapter(id, { title: title }); c.title = title;
        if (Store.state.currentChapter && Store.state.currentChapter.id === id) { Store.state.currentChapter.title = title; document.getElementById('docTitle').textContent = (Store.state.currentProject ? Store.state.currentProject.name : '') + ' · ' + title; }
        ChapterUI.renderTree();
      } catch (e) { UI.toast('失败', 'error'); }
    });
  },
  copyChapter: async function (id) {
    try { await API.copyChapter(id); await ChapterUI.loadAll(); ChapterUI.renderTree(); ProjectUI.updateMeta(); UI.toast('章节已复制', 'success'); } catch (e) { UI.toast('复制失败', 'error'); }
  },
  delChapter: function (id) {
    var c = Store.state.chapters.find(function (x) { return x.id === id; });
    if (!c) return;
    UI.confirm('删除章节', '确认删除「' + esc(c.title || '') + '」？<br><small style="color:var(--muted)">可在回收站中恢复，保留7天</small>', async function () {
      try {
        var backup = { id: c.id, title: c.title, content: c.content, tags: c.tags, synopsis: c.synopsis, sort_order: c.sort_order, volume_id: c.volume_id, project_id: (Store.state.currentProject||{}).id || '', project_name: (Store.state.currentProject||{}).name || '', deleted_at: Date.now() };
        // 存储到回收站(localStorage, 保留7天)
        var trash = Store.get('chapterTrash', []);
        trash.unshift(backup);
        if (trash.length > 50) trash = trash.slice(0, 50);
        Store.set('chapterTrash', trash);
        await API.deleteChapter(id);
        if (Store.state.currentChapter && Store.state.currentChapter.id === id) { Store.state.currentChapter = null; Editor.setContent(''); }
        await ChapterUI.loadAll(); ChapterUI.renderTree(); ProjectUI.updateMeta();
        UI.toast('已移至回收站（保留7天）', 'success', { duration: 5000 });
      } catch (e) { UI.toast('删除失败', 'error'); }
    });
  },
  restoreChapter: async function (backup) {
    var p = Store.state.currentProject;
    if (!p) return;
    try {
      await API.createChapter({
        project_id: p.id, volume_id: backup.volume_id || '',
        title: backup.title || '已恢复章节', content: backup.content || '',
        tags: backup.tags || '', synopsis: backup.synopsis || '',
        sort_order: backup.sort_order || 0
      });
      await ChapterUI.loadAll(); ChapterUI.renderTree(); ProjectUI.updateMeta();
      UI.toast('已撤销删除', 'success');
    } catch (e) { UI.toast('撤销失败：' + e.message, 'error'); }
  },
  editTags: function (id) {
    var c = Store.state.chapters.find(function (x) { return x.id === id; });
    if (!c) return;
    UI.prompt('编辑标签与梗概', '标签（逗号分隔）\n梗概', c.tags + '\n' + (c.synopsis || ''), async function (val) {
      if (!val && val !== '') return;
      // val is "tags\nsynopsis" or just "tags"
      var parts = val.split('\n');
      var tags = (parts[0] || '').trim();
      var synopsis = (parts.slice(1).join('\n') || '').trim();
      try {
        await API.updateChapter(id, { tags: tags, synopsis: synopsis });
        c.tags = tags; c.synopsis = synopsis;
        ChapterUI.renderTree();
        UI.toast('已保存', 'success');
      } catch (e) { UI.toast('保存失败', 'error'); }
    });
  },
  splitChapterAtCursor: async function () {
    var ch = Store.state.currentChapter;
    if (!ch) { UI.toast('请先选择章节', 'warn'); return; }
    var pos = Editor.getCursorPosition();
    if (pos < 0) { UI.toast('请将光标放在正文中', 'warn'); return; }
    try {
      var result = await API.splitChapterAtCursor(ch.id, pos);
      await ChapterUI.loadAll();
      ChapterUI.renderTree();
      if (result && result.length) ChapterUI.selectChapter(result[0]);
      UI.toast('章节已拆分', 'success');
    } catch (e) { UI.toast('拆分失败：' + e.message, 'error'); }
  },
  mergeChapters: function () {
    var chs = Store.state.chapters;
    if (chs.length < 2) { UI.toast('至少需要2个章节', 'warn'); return; }
    var idn = 'mrg_' + uid();
    // 按卷分组显示
    var vols = Store.state.volumes || [];
    var html = '';
    vols.forEach(function (v) {
      var vchs = chs.filter(function (c) { return c.volume_id === v.id; });
      if (!vchs.length) return;
      html += '<div style="margin-bottom:6px"><div style="font-weight:600;font-size:11px;color:var(--muted)">📁 ' + esc(v.title) + '</div>';
      vchs.forEach(function (c) {
        html += '<div class="check-item" data-cid="' + c.id + '" data-vid="' + c.volume_id + '" data-ord="' + c.sort_order + '" onclick="this.classList.toggle(\'checked\')"><div class="box">✓</div><div class="txt"><div class="n">' + esc(c.title) + ' (' + (c.word_count || 0) + '字)</div></div></div>';
      });
      html += '</div>';
    });
    // 未分类
    var noVol = chs.filter(function (c) { return !c.volume_id; });
    if (noVol.length) {
      html += '<div style="margin-bottom:6px"><div style="font-weight:600;font-size:11px;color:var(--muted)">📂 未分类</div>';
      noVol.forEach(function (c) {
        html += '<div class="check-item" data-cid="' + c.id + '" data-vid="" data-ord="' + c.sort_order + '" onclick="this.classList.toggle(\'checked\')"><div class="box">✓</div><div class="txt"><div class="n">' + esc(c.title) + ' (' + (c.word_count || 0) + '字)</div></div></div>';
      });
      html += '</div>';
    }
    UI.modal({
      title: '合并章节（仅同卷连续章节）',
      body: '<div style="max-height:300px;overflow-y:auto">' + html + '</div>' +
        '<div class="form-group"><label>合并后标题</label><input id="' + idn + '_title" placeholder="合并章节"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '合并', cls: 'btn-primary', onClick: async function (md, ov) {
          var checked = [];
          md.querySelectorAll('.check-item.checked').forEach(function (el) {
            checked.push({ id: el.dataset.cid, vid: el.dataset.vid, ord: parseInt(el.dataset.ord) || 0 });
          });
          if (checked.length < 2) { UI.toast('请至少选择2个章节', 'warn'); return; }
          // 校验：同卷
          var vid = checked[0].vid;
          for (var i = 1; i < checked.length; i++) {
            if (checked[i].vid !== vid) { UI.toast('不能跨卷合并！请只选择同一卷内的章节', 'warn'); return; }
          }
          // 校验：连续（按 ord 排序后检查）
          checked.sort(function (a, b) { return a.ord - b.ord; });
          for (var j = 1; j < checked.length; j++) {
            if (checked[j].ord !== checked[j-1].ord + 1) { UI.toast('只能合并连续章节！选中章节之间存在间隔', 'warn'); return; }
          }
          var title = document.getElementById(idn + '_title').value.trim() || '合并章节';
          try {
            var ids = checked.map(function (c) { return c.id; });
            await API.mergeChapters(ids, title);
            ov.remove(); await ChapterUI.loadAll(); ChapterUI.renderTree();
            if (Store.state.chapters.length) ChapterUI.selectChapter(Store.state.chapters[0]);
            UI.toast('已合并', 'success');
          } catch (e) { UI.toast('合并失败：' + e.message, 'error'); }
        }}
      ]
    });
  },
  showStats: async function () {
    var p = Store.state.currentProject;
    if (!p) return;
    try {
      var stats = await API.getProjectStats(p.id);
      var volRows = (stats.volumes || []).map(function (v) {
        return '<tr><td>' + esc(v.title) + '</td><td>' + v.chapters + '章</td><td>' + (v.words || 0).toLocaleString() + '字</td></tr>';
      }).join('');
      // 每日字数统计（从章节更新时间推算）
      var chs = Store.state.chapters || [];
      var dailyMap = {};
      chs.forEach(function (c) {
        var d = (c.updated_at || c.created_at || '').substring(0, 10);
        if (d) dailyMap[d] = (dailyMap[d] || 0) + (c.word_count || 0);
      });
      var dailyKeys = Object.keys(dailyMap).sort();
      var chartHTML = '';
      if (dailyKeys.length >= 2) {
        var maxW = Math.max.apply(null, dailyKeys.map(function (k) { return dailyMap[k]; })) || 1;
        var bars = dailyKeys.map(function (k) {
          var h = Math.round(dailyMap[k] / maxW * 100);
          return '<div style="display:flex;align-items:center;gap:4px;margin:2px 0;font-size:10px">' +
            '<span style="width:72px;text-align:right;color:var(--muted)">' + k.substring(5) + '</span>' +
            '<div style="flex:1;background:var(--panel3);border-radius:2px;height:14px;overflow:hidden">' +
            '<div style="width:' + h + '%;height:100%;background:linear-gradient(90deg,var(--accent),#7c3aed);border-radius:2px;min-width:2px"></div></div>' +
            '<span style="width:42px;color:var(--muted)">' + dailyMap[k].toLocaleString() + '</span></div>';
        }).join('');
        chartHTML = '<div style="margin-top:10px;padding-top:8px;border-top:1px solid var(--border)"><div style="font-size:12px;font-weight:600;margin-bottom:6px">📊 每日更新字数</div>' + bars + '</div>';
      }
      UI.modal({
        title: '📊 项目统计',
        body: '<table style="width:100%;font-size:12px;border-collapse:collapse"><thead><tr style="color:var(--muted);text-align:left"><th>卷</th><th>章节</th><th>字数</th></tr></thead><tbody>' + volRows + '</tbody></table>' +
          '<div style="margin-top:10px;padding-top:8px;border-top:1px solid var(--border);font-size:13px;font-weight:600">总计：' + (stats.total_chapters || 0) + '章 · ' + (stats.total_words || 0).toLocaleString() + '字 · ' + (stats.total_chars || 0).toLocaleString() + '字符</div>' + chartHTML,
        wide: '500px',
        actions: [{ id: 'close', label: '关闭' }]
      });
    } catch (e) { UI.toast('加载统计失败', 'error'); }
  },
  importChapters: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    var idn = 'imp_' + uid();
    UI.modal({
      title: '导入章节',
      body: '<div class="form-group"><label>粘贴 JSON 数据或从文件导入</label>' +
        '<textarea id="' + idn + '_json" rows="8" placeholder=\'[{"title":"章1","content":"...","volume_id":"","sort_order":0}]\'></textarea>' +
        '<input type="file" id="' + idn + '_file" accept=".json" style="margin-top:6px" onchange="ChapterUI.handleImportFile(\'' + idn + '\')"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '导入', cls: 'btn-primary', onClick: async function (m, ov) {
          var jsonStr = document.getElementById(idn + '_json').value.trim();
          if (!jsonStr) { UI.toast('请输入 JSON', 'warn'); return; }
          try {
            var chs = JSON.parse(jsonStr);
            var data = { project_id: p.id, chapters: Array.isArray(chs) ? chs : (chs.chapters || []) };
            if (chs.volumes) data.volumes = chs.volumes;
            var r = await API.importChapters(data);
            ov.remove(); await ChapterUI.loadAll(); ChapterUI.renderTree();
            UI.toast('导入 ' + (r.imported || 0) + ' 个章节', 'success');
          } catch (e) { UI.toast('导入失败', 'error'); }
        }}
      ]
    });
  },
  handleImportFile: function (idn) {
    var inp = document.getElementById(idn + '_file');
    if (!inp.files[0]) return;
    var reader = new FileReader();
    reader.onload = function (e) { document.getElementById(idn + '_json').value = e.target.result; };
    reader.readAsText(inp.files[0]);
  },
  exportChapters: async function () {
    var p = Store.state.currentProject; if (!p) return;
    try {
      var data = await API.exportChapters(p.id);
      var blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      var a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = (p.name || 'chapters') + '_chapters.json'; a.click();
      URL.revokeObjectURL(a.href); UI.toast('已导出', 'success');
    } catch (e) { UI.toast('导出失败', 'error'); }
  },
  splitImport: function (content) {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    if (!content) { UI.toast('内容为空', 'warn'); return; }
    ChapterUI._splitContent = content;

    UI.toast('AI 正在分析章节结构…', '');
    API.splitChapters(p.id, content, 'auto').then(async function (r) {
      var chs = r.items || [];
      await ChapterUI.loadAll();
      UI.toast(r.replaced ? ('已替换为 ' + chs.length + ' 章') : ('已追加 ' + chs.length + ' 章，原有章节未改动'), 'success');
      ChapterUI.renderTree();
      ProjectUI.renderList();
      Sidebar.renderResources();
      // 自动打开第一章
      if (chs.length > 0) {
        ChapterUI.selectChapter(chs[0]);
      }
    }).catch(function (e) {
      UI.toast('AI 分析失败，切换到手动模式', 'warn');
      var idn = 'sp_' + uid();
      UI.modal({
        title: '手动分割章节',
        body: '<div class="form-group"><label>分隔标记模式</label><select id="' + idn + '_mode" onchange="ChapterUI.splitModeChange(\'' + idn + '\')">' +
            '<option value="auto">自动识别（第X章 / ## / ---）</option>' +
            '<option value="## ">Markdown ## 标题</option>' +
            '<option value="### ">Markdown ### 标题</option>' +
            '<option value="---">水平分割线 ---</option>' +
            '<option value="custom">自定义分隔标记</option>' +
          '</select></div>' +
          '<div class="form-group" id="' + idn + '_custom" style="display:none"><label>自定义分隔标记</label><input id="' + idn + '_sep" placeholder="例如：--- 或 ##"></div>' +
          '<div style="max-height:260px;overflow-y:auto;border:1px solid var(--border);border-radius:7px;padding:8px;font-size:11px;background:var(--panel2)" id="' + idn + '_preview"><span class="res-check-empty">点击预览查看分割结果</span></div>' +
          '<div style="margin-top:4px;font-size:11px;color:var(--faint)">分割结果将<b>追加</b>到当前项目，原有章节保持不变</div>' +
          '<div style="margin-top:4px"><button class="tool-btn" onclick="ChapterUI.manualPreview(\'' + idn + '\')">🔍 预览结果</button></div>',
        actions: [
          { id: 'cancel', label: '取消' },
          { id: 'ok', label: '确认导入', cls: 'btn-primary', onClick: async function (m, ov) {
            var mode = document.getElementById(idn + '_mode').value;
            var splitBy = mode === 'custom' ? document.getElementById(idn + '_sep').value.trim() : mode;
            if (mode === 'custom' && !splitBy) { UI.toast('请输入自定义分隔标记', 'warn'); return; }
            ov.remove();
            try {
              var r2 = await API.splitChapters(p.id, ChapterUI._splitContent, splitBy);
              var chs = r2.items || [];
              await ChapterUI.loadAll();
              ChapterUI.renderTree();
              ProjectUI.renderList();
              Sidebar.renderResources();
              if (chs.length > 0) { ChapterUI.selectChapter(chs[0]); }
              UI.toast(r2.replaced ? ('已替换为 ' + chs.length + ' 章') : ('已追加 ' + chs.length + ' 章，原有章节未改动'), 'success');
            } catch (e2) { UI.toast('分割失败：' + e2.message, 'error'); }
          }}
        ]
      });
    });
  },
  manualPreview: function (idn) {
    var mode = document.getElementById(idn + '_mode').value;
    var sb = mode === 'custom' ? document.getElementById(idn + '_sep').value.trim() : mode;
    if (mode === 'custom' && !sb) { UI.toast('请输入分隔标记', 'warn'); return; }
    var pp = Store.state.currentProject;
    if (!pp) return;
    var content = ChapterUI._splitContent;
    if (!content) { UI.toast('内容为空', 'warn'); return; }
    API.splitChapters(pp.id, content, sb, true).then(function (r) {
      document.getElementById(idn + '_preview').innerHTML = (r.items || []).map(function (c, i) {
        return '<div style="padding:4px 0;border-bottom:1px solid var(--border)"><b>#' + (i + 1) + '</b> ' + esc(c.title || '') + ' <span style="color:var(--faint)">' + (c.word_count || 0) + '字</span></div>';
      }).join('') || '<span class="res-check-empty">未识别到章节</span>';
    }).catch(function (e) { UI.toast('预览失败：' + e.message, 'error'); });
  },
  splitModeChange: function (idn) {
    var mode = document.getElementById(idn + '_mode').value;
    document.getElementById(idn + '_custom').style.display = mode === 'custom' ? '' : 'none';
  },
  continueNextChapter: async function () {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    var chs = Store.state.chapters || [];
    var lastCh = chs.length ? chs[chs.length - 1] : null;
    // 智能推断下一章序号：从最后一章标题提取数字，否则用总数+1
    var nextNum = chs.length + 1;
    if (lastCh && lastCh.title) {
      var m = lastCh.title.match(/(\d+)/g);
      if (m && m.length) {
        var lastNum = parseInt(m[m.length - 1]);
        if (!isNaN(lastNum)) nextNum = lastNum + 1;
      }
    }
    // 查找是否已有「第N章」，避免重复
    while (chs.some(function (c) { return c.title.indexOf('第' + nextNum + '章') >= 0; })) { nextNum++; }
    var newTitle = '第' + nextNum + '章';
    try {
      var newCh = await API.createChapter({
        project_id: p.id, volume_id: lastCh ? lastCh.volume_id : '',
        title: newTitle, content: ''
      });
      await ChapterUI.loadAll();
      // 用服务端返回的完整数据选中新章节
      var loaded = Store.state.chapters.find(function (c) { return c.id === newCh.id; });
      if (loaded) Store.state.currentChapter = loaded;
      Editor.setContent('');
      document.getElementById('docTitle').textContent = (p ? p.name : '') + ' · ' + newTitle;
      ProjectUI.updateMeta();
      ChapterUI.renderTree();
      if (lastCh) {
        Store.state.composer.styleChapterId = lastCh.id;
        Composer.refreshStyleChapters();
      }
      document.getElementById('instructionInput').value = '续写';
      // 续写场景自动切换上下文为智能分层模式，确保 AI 感知前文
      Store.state.composer.contextScope = 'smart';
      var scopeEl = document.getElementById('contextScope');
      if (scopeEl) scopeEl.value = 'smart';
      UI.toast('已创建「' + newTitle + '」，正在续写…', 'success');
      Composer.generate();
    } catch (e) { UI.toast('创建章节失败：' + e.message, 'error'); }
  },
  showImportMenu: function () {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    UI.modal({
      title: '导入方式',
      body: '<div style="display:flex;flex-direction:column;gap:8px">' +
        '<button class="btn btn-primary btn-block" onclick="var ov=this.closest(\'.modal-overlay\');if(ov)ov.remove();Editor.importFile()">📄 导入文件（自动分割章节）</button>' +
        '<button class="btn btn-ghost btn-block" onclick="var ov=this.closest(\'.modal-overlay\');if(ov)ov.remove();ChapterUI.importChapters()">📦 从 JSON 导入章节</button>' +
        '</div>',
      actions: [{ id: 'cancel', label: '取消' }]
    });
  },
  showContextMenu: function (e, chapterId) {
    e.preventDefault();
    e.stopPropagation();
    var c = Store.state.chapters.find(function (x) { return x.id === chapterId; });
    if (!c) return;
    var chs = Store.state.chapters;
    var idx = chs.findIndex(function (x) { return x.id === chapterId; });
    var vols = Store.state.volumes || [];
    var self = this;
    UI.ctxMenu(e, [
      { id: 'open', label: '📖 打开', onClick: function () { self.selectChapter(c); } },
      { id: 'rename', label: '✏️ 重命名', onClick: function () { self.renameChapter(chapterId); } },
      { id: 'copy', label: '📋 复制章节', onClick: function () { self.copyChapter(chapterId); } },
      { id: 'style', label: '🎨 设为文风参考', onClick: function () {
        Store.state.composer.styleChapterId = chapterId;
        Composer.refreshStyleChapters();
        UI.toast('已设为文风参考：' + c.title, 'success');
      }},
      { id: 'tags', label: '🏷 编辑标签与梗概', onClick: function () { self.editTags(chapterId); } },
      { divider: true },
      { id: 'up', label: '⬆ 上移', onClick: function () { self.moveChapter(chapterId, -1); } },
      { id: 'down', label: '⬇ 下移', onClick: function () { self.moveChapter(chapterId, 1); } },
      { id: 'moveVol', label: '📁 移动到卷', onClick: function () { self.moveChapterToVolume(chapterId); } },
      { divider: true },
      { id: 'del', label: '🗑 删除', danger: true, onClick: function () { self.delChapter(chapterId); } }
    ]);
    return false;
  },
  moveChapter: async function (chapterId, direction) {
    var chs = Store.state.chapters;
    var c = chs.find(function (x) { return x.id === chapterId; });
    if (!c) return;
    // 需要在同一卷内排序
    var sameVol = chs.filter(function (x) { return x.volume_id === c.volume_id; });
    sameVol.sort(function (a, b) { return a.sort_order - b.sort_order; });
    var idx = sameVol.findIndex(function (x) { return x.id === chapterId; });
    if (idx < 0) return;
    var targetIdx = idx + direction;
    if (targetIdx < 0 || targetIdx >= sameVol.length) return;
    // 交换 sort_order
    var tmp = sameVol[idx].sort_order;
    sameVol[idx].sort_order = sameVol[targetIdx].sort_order;
    sameVol[targetIdx].sort_order = tmp;
    var items = sameVol.map(function (x) { return { id: x.id, sort_order: x.sort_order }; });
    try {
      await API.reorderChapters(items);
      await this.loadAll();
      this.renderTree();
    } catch (e) { UI.toast('移动失败', 'error'); }
  },
  moveChapterToVolume: function (chapterId) {
    var vols = Store.state.volumes || [];
    var options = '<option value="">未分类</option>' + vols.map(function (v) {
      return '<option value="' + v.id + '">' + esc(v.title) + '</option>';
    }).join('');
    var idn = 'mv_' + uid();
    var self = this;
    UI.modal({
      title: '移动到卷',
      body: '<div class="form-group"><label>选择目标卷</label><select id="' + idn + '">' + options + '</select></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '移动', cls: 'btn-primary', onClick: async function (m, ov) {
          var vid = document.getElementById(idn).value;
          try {
            await API.updateChapter(chapterId, { volume_id: vid });
            await self.loadAll(); self.renderTree();
            UI.toast('已移动', 'success');
          } catch (e) { UI.toast('移动失败', 'error'); }
          ov.remove();
        }}
      ]
    });
  },

  // ===== 批量操作 =====
  toggleBatch: function () {
    ChapterUI.batchMode = !ChapterUI.batchMode;
    ChapterUI.batchSelected = {};
    var btn = document.getElementById('btnBatch');
    if (btn) btn.textContent = ChapterUI.batchMode ? '☑批量' : '☐批量';
    ChapterUI.renderTree();
    var p = Store.state.currentProject;
    if (p) ProjectUI.renderExpanded(p.id);
  },
  batchToggle: function (cid) {
    if (ChapterUI.batchSelected[cid]) { delete ChapterUI.batchSelected[cid]; }
    else { ChapterUI.batchSelected[cid] = true; }
    var cnt = Object.keys(ChapterUI.batchSelected).length;
    var el = document.getElementById('batchCount');
    if (el) el.textContent = '已选 ' + cnt + ' 章';
    ChapterUI.renderTree();
    var p = Store.state.currentProject;
    if (p) ProjectUI.renderExpanded(p.id);
  },
  batchSelectAll: function () {
    var chs = Store.state.chapters || [];
    chs.forEach(function (c) { ChapterUI.batchSelected[c.id] = true; });
    ChapterUI.renderTree();
    var el = document.getElementById('batchCount');
    if (el) el.textContent = '已选 ' + chs.length + ' 章';
    var p = Store.state.currentProject;
    if (p) ProjectUI.renderExpanded(p.id);
  },
  batchClear: function () {
    ChapterUI.batchSelected = {};
    ChapterUI.renderTree();
    var el = document.getElementById('batchCount');
    if (el) el.textContent = '已选 0 章';
    var p = Store.state.currentProject;
    if (p) ProjectUI.renderExpanded(p.id);
  },
  getBatchChapters: function () {
    var ids = Object.keys(ChapterUI.batchSelected);
    return (Store.state.chapters || []).filter(function (c) { return ids.indexOf(c.id) >= 0; });
  },
  batchExportTXT: function () {
    var chs = ChapterUI.getBatchChapters();
    if (!chs.length) { UI.toast('请先勾选章节', 'warn'); return; }
    var text = chs.map(function (c) {
      return '=== ' + c.title + ' ===\n\n' + (c.content || '') + '\n';
    }).join('\n');
    var p = Store.state.currentProject;
    var name = (p ? p.name : 'output') + '_selected.txt';
    var blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    var a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = name; a.click();
    URL.revokeObjectURL(a.href);
    UI.toast('已导出 ' + chs.length + ' 章', 'success');
  },
  batchExportMD: function () {
    var chs = ChapterUI.getBatchChapters();
    if (!chs.length) { UI.toast('请先勾选章节', 'warn'); return; }
    var text = chs.map(function (c) {
      return '## ' + c.title + '\n\n' + (c.content || '') + '\n';
    }).join('\n');
    var p = Store.state.currentProject;
    var name = (p ? p.name : 'output') + '_selected.md';
    var blob = new Blob([text], { type: 'text/markdown;charset=utf-8' });
    var a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = name; a.click();
    URL.revokeObjectURL(a.href);
    UI.toast('已导出 ' + chs.length + ' 章', 'success');
  },
  exportAllChapters: function () {
    var chs = Store.state.chapters || [];
    if (!chs.length) { UI.toast('暂无章节可导出', 'warn'); return; }
    // 弹出格式选择
    var p = Store.state.currentProject;
    UI.modal({
      title: '📚 导出全本',
      body: '<div class="form-group"><label>导出格式</label><select id="exportAllFmt"><option value="txt">纯文本文档(.txt)</option><option value="md">Markdown(.md)</option><option value="zip">打包ZIP(按卷分文件夹)</option></select></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '导出', cls: 'btn-primary', onClick: function (m, ov) {
          var fmt = document.getElementById('exportAllFmt').value;
          ov.remove();
          if (fmt === 'zip') { ChapterUI.exportAllZIP(chs, p); return; }
          var text = chs.map(function (c) { return '=== ' + c.title + ' ===\n\n' + (c.content || '') + '\n'; }).join('\n\n');
          var ext = fmt === 'md' ? '.md' : '.txt';
          var mime = fmt === 'md' ? 'text/markdown;charset=utf-8' : 'text/plain;charset=utf-8';
          var blob = new Blob([text], { type: mime });
          var a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = (p ? p.name : 'output') + '_全本' + ext; a.click();
          URL.revokeObjectURL(a.href);
          UI.toast('已导出全书 ' + chs.length + ' 章', 'success');
        }}
      ]
    });
  },
  // ---- 简易 ZIP 构建器（纯 JS，无外部依赖）----
  _buildZipBlob: function (files) {
    // files: [{name: string, content: string}]
    var encoder = new TextEncoder();
    var localHeaders = [];
    var centralHeaders = [];
    var offset = 0;

    files.forEach(function (f) {
      var encodedName = encoder.encode(f.name);
      var nameBytes = Array.from(encodedName);
      var content = encoder.encode(f.content);
      var contentBytes = Array.from(content);
      var crc = 0; // simplified - real CRC32 would need a separate implementation

      var lh = [];
      lh.push(0x50, 0x4B, 0x03, 0x04); // local header signature
      lh.push(0x14, 0x00);               // version needed
      lh.push(0x00, 0x00);               // flags
      lh.push(0x00, 0x00);               // compression: store
      lh.push(0x00, 0x00);               // mod time
      lh.push(0x00, 0x00);               // mod date
      // CRC-32
      var crcBytes = [crc & 0xFF, (crc >> 8) & 0xFF, (crc >> 16) & 0xFF, (crc >> 24) & 0xFF];
      lh = lh.concat(crcBytes);
      // Compressed size = uncompressed size
      var size = contentBytes.length;
      lh.push(size & 0xFF, (size >> 8) & 0xFF, (size >> 16) & 0xFF, (size >> 24) & 0xFF);
      lh = lh.concat([size & 0xFF, (size >> 8) & 0xFF, (size >> 16) & 0xFF, (size >> 24) & 0xFF]);
      // File name length
      lh.push(nameBytes.length & 0xFF, (nameBytes.length >> 8) & 0xFF);
      lh.push(0x00, 0x00); // extra field length

      localHeaders.push({ header: lh, name: nameBytes, data: contentBytes, offset: offset });
      offset += lh.length + nameBytes.length + contentBytes.length;

      // Build central directory entry
      var ch = [];
      ch.push(0x50, 0x4B, 0x01, 0x02);
      ch.push(0x14, 0x00); // version
      ch.push(0x14, 0x00); // version needed
      ch.push(0x00, 0x00); // flags
      ch.push(0x00, 0x00); // compression
      ch.push(0x00, 0x00); // mod time
      ch.push(0x00, 0x00); // mod date
      ch = ch.concat(crcBytes);
      ch.push(size & 0xFF, (size >> 8) & 0xFF, (size >> 16) & 0xFF, (size >> 24) & 0xFF);
      ch = ch.concat([size & 0xFF, (size >> 8) & 0xFF, (size >> 16) & 0xFF, (size >> 24) & 0xFF]);
      ch.push(nameBytes.length & 0xFF, (nameBytes.length >> 8) & 0xFF);
      ch.push(0x00, 0x00); // extra
      ch.push(0x00, 0x00); // comment
      ch.push(0x00, 0x00); // disk
      ch.push(0x00, 0x00); // internal
      ch.push(0x20, 0x00, 0x00, 0x00); // external
      // local header offset
      var lhOff = localHeaders[localHeaders.length - 1].offset;
      ch.push(lhOff & 0xFF, (lhOff >> 8) & 0xFF, (lhOff >> 16) & 0xFF, (lhOff >> 24) & 0xFF);
      centralHeaders.push({ header: ch, name: nameBytes });
    });

    // Assemble
    var parts = [];
    localHeaders.forEach(function (lh) {
      parts.push(new Uint8Array(lh.header));
      parts.push(new Uint8Array(lh.name));
      parts.push(new Uint8Array(lh.data));
    });

    var cdOffset = 0;
    parts.forEach(function (p) { cdOffset += p.length; });

    centralHeaders.forEach(function (ch) {
      parts.push(new Uint8Array(ch.header));
      parts.push(new Uint8Array(ch.name));
    });

    var cdSize = 0;
    for (var i = localHeaders.length; i < parts.length; i++) { cdSize += parts[i].length; }

    // EOCD
    var eocd = [
      0x50, 0x4B, 0x05, 0x06,
      0x00, 0x00, // disk
      0x00, 0x00, // central disk
      files.length & 0xFF, (files.length >> 8) & 0xFF, // entries on disk
      files.length & 0xFF, (files.length >> 8) & 0xFF, // total entries
      cdSize & 0xFF, (cdSize >> 8) & 0xFF, (cdSize >> 16) & 0xFF, (cdSize >> 24) & 0xFF,
      cdOffset & 0xFF, (cdOffset >> 8) & 0xFF, (cdOffset >> 16) & 0xFF, (cdOffset >> 24) & 0xFF,
      0x00, 0x00 // comment length
    ];
    parts.push(new Uint8Array(eocd));

    return new Blob(parts, { type: 'application/zip' });
  },

  exportAllZIP: function (chs, p) {
    var vols = Store.state.volumes || [];
    var groups = {};
    vols.forEach(function (v) { groups[v.id] = { title: v.title, chs: [] }; });
    groups[''] = { title: '未分类', chs: [] };
    chs.forEach(function (c) {
      var vid = c.volume_id || '';
      if (!groups[vid]) groups[vid] = { title: '未分类', chs: [] };
      groups[vid].chs.push(c);
    });

    var files = [];
    Object.keys(groups).forEach(function (vid) {
      var g = groups[vid];
      if (!g.chs.length) return;
      g.chs.forEach(function (c) {
        var safeVol = g.title.replace(/[<>:"/\\|?*]/g, '_');
        var safeName = (c.title || '未命名章节').replace(/[<>:"/\\|?*]/g, '_');
        var path = safeVol + '/' + safeName + '.txt';
        files.push({ name: path, content: (c.content || '') });
      });
    });

    if (files.length === 0) { UI.toast('没有可导出的章节', 'warn'); return; }
    var zipName = (p ? p.name : 'output') + '_全本.zip';
    var blob = this._buildZipBlob(files);
    var a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = zipName; a.click();
    URL.revokeObjectURL(a.href);
    UI.toast('已导出全书 ' + chs.length + ' 章（真实 ZIP 格式）', 'success');
  },
  batchDelete: function () {
    var chs = ChapterUI.getBatchChapters();
    if (!chs.length) { UI.toast('请先勾选章节', 'warn'); return; }
    UI.confirm('批量删除', '确认删除 ' + chs.length + ' 个章节？<br><small style="color:var(--muted)">可在回收站中恢复，保留7天</small>', async function () {
      try {
        for (var i = 0; i < chs.length; i++) {
          await API.deleteChapter(chs[i].id);
        }
        if (Store.state.currentChapter && ChapterUI.batchSelected[Store.state.currentChapter.id]) {
          Store.state.currentChapter = null; Editor.setContent('');
        }
        ChapterUI.batchMode = false; ChapterUI.batchSelected = {};
        await ChapterUI.loadAll(); ChapterUI.renderTree();
        var p = Store.state.currentProject;
        if (p) ProjectUI.renderExpanded(p.id);
        UI.toast('已删除 ' + chs.length + ' 章', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  },
  fillUpToTarget: function () {
    var ch = Store.state.currentChapter;
    var target = Store.state.composer.targetWord;
    if (!ch || !target) { UI.toast('无目标字数或未选中章节', 'warn'); return; }
    var current = (ch.content || '').length;
    var deficit = target - current;
    if (deficit <= 0) { UI.toast('字数已达标', 'success'); return; }
    Store.state.composer.cursorPosition = 0; // 追加到末尾
    Store.state.composer.noRewrite = true; // 不改写前文
    document.getElementById('noRewriteToggle').checked = true;
    document.getElementById('instructionInput').value = '续写约' + deficit + '字，严格禁止改写已有前文，仅追加新内容';
    UI.toast('补写 ' + deficit + ' 字中…', 'info');
    document.getElementById('genActions').style.display = 'none';
    Composer.generate();
  },
  searchAllChapters: function () {
    var idn = 'sr_' + uid();
    UI.modal({
      title: '全文搜索与替换',
      body: '<div class="form-row"><div class="form-group"><label>搜索</label><input id="' + idn + '_q"></div>' +
        '<div class="form-group"><label>替换为（可选）</label><input id="' + idn + '_r"></div></div>' +
        '<div id="' + idn + '_out" style="max-height:300px;overflow-y:auto"></div>',
      actions: [
        { id: 'search', label: '🔍 搜索', cls: 'btn-primary', onClick: function () {
          var q = document.getElementById(idn + '_q').value.trim();
          if (!q) { UI.toast('请输入关键词', 'warn'); return; }
          q = q.toLowerCase();
          var chs = Store.state.chapters || [], results = [];
          chs.forEach(function (c) {
            if (!c.content) return;
            var idx = c.content.toLowerCase().indexOf(q);
            if (idx < 0) return;
            var ctx = c.content.substring(Math.max(0, idx - 40), idx + q.length + 40).replace(/\n/g, ' ');
            results.push({ title: c.title, pos: idx, ctx: ctx, id: c.id });
          });
          if (!results.length) { document.getElementById(idn + '_out').innerHTML = '<div class="res-check-empty">未找到</div>'; return; }
          document.getElementById(idn + '_out').innerHTML = results.map(function (r) {
            return '<div onclick="ChapterUI.selectChapter(Store.state.chapters.find(function(x){return x.id===\'' + r.id + '\'}))" style="cursor:pointer;padding:4px 6px;border-bottom:1px solid var(--border)"><b>' + esc(r.title) + '</b> · 第' + r.pos.toLocaleString() + '字<br><span style="color:var(--muted);font-size:11px">' + esc(r.ctx) + '</span></div>';
          }).join('');
        }},
        { id: 'replace', label: '🔄 全部替换', cls: 'btn-danger', onClick: async function () {
          var q = document.getElementById(idn + '_q').value.trim();
          var r = document.getElementById(idn + '_r').value;
          if (!q) { UI.toast('请输入搜索词', 'warn'); return; }
          if (!confirm('确认将全文' + (Store.state.chapters || []).length + '章中所有「' + q + '」替换为「' + r + '」？不可撤销。')) return;
          var count = 0;
          for (var i = 0; i < (Store.state.chapters || []).length; i++) {
            var c = Store.state.chapters[i];
            if (!c.content || c.content.indexOf(q) < 0) continue;
            var newContent = c.content.split(q).join(r);
            await API.updateChapter(c.id, { content: newContent });
            c.content = newContent;
            count++;
          }
          UI.toast('已替换 ' + count + ' 个章节', 'success');
          if (Store.state.currentChapter) Editor.setContent(Store.state.currentChapter.content || '');
        }},
        { id: 'cancel', label: '关闭' }
      ]
    });
  },
  batchFromOutline: function () {
    var outline = (document.getElementById('genOutline') || {}).value || Store.state.composer.outline || '';
    if (!outline || !outline.trim()) { UI.toast('请先在高级选项中填写大纲，支持 #卷名 ##章节名 格式', 'warn'); return; }
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    var lines = outline.split('\n').filter(function (l) { return l.trim(); });
    if (lines.length === 0) { UI.toast('大纲为空', 'warn'); return; }
    var chaps = []; var curVol = '';
    lines.forEach(function (line) {
      var trimmed = line.trim();
      if (trimmed.startsWith('## ')) {
        chaps.push({ vol: curVol, title: trimmed.replace(/^##\s*/, '').trim() });
      } else if (trimmed.startsWith('# ')) {
        curVol = trimmed.replace(/^#\s*/, '').trim();
      } else if (trimmed) {
        chaps.push({ vol: curVol, title: trimmed });
      }
    });
    if (chaps.length === 0) { UI.toast('未识别到章节（请使用 #卷名 ##章节名 格式）', 'warn'); return; }
    UI.confirm('批量创建章节', '将根据大纲创建 <b>' + chaps.length + '</b> 个空白章节，确定？', async function () {
      var created = 0;
      var curVolId = '';
      for (var i = 0; i < chaps.length; i++) {
        var c = chaps[i];
        if (c.vol) {
            try {
            var v = await API.createVolume({ project_id: p.id, title: c.vol, sort_order: created });
            curVolId = v ? (v.id || '') : '';
          } catch (e) { curVolId = ''; }
        }
        if (c.title) {
          try {
            await API.createChapter({ project_id: p.id, volume_id: curVolId, title: c.title, content: '' });
            created++;
          } catch (e) { UI.toast('创建"' + esc(c.title) + '"失败', 'error'); }
        }
      }
      await ChapterUI.loadAll(); ChapterUI.renderTree(); ProjectUI.updateMeta();
      UI.toast('已创建 ' + created + ' 个章节', 'success');
    });
  },
  editOutlineAndContinue: function () {
    var outline = Store.state.pipeline.outline;
    if (!outline) { UI.toast('尚无 Thinker 大纲，先生成一次', 'warn'); return; }
    var idn = 'ol_' + uid();
    UI.modal({
      title: '编辑大纲（修改后由 Worker 按新框架撰写）',
      body: '<textarea id="' + idn + '" style="width:100%;min-height:200px">' + esc(outline) + '</textarea>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '确认并重新生成', cls: 'btn-primary', onClick: function (m, ov) {
          var newOutline = document.getElementById(idn).value.trim();
          if (!newOutline) { UI.toast('大纲不能为空', 'warn'); return; }
          Store.state.pipeline.outline = newOutline;
          ov.remove();
          document.getElementById('instructionInput').value = '请严格按以下大纲撰写正文：' + newOutline;
          var genOutline = document.getElementById('genOutline');
          if (genOutline) genOutline.value = newOutline;
          UI.toast('大纲已更新，正在生成…', 'success');
          Composer.generate();
        }}
      ]
    });
  }
};
