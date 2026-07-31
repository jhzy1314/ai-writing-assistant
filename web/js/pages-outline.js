/* ============ pages-outline.js：章节大纲树状面板 ============ */
var OutlinePage = {
  init: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    this.load();
  },
  wcClass: function (n) {
    n = n || 0;
    if (n === 0) return 'wc-0';
    if (n < 1000) return 'wc-low';
    if (n < 3000) return 'wc-mid';
    if (n < 5000) return 'wc-done';
    return 'wc-big';
  },
  load: async function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty('请先在左侧选中一个项目'); return; }
    var tree = document.getElementById('outlineTree');
    tree.innerHTML = '<div class="loading">加载中</div>';
    try {
      var vols = await API.listVolumes(p.id);
      var chs = await API.listChapters(p.id);
      this.renderTree(vols, chs);
      this.updateStats(vols, chs);
    } catch (e) {
      tree.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>加载失败: ' + esc(e.message) + '</div></div>';
    }
  },
  renderTree: function (vols, chs) {
    var tree = document.getElementById('outlineTree');
    if (!vols.length && !chs.length) {
      var orphanChs = chs.filter(function (c) { return !c.volume_id; });
      if (!orphanChs.length) {
        tree.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">📖</div><div>暂无章节，点击上方按钮创建</div></div>';
        return;
      }
    }
    var html = '';
    vols.forEach(function (vol) {
      var volChs = chs.filter(function (c) { return c.volume_id === vol.id; });
      html += '<div class="ot-volume" data-vid="' + vol.id + '">';
      html += '<div class="ot-vol-head" onclick="OutlinePage.toggleVolume(this)">';
      html += '<span class="caret">▾</span>';
      html += '<span class="ot-vol-icon">📁</span>';
      html += '<span class="ot-vol-name">' + esc(vol.name || vol.title || '未命名卷') + '</span>';
      html += '<span class="ot-vol-count">' + volChs.length + ' 章</span>';
      html += '<span class="ot-vol-acts">';
      html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.renameVolume(\'' + vol.id + '\')">✏</span>';
      html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.addChapterToVol(\'' + vol.id + '\')">＋</span>';
      html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.delVolume(\'' + vol.id + '\')">✕</span>';
      html += '</span>';
      html += '</div>';
      html += '<div class="ot-vol-body">';
      if (volChs.length === 0) {
        html += '<div class="ot-empty-hint">拖拽章节到此处</div>';
      } else {
        volChs.forEach(function (ch, i) {
          html += '<div class="ot-ch-item ' + OutlinePage.wcClass(ch.word_count) + '" data-cid="' + ch.id + '" draggable="true" ondragstart="OutlinePage.dragStart(event,\'' + ch.id + '\')" ondragover="OutlinePage.dragOver(event)" ondrop="OutlinePage.dragDrop(event,\'' + vol.id + '\',\'' + ch.id + '\')">';
          html += '<span class="ot-ch-num">' + (i + 1) + '</span>';
          html += '<span class="ot-ch-status"></span>';
          html += '<span class="ot-ch-icon">📄</span>';
          html += '<span class="ot-ch-name">' + esc(ch.title || '未命名章节') + '</span>';
          html += '<span class="ot-ch-wc">' + (ch.word_count || 0) + ' 字</span>';
          html += '<span class="ot-ch-acts">';
          html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.moveUp(\'' + ch.id + '\')" title="上移">▲</span>';
          html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.moveDown(\'' + ch.id + '\')" title="下移">▼</span>';
          html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.renameChapter(\'' + ch.id + '\')" title="重命名">✏</span>';
          html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.delChapter(\'' + ch.id + '\')" title="删除">✕</span>';
          html += '</span>';
          html += '</div>';
        });
      }
      html += '</div>';
      html += '</div>';
    });
    var orphans = chs.filter(function (c) { return !c.volume_id; });
    if (orphans.length > 0) {
      html += '<div class="ot-volume ot-orphan">';
      html += '<div class="ot-vol-head"><span class="caret">▾</span><span class="ot-vol-icon">📑</span><span class="ot-vol-name">未归类章节</span><span class="ot-vol-count">' + orphans.length + ' 章</span></div>';
      html += '<div class="ot-vol-body">';
      orphans.forEach(function (ch, i) {
        html += '<div class="ot-ch-item ' + OutlinePage.wcClass(ch.word_count) + '" data-cid="' + ch.id + '" draggable="true" ondragstart="OutlinePage.dragStart(event,\'' + ch.id + '\')" ondragover="OutlinePage.dragOver(event)" ondrop="OutlinePage.dragDrop(event,null,\'' + ch.id + '\')">';
        html += '<span class="ot-ch-num">' + (i + 1) + '</span><span class="ot-ch-status"></span><span class="ot-ch-icon">📄</span>';
        html += '<span class="ot-ch-name">' + esc(ch.title || '未命名章节') + '</span>';
        html += '<span class="ot-ch-wc">' + (ch.word_count || 0) + ' 字</span>';
        html += '<span class="ot-ch-acts">';
        html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.renameChapter(\'' + ch.id + '\')">✏</span>';
        html += '<span class="link-btn" onclick="event.stopPropagation();OutlinePage.delChapter(\'' + ch.id + '\')">✕</span>';
        html += '</span></div>';
      });
      html += '</div></div>';
    }
    tree.innerHTML = html;
  },
  toggleVolume: function (head) {
    head.parentElement.classList.toggle('collapsed');
  },
  expandAll: function () {
    document.querySelectorAll('#outlineTree .ot-volume').forEach(function (v) { v.classList.remove('collapsed'); });
  },
  collapseAll: function () {
    document.querySelectorAll('#outlineTree .ot-volume').forEach(function (v) { v.classList.add('collapsed'); });
  },
  updateStats: function (vols, chs) {
    var totalWords = chs.reduce(function (s, c) { return s + (c.word_count || 0); }, 0);
    document.getElementById('outlineStats').textContent = vols.length + ' 卷 · ' + chs.length + ' 章 · ' + totalWords + ' 字';
  },
  showEmpty: function (msg) {
    document.getElementById('outlineTree').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">📖</div><div>' + msg + '</div></div>';
  },
  addVolume: function () {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var self = this;
    UI.prompt('新建卷', '卷名称：', '新卷', function (name) {
      if (!name) return;
      API.createVolume({ project_id: p.id, name: name }).then(function () {
        UI.toast('卷已创建', 'success');
        self.load();
      }).catch(function (e) { UI.toast('创建失败: ' + e.message, 'error'); });
    });
  },
  renameVolume: function (vid) {
    var self = this;
    UI.prompt('重命名卷', '新名称：', '', function (name) {
      if (!name) return;
      API.updateVolume(vid, { name: name }).then(function () {
        UI.toast('已重命名', 'success');
        self.load();
      }).catch(function (e) { UI.toast('重命名失败: ' + e.message, 'error'); });
    });
  },
  delVolume: function (vid) {
    var self = this;
    UI.confirm('删除卷', '确定删除此卷及其下所有章节？此操作不可撤销！', function () {
      API.deleteVolume(vid).then(function () {
        UI.toast('已删除', 'success');
        self.load();
      }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  addChapter: function () {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var self = this;
    UI.prompt('新建章节', '章节标题：', '新章节', function (title) {
      if (!title) return;
      API.createChapter({ project_id: p.id, title: title }).then(function () {
        UI.toast('章节已创建', 'success');
        self.load();
        if (typeof ChapterUI !== 'undefined') ChapterUI.load();
      }).catch(function (e) { UI.toast('创建失败: ' + e.message, 'error'); });
    });
  },
  addChapterToVol: function (vid) {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var self = this;
    UI.prompt('新建章节', '章节标题：', '新章节', function (title) {
      if (!title) return;
      API.createChapter({ project_id: p.id, title: title, volume_id: vid }).then(function () {
        UI.toast('章节已创建', 'success');
        self.load();
        if (typeof ChapterUI !== 'undefined') ChapterUI.load();
      }).catch(function (e) { UI.toast('创建失败: ' + e.message, 'error'); });
    });
  },
  renameChapter: function (cid) {
    var self = this;
    UI.prompt('重命名章节', '新标题：', '', function (title) {
      if (!title) return;
      API.updateChapter(cid, { title: title }).then(function () {
        UI.toast('已重命名', 'success');
        self.load();
      }).catch(function (e) { UI.toast('重命名失败: ' + e.message, 'error'); });
    });
  },
  delChapter: function (cid) {
    var self = this;
    UI.confirm('删除章节', '确定删除此章节？可到回收站恢复。', function () {
      API.deleteChapter(cid).then(function () {
        UI.toast('已移入回收站', 'success');
        self.load();
        if (typeof ChapterUI !== 'undefined') ChapterUI.load();
      }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  moveUp: function (cid) {
    var self = this;
    this.reorderChapter(cid, -1).catch(function (e) { UI.toast(e.message, 'error'); });
  },
  moveDown: function (cid) {
    var self = this;
    this.reorderChapter(cid, 1).catch(function (e) { UI.toast(e.message, 'error'); });
  },
  reorderChapter: async function (cid, delta) {
    var p = Store.state.currentProject;
    var chs = await API.listChapters(p.id);
    var idx = -1;
    for (var i = 0; i < chs.length; i++) { if (chs[i].id === cid) { idx = i; break; } }
    if (idx < 0 || (idx + delta < 0) || (idx + delta >= chs.length)) return;
    var items = [];
    for (var j = 0; j < chs.length; j++) {
      var pos = j;
      if (j === idx) pos = j + delta;
      else if (j === idx + delta) pos = idx;
      items.push({ id: chs[j].id, sort_order: pos });
    }
    await API.reorderChapters(items);
    this.load();
  },
  batchFromOutline: function () {
    if (typeof ChapterUI !== 'undefined' && ChapterUI.batchFromOutline) {
      ChapterUI.batchFromOutline();
    }
  },
  dragStart: function (e, cid) {
    e.dataTransfer.setData('text/plain', cid);
    e.dataTransfer.effectAllowed = 'move';
  },
  dragOver: function (e) { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; },
  dragDrop: function (e, targetVid, targetCid) {
    e.preventDefault();
    var cid = e.dataTransfer.getData('text/plain');
    if (!cid) return;
    var p = Store.state.currentProject;
    if (!p) return;
    API.updateChapter(cid, { volume_id: targetVid || null }).then(function () {
      OutlinePage.load();
    }).catch(function (err) { UI.toast('移动失败: ' + err.message, 'error'); });
  },
  exportOutline: function () {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var text = [];
    var vols = document.querySelectorAll('#outlineTree .ot-volume');
    vols.forEach(function (v) {
      var name = v.querySelector('.ot-vol-name');
      text.push('## ' + (name ? name.textContent : '未命名'));
      v.querySelectorAll('.ot-ch-item').forEach(function (ch, i) {
        var cn = ch.querySelector('.ot-ch-name');
        text.push('  ' + (i + 1) + '. ' + (cn ? cn.textContent : ''));
      });
      text.push('');
    });
    var blob = new Blob([text.join('\n')], { type: 'text/plain' });
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = (p.name || 'untitled') + '-outline.txt';
    a.click();
    UI.toast('大纲已导出', 'success');
  },
  importOutline: function () {
    var self = this;
    UI.modal({
      title: '导入大纲',
      body: '<p style="font-size:12px;color:var(--muted);margin-bottom:10px">粘贴大纲文本，格式：<br>"## 卷名" 后跟 "1. 章节标题"</p><textarea id="outlineImportText" rows="10" style="width:100%" placeholder="## 卷一&#10;1. 第一章标题&#10;2. 第二章标题"></textarea>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '导入', cls: 'btn-primary', onClick: function (m, ov) {
          var text = document.getElementById('outlineImportText').value;
          if (!text) { UI.toast('请输入大纲内容', 'warn'); return; }
          ov.remove();
          self.parseImport(text);
        }}
      ]
    });
  },
  parseImport: async function (text) {
    var p = Store.state.currentProject;
    if (!p) return;
    var lines = text.split('\n');
    var currentVolId = null;
    var total = 0;
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i].trim();
      if (!line) continue;
      if (/^##\s+/.test(line)) {
        var volName = line.replace(/^##\s+/, '');
        var vol = await API.createVolume({ project_id: p.id, name: volName });
        currentVolId = vol.id;
      } else if (/^\d+[\.\、\)]\s*/.test(line) && currentVolId) {
        var chTitle = line.replace(/^\d+[\.\、\)]\s*/, '');
        await API.createChapter({ project_id: p.id, title: chTitle, volume_id: currentVolId });
        total++;
      }
    }
    UI.toast('成功导入 ' + total + ' 个章节', 'success');
    this.load();
  }
};
