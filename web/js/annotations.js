/* ============ annotations.js：批注/高亮 + 阅读进度（2026-08-05 阅读工具） ============
   锚定方案：章节以纯文本保存（Tiptap getText()，块间 '\n\n' 分隔）。
   高亮/批注存 [start, end) 字符偏移；渲染用 ProseMirror decorations（不污染 schema、不影响保存内容）。
   偏移→文档位置：二分 textBetween(0, mid) 长度。 */
var Annotations = {
  items: [],

  /* 偏移 → 文档位置（二分，O(n log n)，章节级文本足够快） */
  findPos: function (doc, offset) {
    if (offset <= 0) return 1;
    var lo = 1, hi = Math.max(doc.content.size, 1);
    while (lo < hi) {
      var mid = (lo + hi) >> 1;
      if (doc.textBetween(0, mid, '\n\n', '').length < offset) lo = mid + 1; else hi = mid;
    }
    return Math.min(Math.max(lo, 1), doc.content.size);
  },

  /* Tiptap editorProps.decorations：把批注/高亮渲染到编辑器 */
  decorations: function (state) {
    var decos = [];
    var doc = state.doc;
    Annotations.items.forEach(function (a) {
      var posA = Annotations.findPos(doc, a.start);
      var posB = Annotations.findPos(doc, a.end);
      if (posB <= posA) return;
      if (a.type === 'highlight') {
        decos.push(Decoration.inline(posA, posB, {
          style: 'background:' + a.color + ';border-radius:3px;cursor:pointer',
          'data-ann-id': a.id, 'data-ann-type': 'highlight'
        }));
      } else {
        decos.push(Decoration.inline(posA, posB, {
          style: 'background:rgba(250,204,21,.16);border-bottom:1.5px dashed #f59e0b;border-radius:3px;cursor:pointer',
          'data-ann-id': a.id, 'data-ann-type': 'comment'
        }));
        decos.push(Decoration.widget(posB + 1, function () {
          var s = document.createElement('span');
          s.textContent = '📝';
          s.style.cssText = 'cursor:pointer;font-size:11px;margin:0 1px';
          s.setAttribute('data-ann-id', a.id);
          s.title = a.note || '批注';
          return s;
        }));
      }
    });
    return DecorationSet.create(doc, decos);
  },

  load: async function (chapterId) {
    if (!chapterId) { this.items = []; this.refresh(); return; }
    try { this.items = await API.listAnnotations(chapterId); } catch (e) { this.items = []; }
    this.refresh();
  },

  /* 空事务触发 decorations 重算 */
  refresh: function () {
    var ed = Editor.tiptap;
    if (ed) ed.view.dispatch(ed.state.tr.setMeta('addToHistory', false));
  },

  /* ===== 编辑后锚定对齐（快照匹配）：正文增删后按 selected_text 就近重新定位 ===== */
  scheduleRealign: function () {
    if (this._rlTimer) clearTimeout(this._rlTimer);
    var self = this;
    this._rlTimer = setTimeout(function () { self.realign(); }, 800);
  },
  realign: function () {
    var ed = Editor.tiptap;
    if (!ed || !this.items.length) return;
    var doc = ed.state.doc;
    var text = doc.textBetween(0, doc.content.size, '\n\n', '');
    var changed = [];
    this.items.forEach(function (a) {
      var st = a.selected_text || '';
      if (st.length < 2) return; // 快照太短（1字）不匹配，避免误对齐
      var idx = text.indexOf(st);
      if (idx < 0) return; // 快照已被改掉：保留原偏移，渲染越界时自动跳过
      // 就近匹配：取距原 start 最近的一次出现（章节内重复句少，防误对齐）
      var best = idx, bestDist = Math.abs(idx - a.start);
      var from = idx;
      while (true) {
        var n = text.indexOf(st, from + 1);
        if (n < 0) break;
        var d = Math.abs(n - a.start);
        if (d < bestDist) { bestDist = d; best = n; from = n; } else break;
      }
      var nend = best + st.length;
      if (nend > text.length) return;
      if (best === a.start && nend === a.end) return;
      changed.push({ id: a.id, start: best, end: nend });
      a.start = best;
      a.end = nend;
    });
    if (!changed.length) return;
    var self = this;
    changed.forEach(function (c) {
      API.updateAnnotation(c.id, { start: c.start, end: c.end }).catch(function () {});
    });
    this.refresh();
  },

  /* 当前选中文字 → {start, end, selected_text}（基于保存格式 '\n\n' 分隔） */
  selRange: function () {
    var ed = Editor.tiptap;
    if (!ed) return null;
    var sel = ed.state.selection;
    if (sel.from === sel.to) return null;
    var start = ed.state.doc.textBetween(0, sel.from, '\n\n', '').length;
    var selText = ed.state.doc.textBetween(sel.from, sel.to, '\n\n', '');
    if (!selText || !selText.trim()) return null;
    if (selText.length > 500) { UI.toast('一次标注最多 500 字', 'warn'); return null; }
    return { start: start, end: start + selText.length, selected_text: selText.slice(0, 300) };
  },

  add: function (type, color, note) {
    var p = Store.state.currentProject, ch = Store.state.currentChapter;
    if (!p || !ch) { UI.toast('请先选择项目与章节', 'warn'); return; }
    var r = this.selRange();
    if (!r) { UI.toast('请先选中需要标注的文字', 'warn'); return; }
    Editor.hideSelToolbar();
    var self = this;
    API.createAnnotation({
      project_id: p.id, chapter_id: ch.id,
      start: r.start, end: r.end, selected_text: r.selected_text,
      type: type, color: color || '#fde68a', note: note || ''
    }).then(function () {
      UI.toast(type === 'comment' ? '批注已添加' : '高亮已添加', 'success');
      return self.load(ch.id);
    }).catch(function (e) { UI.toast('标注失败: ' + e.message, 'error'); });
  },

  /* 点击高亮/批注（mark 带 data-ann-id）→ 查看/编辑 */
  open: function (annId) {
    var a = null;
    this.items.forEach(function (x) { if (x.id === annId) a = x; });
    if (!a) return;
    var self = this;
    var html = '<div style="font-size:12px;color:var(--muted);margin-bottom:8px;background:var(--panel);border-radius:6px;padding:8px;line-height:1.6">“' + esc(a.selected_text) + '”</div>';
    var actions = [];
    if (a.type === 'highlight') {
      var colors = ['#fde68a', '#bbf7d0', '#bfdbfe', '#fbcfe8', '#fed7aa', '#e9d5ff'];
      html += '<div class="form-group"><label>颜色（点选即改）</label><div style="display:flex;gap:8px">';
      colors.forEach(function (c) {
        html += '<span style="width:24px;height:24px;border-radius:50%;background:' + c + ';cursor:pointer;border:2px solid ' + (a.color === c ? 'var(--accent)' : 'transparent') + '" onclick="Annotations.changeColor(\'' + a.id + '\',\'' + c + '\')" title="选此颜色"></span>';
      });
      html += '</div></div>';
    } else {
      html += '<div class="form-group"><label>批注内容</label><textarea id="annNote" rows="3">' + esc(a.note || '') + '</textarea></div>';
    }
    if (a.type === 'comment') {
      actions.push({ id: 'ok', label: '保存批注', cls: 'btn-primary', onClick: function (m, ov) {
        var note = document.getElementById('annNote') ? document.getElementById('annNote').value.trim() : '';
        ov.remove();
        self.update(a.id, note, a.color);
      } });
    }
    actions.push({ id: 'del', label: '🗑 删除', cls: 'btn-danger', onClick: function (m, ov) { ov.remove(); self.del(a.id); } });
    actions.push({ id: 'cancel', label: '关闭' });
    UI.modal({ title: a.type === 'comment' ? '💬 批注' : '🎨 高亮', body: html, actions: actions });
  },

  changeColor: function (id, color) {
    this.items.forEach(function (x) { if (x.id === id) x.color = color; });
    this.update(id, '', color);
  },

  update: function (id, note, color) {
    var self = this, ch = Store.state.currentChapter;
    API.updateAnnotation(id, { note: note, color: color }).then(function () {
      UI.toast('已保存', 'success');
      if (ch) return self.load(ch.id);
    }).catch(function (e) { UI.toast('保存失败: ' + e.message, 'error'); });
  },

  del: function (id) {
    var self = this, ch = Store.state.currentChapter;
    API.deleteAnnotation(id).then(function () {
      UI.toast('已删除', 'success');
      if (ch) return self.load(ch.id);
    }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
  }
};

/* ============ 阅读进度：打开章节自动记录 ============ */
var ReaderProgress = {
  record: function () {
    var p = Store.state.currentProject, ch = Store.state.currentChapter;
    if (!p || !ch) return;
    API.setReadingProgress({ project_id: p.id, chapter_id: ch.id, scroll_pct: 0 }).catch(function () {});
  },
  /* 拉取进度并高亮章节列表中的"读到"标记 */
  markList: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    API.getReadingProgress(p.id).then(function (rp) {
      ReaderProgress.current = rp || null;
      ReaderProgress.applyMark();
    }).catch(function () {});
  },
  applyMark: function () {
    var rp = ReaderProgress.current;
    document.querySelectorAll('#chapterTree .ch-item').forEach(function (el) {
      var cid = el.getAttribute('data-cid');
      var mark = el.querySelector('.rp-mark');
      if (mark) mark.remove();
      if (rp && rp.chapter_id && cid === rp.chapter_id) {
        var s = document.createElement('span');
        s.className = 'rp-mark';
        s.textContent = '📍';
        s.title = '上次读到这里';
        s.style.cssText = 'margin-left:4px;font-size:11px';
        el.appendChild(s);
      }
    });
  }
};
