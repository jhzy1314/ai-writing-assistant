/* ============ editor.js：Tiptap 双模式编辑器 ============ */
var Editor = {
  tiptap: null,
  streaming: false,
  _locked: false,
  mode: 'rich',
  preview: false,
  elRich: function () { return document.getElementById('editorRich'); },
  elMd: function () { return document.getElementById('editorMd'); },
  elPane: function () { return document.getElementById('editorPane'); },
  elPreview: function () { return document.getElementById('previewPane'); },
  elPreviewInner: function () { return document.getElementById('previewInner'); },

  lock: function () {
    this._locked = true;
    if (this.tiptap) this.tiptap.setEditable(false);
    var mdEl = this.elMd();
    if (mdEl) { mdEl.readOnly = true; mdEl.style.cursor = 'not-allowed'; }
    // 禁止侧栏项目切换
    var sidebar = document.getElementById('sidebar');
    if (sidebar) sidebar.style.pointerEvents = 'none';
    var rightPanel = document.getElementById('rightPanel');
    if (rightPanel) rightPanel.style.pointerEvents = 'none';
  },

  unlock: function () {
    this._locked = false;
    if (this.tiptap) this.tiptap.setEditable(true);
    var mdEl = this.elMd();
    if (mdEl) { mdEl.readOnly = false; mdEl.style.cursor = 'text'; }
    var sidebar = document.getElementById('sidebar');
    if (sidebar) sidebar.style.pointerEvents = '';
    var rightPanel = document.getElementById('rightPanel');
    if (rightPanel) rightPanel.style.pointerEvents = '';
  },

  init: function () {
    this.mode = Store.state.editor.mode;
    this._idleDisabled = Store.get('idleHintsDisabled', false);
    this.applyModeUI();
    var self = this;
    if (window.initTiptap) {
      window.initTiptap().then(function (editor) {
        self.tiptap = editor;
        self.loadLatest();
        editor.on('update', function () { self.onTiptapUpdate(); });
        editor.on('selectionUpdate', function () { self.onSelection(); });
      }).catch(function (err) {
        console.warn('[editor] Tiptap init failed, switching to markdown mode:', err && err.message);
        self.fallbackToMarkdown('Tiptap 编辑器加载失败，已自动切换到 Markdown 模式。联网后刷新页面即可恢复富文本编辑。');
      });
    } else {
      this.fallbackToMarkdown('Tiptap 编辑器不可用（可能处于离线状态），已自动切换到 Markdown 模式。联网后刷新页面即可使用富文本编辑。');
    }
    document.addEventListener('selectionchange', function () {
      if (self.mode === 'rich' && self.tiptap) {
        if (self._selRAF) return;
        self._selRAF = requestAnimationFrame(function () {
          self._selRAF = null;
          self.onSelection();
        });
      }
    });
  },
  fallbackToMarkdown: function (msg) {
    this.mode = 'markdown';
    this.applyModeUI();
    this.loadLatest();
    if (typeof UI !== 'undefined') UI.toast(msg, 'warn');
  },
  onTiptapUpdate: function () {
    this.updateWordCountThrottled();
    this.refreshPreviewThrottled();
    this.autosaveDraft();
    this.scheduleDbSave();
    this.toggleEmptyState();
    this.resetIdleTimer();
    document.getElementById('draftSavedTag').style.display = 'none';
  },
  resetIdleTimer: function () {
    if (this._idleDisabled) return;
    var self = this;
    if (this._idleTimer) clearTimeout(this._idleTimer);
    this._idleTimer = setTimeout(function () { self.showIdleHints(); }, 30000);
  },
  // 真正关闭自动灵感提示（持久化，不再每 30 秒打扰）
  closeIdleHints: function () {
    this._idleDisabled = true;
    Store.set('idleHintsDisabled', true);
    if (this._idleTimer) { clearTimeout(this._idleTimer); this._idleTimer = null; }
    var hintEl = document.getElementById('pipeIntro');
    if (hintEl) hintEl.innerHTML = this._pipeIntroOriginal || '';
    UI.toast('已关闭自动灵感提示', '');
  },
  showIdleHints: function () {
    if (this._idleDisabled) return;
    if (!Store.state.currentProject || !Store.state.currentChapter) return;
    if (SSE.active) return;
    var ch = Store.state.currentChapter;
    var lastText = (ch.content || '').slice(-200);
    if (!lastText.trim()) return;
    var hints = [
      '「' + (Store.state.characters[0] ? Store.state.characters[0].name : '主角') + '」接下来会做出什么选择？',
      '是否能引入一个意外事件打破当前局面？',
      '试着从' + (Store.state.characters[1] ? Store.state.characters[1].name : '另一角色') + '的视角写一段内心独白',
      '当前场景的紧张度够不够？是否需要加冲突？',
      '前方是否有未回收的伏笔可以呼应？'
    ];
    var hintEl = document.getElementById('pipeIntro');
    if (hintEl) {
      if (!this._pipeIntroOriginal) this._pipeIntroOriginal = hintEl.innerHTML;
      var html = '<div style="font-weight:600;margin-bottom:4px;color:var(--accent)">💡 写作灵感</div>';
      for (var i = 0; i < 3; i++) {
        var h = hints[Math.floor(Math.random() * hints.length)];
        html += '<div style="padding:4px 0;font-size:11px;color:var(--muted);cursor:pointer;border-radius:4px;margin-bottom:2px" onclick="document.getElementById(\'instructionInput\').value=\'' + esc(h) + '\';UI.toast(\'灵感已填入输入框\',\'\')">' + (i + 1) + '. ' + h + '</div>';
      }
      html += '<div style="font-size:9px;color:var(--faint);margin-top:4px;cursor:pointer" onclick="Editor.closeIdleHints()">✕ 关闭自动提示</div>';
      hintEl.innerHTML = html;
    }
    UI.toast('光标停留30秒，已推送灵感提示', '');
  },
  applyModeUI: function () {
    var rich = this.elRich(), md = this.elMd();
    var richBtn = document.getElementById('modeRichBtn');
    var mdBtn = document.getElementById('modeMdBtn');
    if (this.mode === 'rich') {
      if (rich) rich.style.display = ''; if (md) md.style.display = 'none';
      if (richBtn) richBtn.classList.add('on');
      if (mdBtn) mdBtn.classList.remove('on');
    } else {
      if (rich) rich.style.display = 'none'; if (md) md.style.display = '';
      if (richBtn) richBtn.classList.remove('on');
      if (mdBtn) mdBtn.classList.add('on');
    }
    Store.state.editor.mode = this.mode;
    Store.savePrefs();
    this.updateWordCount();
  },
  setMode: function (m) {
    if (m === this.mode) return;
    if (m === 'markdown') {
      this.elMd().value = this.htmlToMd(this.getRichHTML());
    } else {
      var mdText = this.elMd().value;
      if (this.tiptap) {
        this.setRichHTML(this.mdToHtml(mdText));
      }
    }
    this.mode = m;
    this.applyModeUI();
    this.refreshPreview();
  },
  togglePreview: function () {
    this.preview = !this.preview;
    this.elPreview().classList.toggle('show', this.preview);
    // 顶栏与工具栏各有一个预览按钮，同步高亮态
    var btn1 = document.getElementById('previewBtn');
    if (btn1) btn1.classList.toggle('on', this.preview);
    var btn2 = document.getElementById('previewBtn2');
    if (btn2) btn2.classList.toggle('on', this.preview);
    // 阅读模式：自动折叠右面板 + 加大字体
    if (this.preview) {
      document.getElementById('rightPanel').classList.add('collapsed');
      document.querySelector('.editor-pane').classList.add('reading-mode');
    } else {
      document.getElementById('rightPanel').classList.remove('collapsed');
      document.querySelector('.editor-pane').classList.remove('reading-mode');
    }
    this.refreshPreview();
  },
  toggleFocus: function () {
    this.focusMode = !this.focusMode;
    document.getElementById('sidebar').classList.toggle('collapsed', this.focusMode);
    document.body.classList.toggle('sidebar-hidden', this.focusMode);
    document.getElementById('rightPanel').classList.toggle('collapsed', this.focusMode);
    document.body.classList.toggle('right-hidden', this.focusMode);
    document.querySelector('.quota-bar').classList.toggle('focus-hidden', this.focusMode);
    document.querySelector('.composer').classList.toggle('focus-mini', this.focusMode);
    document.getElementById('focusBtn').classList.toggle('on', this.focusMode);
    Store.set('focusMode', this.focusMode);
    if (this.focusMode) { UI.toast('专注模式已开启', ''); }
  },
  // 去AI味：检测 AI 痕迹 + 文字层润色（替换全文，支持撤销）
  deAIfy: async function () {
    var text = this.getText();
    if (!text || text.trim().length < 50) { UI.toast('正文太短（<50字），请先写一段内容', 'warn'); return; }
    if (this._deAIing) return;
    this._deAIing = true;
    var btn = document.getElementById('deAIfyBtn');
    if (btn) btn.disabled = true;
    try {
      UI.toast('🔍 正在检测 AI 味…', '');
      var tells = null;
      try { tells = await API.aiTells({ content: text }); } catch (e) { /* 检测失败不阻塞润色 */ }
      UI.toast('✨ 正在去 AI 味润色（约 10-60 秒）…', '');
      var r;
      try { r = await API.aiPolish({ content: text, language: 'zh' }); }
      catch (e) { UI.toast('润色失败：' + e.message, 'error'); return; }
      if (!r || !r.text || !r.text.trim()) { UI.toast('润色结果为空，请重试', 'error'); return; }
      // 撤销快照 + 替换全文
      this.undoContent = text;
      this.setContent(r.text);
      UI.toast('✅ 已去 AI 味润色（' + (r.model || '') + '）', 'success');
      // 展示检测报告（润色前检出问题才弹）
      if (tells && tells.issues && tells.issues.length) {
        var html = tells.issues.map(function (it) {
          var color = it.severity === 'warning' ? '#e6a23c' : '#909399';
          return '<div style="margin:5px 0;padding-left:4px;border-left:1px solid ' + color + '">' +
            '<b style="color:' + color + '">【' + it.category + '】</b> ' + it.description +
            '<div style="color:var(--muted);font-size:11px;margin:2px 0 0 10px">→ ' + (it.suggestion || '') + '</div></div>';
        }).join('');
        UI.modal({
          title: '🔍 AI味检测报告（润色前检出 ' + tells.count + ' 项）',
          body: '<div style="font-size:12px;line-height:1.7;max-height:300px;overflow:auto">' + html + '</div>',
          actions: [{ id: 'close', label: '知道了' }]
        });
      }
    } finally {
      this._deAIing = false;
      if (btn) btn.disabled = false;
    }
  },
  adjustFontSize: function (delta) {
    this._fs = Math.max(12, Math.min(28, (this._fs || Store.get('fontSize', 16)) + delta));
    var rich = document.querySelector('.editor-rich');
    var md = document.querySelector('.editor-md');
    if (rich) rich.style.fontSize = this._fs + 'px';
    if (md) md.style.fontSize = this._fs + 'px';
    Store.set('fontSize', this._fs);
  },
  refreshPreview: function () {
    if (!this.preview) return;
    this.elPreviewInner().innerHTML = this.mdToHtml(this.getText());
  },
  refreshPreviewThrottled: function () {
    if (!this.preview) return;
    var self = this;
    if (this._pvTimer) return;
    this._pvTimer = setTimeout(function () {
      self._pvTimer = null;
      self.refreshPreview();
    }, 500);
  },
  getRichHTML: function () {
    return this.tiptap ? this.tiptap.getHTML() : this.elRich().innerHTML;
  },
  setRichHTML: function (html) {
    if (this.tiptap) {
      this.tiptap.commands.setContent(html || '');
    } else {
      this.elRich().innerHTML = html || '';
    }
  },
  getText: function () {
    if (this.mode === 'rich') {
      return this.tiptap ? this.tiptap.getText() : this.elRich().innerText.replace(/\u00a0/g, ' ');
    }
    return this.elMd().value;
  },
  setContent: function (text) {
    var hasText = text && text.trim().length > 0;
    if (this.mode === 'rich') {
      this.setRichHTML(this.mdToHtml(text || ''));
      // Tiptap 异步渲染，先同步加 class 防止 ::before 遮罩挡住编辑器
      if (hasText) {
        var pane = document.querySelector('.editor-pane');
        if (pane) pane.classList.add('has-content');
      }
    } else {
      this.elMd().value = text || '';
    }
    this.updateWordCount();
    this.refreshPreview();
    this.autosaveDraft();
    if (!hasText) this.toggleEmptyState();
    this.updateEmptyGuide();
  },
  dismissGuide: function (ev) {
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var g = document.getElementById('emptyGuide');
    if (g) g.classList.remove('show');
    // 记住用户选择：本次会话不再自动弹出
    this._guideDismissed = true;
    try { sessionStorage.setItem('guideDismissed', '1'); } catch (e) {}
    UI.toast('引导已关闭', 'info');
  },
  guideDismissed: function () {
    if (this._guideDismissed) return true;
    try { return sessionStorage.getItem('guideDismissed') === '1'; } catch (e) { return false; }
  },
  toggleMoreMenu: function (ev) {
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var menu = document.getElementById('moreMenu');
    if (!menu) return;
    var showing = menu.style.display !== 'none';
    menu.style.display = showing ? 'none' : '';
    if (!showing) {
      // 打开「更多」菜单时，收起专业模式面板（两者互斥，避免悬浮层互相遮挡）
      var pp = document.getElementById('proModePanel');
      if (pp && pp.style.display !== 'none') {
        pp.style.display = 'none';
        var pb = document.getElementById('proModeBtn');
        if (pb) pb.classList.remove('on');
        try { Store.set('proModeOpen', false); } catch (e) {}
      }
      var close = function (e) {
        if (menu && !menu.contains(e.target) && e.target.id !== 'moreMenuWrap' && !e.target.closest('.more-menu-wrap')) {
          menu.style.display = 'none';
          document.removeEventListener('click', close, true);
        }
      };
      setTimeout(function () { document.addEventListener('click', close, true); }, 0);
    }
  },
  updateEmptyGuide: function () {
    var g = document.getElementById('emptyGuide');
    if (!g) return;
    var ch = Store.state.currentChapter;
    var hasText = false;
    if (this.mode === 'rich' && this.tiptap) {
      hasText = this.tiptap.state.doc.textContent.trim().length > 0;
    } else {
      hasText = this.getText().trim().length > 0;
    }
    // 无章节，或章节为空（无内容）时显示引导；用户主动关闭过则不显示
    var showGuide = (!ch || !hasText) && !this.guideDismissed();
    g.classList.toggle('show', showGuide);
  },
  toggleEmptyState: function () {
    var pane = document.querySelector('.editor-pane');
    if (!pane) return;
    var hasContent;
    if (this.mode === 'rich' && this.tiptap) {
      hasContent = this.tiptap.state.doc.textContent.trim().length > 0;
    } else {
      hasContent = this.getText().trim().length > 0;
    }
    pane.classList.toggle('has-content', hasContent);
  },
  getSelectionText: function () {
    if (this.mode === 'rich') {
      if (this.tiptap) {
        var _a = this.tiptap.state.selection;
        var from = _a.from, to = _a.to;
        return from === to ? '' : this.tiptap.state.doc.textBetween(from, to, ' ');
      }
      var sel = window.getSelection();
      return sel ? sel.toString() : '';
    }
    var ta = this.elMd();
    return ta.value.substring(ta.selectionStart, ta.selectionEnd);
  },
  getSelectedText: function () { return Store.state.editor.selectedText || ''; },
  onInput: function () {
    this.updateWordCountThrottled();
    this.refreshPreviewThrottled();
    this.autosaveDraft();
    this.scheduleDbSave();
    document.getElementById('draftSavedTag').style.display = 'none';
  },
  autosaveDraft: function () {
    var self = this;
    if (this._draftTimer) clearTimeout(this._draftTimer);
    this._draftTimer = setTimeout(function () {
      var p = Store.state.currentProject;
      if (p) Store.saveDraft(p.id, self.getText());
    }, 1500);
  },
  // 5 秒防抖自动保存到数据库（localStorage 草稿仍每次输入即时写入，双保险）
  scheduleDbSave: function () {
    var self = this;
    if (this._dbSaveTimer) clearTimeout(this._dbSaveTimer);
    this._dbSaveTimer = setTimeout(function () { self.saveToDb(); }, 5000);
  },
  saveToDb: function () {
    var ch = Store.state.currentChapter;
    if (!ch) return;
    var text = this.getText();
    if (text === (ch.content || '')) return;
    // 多窗口冲突检测：加载时记下 updated_at，保存前比对
    if (ch._saveVersion && this._conflictChecked !== ch.id) {
      this._conflictChecked = ch.id;
      var self = this;
      API.getChapter(ch.id).then(function (latest) {
        if (latest && latest.updated_at !== ch._saveVersion) {
          UI.confirm('内容冲突', '该章节在另一窗口已被修改。\n\n选择「覆盖」将丢失另一窗口的修改，选择「刷新」将加载最新版本。', function () {
            self._doSave(ch, text);
          }, '覆盖', '刷新', function () {
            var newText = latest.content || '';
            self.setContent(newText);
            if (ch) { ch.content = newText; ch.word_count = wordCount(newText); ch._saveVersion = latest.updated_at; }
            self.updateWordCount();
          });
        } else {
          self._doSave(ch, text);
        }
      }).catch(function () { self._doSave(ch, text); });
      return;
    }
    this._doSave(ch, text);
  },
  _doSave: function (ch, text) {
    ch.content = text;
    ch.word_count = wordCount(text);
    this._conflictChecked = null;
    var self = this;
    API.updateChapter(ch.id, { content: text, if_updated_at: ch._saveVersion }).then(function () {
      if (ch.content === text) ch._saveVersion = new Date().toISOString();
      self.flashSaved();
      self.updateStatusBar();
    }).catch(function (e) { console.warn('[save] auto-save failed:', e && e.message); });
  },
  // 保存成功提示：字数统计旁 + 底部状态栏各闪一下
  flashSaved: function () {
    var dt = document.getElementById('draftSavedTag');
    if (dt) {
      dt.style.display = '';
      if (this._dtTimer) clearTimeout(this._dtTimer);
      this._dtTimer = setTimeout(function () { dt.style.display = 'none'; }, 2500);
    }
    var sb = document.getElementById('sbSavedTag');
    if (sb) { sb.classList.remove('flash'); void sb.offsetWidth; sb.classList.add('flash'); }
  },
  // 底部状态栏：全书字数 / 本章字数
  updateStatusBar: function () {
    var sbTotal = document.getElementById('sbTotalWords');
    var sbCh = document.getElementById('sbChapterWords');
    if (!sbTotal || !sbCh) return;
    var cur = Store.state.currentChapter;
    var live = wordCount(this.getText() || '');
    sbCh.textContent = live.toLocaleString();
    var total = 0;
    (Store.state.chapters || []).forEach(function (c) {
      total += (cur && c.id === cur.id) ? live : (c.word_count || 0);
    });
    sbTotal.textContent = total.toLocaleString();
  },
  updateWordCount: function () {
    var t = this.getText();
    document.getElementById('wcNow').textContent = wordCount(t).toLocaleString();
    document.getElementById('charNow').textContent = charCount(t).toLocaleString();
    this.updateStatusBar();
  },
  updateWordCountThrottled: function () {
    var self = this;
    if (this._wcTimer) return;
    this._wcTimer = setTimeout(function () {
      self._wcTimer = null;
      self.updateWordCount();
    }, 500);
  },
  syncInstructionHeight: function (ta) {
    ta.style.height = 'auto';
    ta.style.height = Math.min(Math.max(ta.scrollHeight, 38), 140) + 'px';
  },
  loadLatest: function () {
    var ch = Store.state.currentChapter;
    if (ch && ch.content !== undefined) {
      this.setContent(ch.content || '');
    } else {
      var v = Store.state.latestVersion;
      this.setContent(v ? v.content : '');
    }
    // 确保编辑器可聚焦
    var self = this;
    setTimeout(function () {
      if (self.tiptap && self.mode === 'rich') {
        self.tiptap.commands.focus('start');
        document.querySelector('.editor-pane')?.classList.add('has-content');
      }
    }, 150);
  },
  execFmt: function (cmd) {
    if (this.mode === 'rich' && this.tiptap) {
      var chain = this.tiptap.chain().focus();
      switch (cmd) {
        case 'bold': chain.toggleBold().run(); break;
        case 'italic': chain.toggleItalic().run(); break;
        case 'underline': chain.toggleUnderline().run(); break;
      }
      return;
    }
    // 修复：markdown 模式（离线/Tiptap 降级/手动切换）下格式按钮同样生效，直接包裹 Markdown 语法
    if (this.mode === 'markdown') {
      var ta = this.elMd();
      if (!ta) return;
      var start = ta.selectionStart, end = ta.selectionEnd;
      var wrap = { bold: ['**', '**'], italic: ['*', '*'], underline: ['<u>', '</u>'] }[cmd];
      if (!wrap) return;
      var sel = ta.value.substring(start, end) || '加粗文字';
      var newVal = ta.value.substring(0, start) + wrap[0] + sel + wrap[1] + ta.value.substring(end);
      ta.value = newVal;
      ta.dispatchEvent(new Event('input', { bubbles: true }));
      ta.focus();
      ta.selectionStart = start + wrap[0].length;
      ta.selectionEnd = end + wrap[0].length + sel.length;
    }
  },
  insertH: function (prefix) {
    if (this.mode === 'rich' && this.tiptap) {
      this.tiptap.chain().focus().toggleHeading({ level: prefix === '## ' ? 2 : 3 }).run();
      return;
    }
    // 修复：markdown 模式在光标所在行首插入标题前缀
    if (this.mode === 'markdown') {
      var ta = this.elMd();
      if (!ta) return;
      var start = ta.selectionStart;
      var lineStart = ta.value.lastIndexOf('\n', start - 1) + 1;
      var newVal = ta.value.substring(0, lineStart) + prefix + ta.value.substring(lineStart);
      ta.value = newVal;
      ta.dispatchEvent(new Event('input', { bubbles: true }));
      ta.focus();
      ta.selectionStart = ta.selectionEnd = lineStart + prefix.length;
    }
  },
  getCursorPosition: function () {
    if (this.mode === 'rich' && this.tiptap) {
      return this.tiptap.state.selection.from;
    }
    if (this.mode === 'markdown') {
      return this.elMd().selectionStart;
    }
    return -1;
  },
  insertAtCursor: function (text) {
    if (this.mode === 'rich' && this.tiptap) {
      this.tiptap.chain().focus().insertContent(text).run();
      this.onInput();
    } else if (this.mode === 'markdown') {
      var ta = this.elMd();
      var pos = ta.selectionStart;
      ta.value = ta.value.substring(0, pos) + text + ta.value.substring(pos);
      ta.selectionStart = ta.selectionEnd = pos + text.length;
      this.onInput();
    }
  },
  appendToEnd: function (text) {
    if (this.mode === 'rich' && this.tiptap) {
      this.tiptap.chain().focus('end').insertContent('\n' + text).run();
      this.onInput();
    } else if (this.mode === 'markdown') {
      var ta = this.elMd();
      ta.value = ta.value + '\n' + text;
      this.onInput();
    }
  },
  appendStream: function (text) {
    if (!this.streaming) { this.streaming = true; this.streamText = ''; this._streamBuf = ''; }
    this.streamText = (this.streamText || '') + text;
    // 实时更新当前章节字数（节流：每 500ms 才重建章节树，避免每个 token 全量重绘）
    var ch = Store.state.currentChapter;
    if (ch && (!SSE._bindChapterId || ch.id === SSE._bindChapterId)) {
      ch.word_count = this.streamText.length;
      var self2 = this;
      if (!this._wcTreeTimer) {
        this._wcTreeTimer = setTimeout(function () {
          self2._wcTreeTimer = null;
          ChapterUI.renderTree();
        }, 500);
      }
    }
    this._streamBuf = (this._streamBuf || '') + text;
    // 每攒够 80 字或最多 150ms 才刷新到编辑器，避免每个 token 创建独立 <p>
    var self = this;
    if (this._streamTimer) clearTimeout(this._streamTimer);
    var doFlush = this._streamBuf.length >= 80;
    if (doFlush) {
      self._flushStreamBuf();
    }
    this._streamTimer = setTimeout(function () {
      self._flushStreamBuf();
    }, 150);
    this.updateWordCountThrottled();
    this.refreshPreviewThrottled();
    var cursor = document.querySelector('.cursor');
    if (!cursor) {
      cursor = document.createElement('span'); cursor.className = 'cursor';
      if (this.mode === 'rich') this.elRich().appendChild(cursor);
    }
  },
  _flushStreamBuf: function () {
    if (!this._streamBuf) return;
    var text = this._streamBuf;
    this._streamBuf = '';
    if (this._streamTimer) { clearTimeout(this._streamTimer); this._streamTimer = null; }
    // 生成中切换了章节：不写入当前 DOM，内容由 done 事件写入绑定章节
    var curCh = Store.state.currentChapter;
    if (SSE.active && SSE._bindChapterId && curCh && curCh.id !== SSE._bindChapterId) return;
    if (this.mode === 'rich' && this.tiptap) {
      // 首次插入：用 setContent 替换空文档，避免 insertContentAt 在空文档时创建多余 <p>
      var docSize = this.tiptap.state.doc.content.size;
      if (docSize <= 2) {
        this.tiptap.commands.setContent(text);
      } else {
        this.tiptap.chain().insertContentAt(docSize, text).run();
      }
    } else if (this.mode === 'markdown') {
      var ta = this.elMd();
      ta.value += text; ta.scrollTop = ta.scrollHeight;
    }
  },
  resetStream: function () {
    this.streamText = ''; this._streamBuf = '';
    if (this._streamTimer) { clearTimeout(this._streamTimer); this._streamTimer = null; }
    if (this.mode === 'rich' && this.tiptap) {
      this.tiptap.commands.setContent('');
    } else {
      this.elMd().value = '';
    }
  },
  endStream: function () {
    this._flushStreamBuf();
    this.streaming = false;
    var cursor = document.querySelector('.cursor');
    if (cursor) cursor.remove();
  },
  onSelection: function () {
    Store.state.editor.selectedText = this.getSelectionText();
    this.showSelToolbar();
  },
  showSelToolbar: function () {
    var sel = Store.state.editor.selectedText;
    var tb = document.getElementById('selToolbar');
    if (!tb) return;
    if (sel && sel.length > 0) {
      var rect;
      if (this.tiptap) {
        var _a = this.tiptap.state.selection;
        var from = _a.from, to = _a.to;
        if (from === to) { tb.style.display = 'none'; return; }
        var pos = this.tiptap.view.coordsAtPos(to);
        if (pos) {
          rect = { top: pos.top, bottom: pos.bottom, left: pos.left, right: pos.right };
        }
      }
      if (!rect) {
        try {
          var sel_ = window.getSelection();
          if (sel_ && sel_.rangeCount) rect = sel_.getRangeAt(0).getBoundingClientRect();
        } catch (e) {}
      }
      if (!rect) return;
      tb.style.display = 'flex';
      tb.style.position = 'fixed';
      tb.style.top = Math.max(8, rect.top - 48) + 'px';
      tb.style.left = Math.max(8, rect.left) + 'px';
    }
  },
  hideSelToolbar: function () {
    var tb = document.getElementById('selToolbar');
    if (tb && tb.style.display !== 'none') {
      setTimeout(function () { if (!Store.state.editor.selectedText) tb.style.display = 'none'; }, 200);
    }
  },
  scrollToEnd: function () {
    var self = this;
    setTimeout(function () {
      if (self.mode === 'rich' && self.tiptap) {
        var dom = self.tiptap.view.dom;
        dom.scrollTop = dom.scrollHeight;
      } else {
        var ta = self.elMd();
        ta.scrollTop = ta.scrollHeight;
      }
    }, 100);
  },
  cleanAIFiller: function () {
    if (!Store.state.currentProject) { UI.toast('请先选择项目', 'warn'); return; }
    var s = this.getText();
    s = s.replace(/[总而言之|综上所述|此外|另外|值得注意的是|值得一提的是|不得不说|毫无疑问|毋庸置疑][，,]/g, '');
    s = s.replace(/[我们|咱们|大家|各位读者|各位看官][可以|能][看到|看出|发现|注意到][，,]?/g, '');
    s = s.replace(/[在|从][这个|某种|某]?[意义|角度|层面]上[来说|讲]，?/g, '');
    this.setContent(s);
    UI.toast('已清理 AI 冗余话术', 'success');
  },
  importFile: function () {
    if (!Store.state.currentProject) { UI.toast('请先选择或创建项目', 'warn'); return; }
    var idf = 'if_' + uid();
    UI.modal({
      title: '导入稿件',
      sub: '支持 TXT / Markdown / Word / HTML / ePub / RTF 文件。勾选「自动分割章节」将按标题标记自动拆分为多个章节。',
      body: '<div class="form-group"><label class="btn btn-ghost btn-sm" style="cursor:pointer;display:inline-block">选择文件…' +
        '<input type="file" id="' + idf + '" accept=".txt,.md,.markdown,.docx,.html,.htm,.epub,.rtf,.doc" style="display:none" onchange="Editor.handleImport(this,\'' + idf + '\')"></label>' +
        '<span id="' + idf + '_n" style="font-size:11.5px;color:var(--muted);margin-left:10px"></span></div>' +
        '<div class="form-group"><label>或粘贴文本</label><textarea id="' + idf + '_t" rows="8"></textarea></div>' +
        '<div class="import-progress" id="' + idf + '_prog" style="display:none"><div class="pipe-track"><i id="' + idf + '_bar" style="width:0%"></i></div><span style="font-size:10px;color:var(--muted)" id="' + idf + '_progText"></span></div>' +
        '<div style="display:flex;align-items:center;gap:6px"><input type="checkbox" id="' + idf + '_split"><label for="' + idf + '_split">自动分割章节</label></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '导入', cls: 'btn-primary', onClick: function (m, ov) {
          var txt = document.getElementById(idf + '_t').value;
          var doSplit = document.getElementById(idf + '_split').checked;
          if (!txt.trim()) { UI.toast('请选择文件或粘贴文本', 'warn'); return; }
          if (doSplit) { ov.remove(); ChapterUI.splitImport(txt); }
          else { Editor.setContent(txt); UI.toast('已导入', 'success'); ov.remove(); }
        }}
      ]
    });
  },
  handleImport: async function (input, idf) {
    var f = input.files[0]; if (!f) return;
    var labelEl = document.getElementById(idf + '_n');
    var textArea = document.getElementById(idf + '_t');
    var ext = f.name.toLowerCase().split('.').pop();
    // 需要后端解析的格式
    var backendExts = ['docx', 'doc', 'epub', 'html', 'htm', 'rtf'];
    if (backendExts.indexOf(ext) >= 0) {
      labelEl.textContent = '⏳ 正在解析文档…';
      labelEl.style.color = '';
      try {
        var fd = new FormData();
        fd.append('project_id', Store.state.currentProject.id);
        fd.append('file', f);
        fd.append('name', '_import_tmp_' + Date.now());
        var item = await API.uploadMaterial(fd);
        if (item && item.id) await API.deleteMaterial(item.id);
        textArea.value = item.content || '';
        labelEl.textContent = f.name + ' ✓ 已加载，请确认内容并选择是否分割章节，点击导入完成操作';
        labelEl.style.color = 'var(--success)';
      } catch (e) { labelEl.textContent = '解析失败：' + e.message; labelEl.style.color = 'var(--danger)'; }
      return;
    }
    // txt / md 本地读取
    var reader = new FileReader();
    reader.onload = function () {
      textArea.value = reader.result || '';
      labelEl.textContent = f.name + ' ✓ 已加载，请确认内容并选择是否分割章节，点击导入完成操作';
      labelEl.style.color = 'var(--success)';
    };
    reader.onerror = function () { labelEl.textContent = '文件读取失败'; labelEl.style.color = 'var(--danger)'; };
    reader.readAsText(f);
  },
  exportFile: function () {
    var p = Store.state.currentProject;
    var text = this.getText();
    UI.modal({
      title: '导出格式选择',
      body: '<div class="form-group"><label>导出格式</label><select id="exportFmt"><option value="txt">纯文本 TXT</option><option value="md">Markdown</option><option value="html">HTML 网页</option><option value="docx">Word 文档</option><option value="epub">EPUB 电子书</option><option value="pdf">PDF 文档</option></select></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '导出', cls: 'btn-primary', onClick: function (m, ov) {
          var fmt = document.getElementById('exportFmt').value;
          if (fmt === 'epub') {
            ov.remove();
            Editor.exportEPUB();
            return;
          }
          if (fmt === 'docx') {
            // 使用服务端真 OOXML 导出（正确的 Open XML 格式）
            ov.remove();
            var p = Store.state.currentProject;
            if (p) {
              var a = document.createElement('a');
              a.href = '/api/export/docx?project_id=' + p.id;
              a.download = (p.name || 'output') + '.docx';
              a.click();
              UI.toast('DOCX 导出中…', 'success');
            }
            return;
          }
          if (fmt === 'pdf') {
            var w = window.open('', '_blank');
            if (!w) {
              UI.toast('浏览器拦截了弹窗，请允许本站弹出窗口后重试导出 PDF', 'error');
              return;
            }
            w.document.write('<!DOCTYPE html><html><head><meta charset="UTF-8"><title>' + esc(p ? p.name : 'document') + '</title><style>body{max-width:760px;margin:40px auto;font-size:16px;line-height:2.1;font-family:"PingFang SC","Microsoft YaHei",sans-serif}p{margin:.4em 0}h1,h2,h3{margin:.6em 0 .3em}</style></head><body>' + Editor.mdToHtml(text) + '</body></html>');
            w.document.close(); setTimeout(function () { w.print(); }, 500); ov.remove();
            return;
          }
          var content, mime, ext_;
          if (fmt === 'md') { content = text; mime = 'text/markdown;charset=utf-8'; ext_ = '.md'; }
          else if (fmt === 'html') { content = '<!DOCTYPE html><html><head><meta charset="UTF-8"><title>' + esc(p ? p.name : 'document') + '</title><style>body{max-width:760px;margin:40px auto;font-size:16px;line-height:2.1;font-family:"PingFang SC","Microsoft YaHei",sans-serif;color:#1a1f2e}p{margin:.4em 0}h1,h2,h3{margin:.6em 0 .3em}</style></head><body>' + Editor.mdToHtml(text) + '</body></html>'; mime = 'text/html;charset=utf-8'; ext_ = '.html'; }
          else { content = text; mime = 'text/plain;charset=utf-8'; ext_ = '.txt'; }
          var blob = new Blob([content], { type: mime });
          var a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = (p ? p.name : 'output') + ext_; a.click();
          URL.revokeObjectURL(a.href); ov.remove(); UI.toast('导出成功', 'success');
        }}
      ]
    });
  },
  insertGeneratedAtCursor: function () {
    if (SSE.streamText) { this.insertAtCursor(SSE.streamText); SSE.streamText = ''; this.syncChapterContent(); document.getElementById('genActions').style.display = 'none'; UI.toast('已插入光标位置', 'success'); }
  },
  appendGeneratedToEnd: function () {
    if (SSE.streamText) { this.appendToEnd(SSE.streamText); SSE.streamText = ''; this.syncChapterContent(); document.getElementById('genActions').style.display = 'none'; UI.toast('已追加到末尾', 'success'); }
  },
  discardGenerated: function () {
    SSE.streamText = ''; document.getElementById('genActions').style.display = 'none'; UI.toast('已丢弃', 'warn');
  },
  undoGenerated: function () {
    if (this.undoContent) { this.setContent(this.undoContent); this.syncChapterContent(); this.undoContent = ''; }
    document.getElementById('genActions').style.display = 'none'; UI.toast('已撤销本次生成', 'warn');
  },
  exportEPUB: function () {
    var p = Store.state.currentProject;
    var chs = Store.state.chapters || [];
    if (!chs.length) { UI.toast('暂无章节可导出', 'warn'); return; }
    var name = (p ? p.name : 'output');
    var htmlChapters = chs.map(function(c, i) {
      return '<h1>' + esc(c.title) + '</h1>\n' + Editor.mdToHtml(c.content || '');
    }).join('\n');
    var epubHTML = '<?xml version="1.0" encoding="UTF-8"?>\n' +
      '<!DOCTYPE html>\n<html xmlns="http://www.w3.org/1999/xhtml">\n<head><meta charset="UTF-8"/><title>' + esc(name) + '</title></head>\n<body>\n' +
      htmlChapters + '\n</body>\n</html>';
    var opf = '<?xml version="1.0"?>\n<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="book-id" version="3.0">\n' +
      '<metadata><dc:title xmlns:dc="http://purl.org/dc/elements/1.1/">' + esc(name) + '</dc:title><dc:identifier id="book-id">urn:uuid:' + (p ? p.id : '00000000') + '</dc:identifier></metadata>\n' +
      '<manifest><item id="content" href="content.xhtml" media-type="application/xhtml+xml"/></manifest>\n' +
      '<spine><itemref idref="content"/></spine>\n</package>';
    var container = '<?xml version="1.0"?>\n<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">\n' +
      '<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>\n</container>';
    // 极简 ZIP 实现（EPUB = ZIP 文件）
    var files = [
      {name:'mimetype', data:'application/epub+zip', compress:false},
      {name:'META-INF/container.xml', data:container},
      {name:'OEBPS/content.opf', data:opf},
      {name:'OEBPS/content.xhtml', data:epubHTML}
    ];
    var blob = Editor._buildZip(files);
    var a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = name + '.epub'; a.click();
    URL.revokeObjectURL(a.href);
    UI.toast('EPUB 已导出', 'success');
  },
  _buildZip: function (files) {
    var buf = []; var offsets = []; var dirOffset = 0;
    var encoder = new TextEncoder();
    for (var i = 0; i < files.length; i++) {
      var f = files[i];
      var data = encoder.encode(f.data);
      var nameBytes = encoder.encode(f.name);
      var crc = Editor._crc32(data);
      var compressed = new Uint8Array(data); // no compression for simplicity
      var method = 0; // stored
      offsets.push(buf.reduce(function(s, a) { return s + a.length; }, 0));
      var localHeader = new Uint8Array(30 + nameBytes.length);
      var lv = new DataView(localHeader.buffer);
      lv.setUint32(0, 0x04034b50, true);
      lv.setUint16(8, method, true);
      lv.setUint32(14, crc, true);
      lv.setUint32(18, compressed.length, true);
      lv.setUint32(22, data.length, true);
      lv.setUint16(26, nameBytes.length, true);
      localHeader.set(nameBytes, 30);
      buf.push(localHeader, compressed);
    }
    dirOffset = buf.reduce(function(s, a) { return s + a.length; }, 0);
    var dirEntries = [];
    for (var i = 0; i < files.length; i++) {
      var f_ = files[i];
      var data_ = encoder.encode(f_.data);
      var nameBytes_ = encoder.encode(f_.name);
      var crc_ = Editor._crc32(data_);
      var entry = new Uint8Array(46 + nameBytes_.length);
      var dv = new DataView(entry.buffer);
      dv.setUint32(0, 0x02014b50, true);
      dv.setUint32(16, crc_, true);
      dv.setUint32(20, data_.length, true);
      dv.setUint32(24, data_.length, true);
      dv.setUint16(28, nameBytes_.length, true);
      dv.setUint32(42, offsets[i], true);
      entry.set(nameBytes_, 46);
      dirEntries.push(entry);
    }
    var dirSize = dirEntries.reduce(function(s, e) { return s + e.length; }, 0);
    var eocd = new Uint8Array(22);
    var ev = new DataView(eocd.buffer);
    ev.setUint32(0, 0x06054b50, true);
    ev.setUint16(8, files.length, true);
    ev.setUint16(10, files.length, true);
    ev.setUint32(12, dirSize, true);
    ev.setUint32(16, dirOffset, true);
    var all = new Uint8Array(dirOffset + dirSize + 22);
    var pos = 0;
    for (var i = 0; i < buf.length; i++) { all.set(buf[i], pos); pos += buf[i].length; }
    for (var i = 0; i < dirEntries.length; i++) { all.set(dirEntries[i], pos); pos += dirEntries[i].length; }
    all.set(eocd, pos);
    return new Blob([all], {type:'application/epub+zip'});
  },
  _crc32: function (data) {
    var table = Editor._crcTable || (Editor._crcTable = (function(){
      var t = []; for (var n = 0; n < 256; n++) { var c = n; for (var k = 0; k < 8; k++) { c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1); } t[n] = c; }
      return t;
    })());
    var crc = 0xFFFFFFFF;
    for (var i = 0; i < data.length; i++) crc = (crc >>> 8) ^ table[(crc ^ data[i]) & 0xFF];
    return (crc ^ 0xFFFFFFFF) >>> 0;
  },
  syncChapterContent: function () {
    var ch = Store.state.currentChapter;
    if (ch) { ch.content = this.getText(); API.updateChapter(ch.id, { content: ch.content }).catch(function () {}); }
  },
  // HTML <-> Markdown 互转（轻量实现）
  htmlToMd: function (html) {
    if (!html) return '';
    var md = html;
    md = md.replace(/<h1[^>]*>([\s\S]*?)<\/h1>/gi, '\n# $1\n');
    md = md.replace(/<h2[^>]*>([\s\S]*?)<\/h2>/gi, '\n## $1\n');
    md = md.replace(/<h3[^>]*>([\s\S]*?)<\/h3>/gi, '\n### $1\n');
    md = md.replace(/<strong[^>]*>([\s\S]*?)<\/strong>/gi, '**$1**');
    md = md.replace(/<b[^>]*>([\s\S]*?)<\/b>/gi, '**$1**');
    md = md.replace(/<em[^>]*>([\s\S]*?)<\/em>/gi, '*$1*');
    md = md.replace(/<i[^>]*>([\s\S]*?)<\/i>/gi, '*$1*');
    md = md.replace(/<u[^>]*>([\s\S]*?)<\/u>/gi, '_$1_');
    md = md.replace(/<p[^>]*>/gi, ''); md = md.replace(/<\/p>/gi, '\n\n');
    md = md.replace(/<br\s*\/?>/gi, '\n');
    md = md.replace(/<[^>]+>/g, '');
    md = md.replace(/&nbsp;/g, ' ').replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>');
    return md.replace(/\n{3,}/g, '\n\n').trim();
  },
  mdToHtml: function (md) {
    if (!md) return '';
    var html = md.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    // Markdown 标题转 HTML（必须在段落包装之前执行）
    html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
    html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
    html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
    // 段落包装
    html = html.replace(/\n\n/g, '</p><p>');
    html = html.replace(/\n/g, '<br>');
    html = '<p>' + html + '</p>';
    html = html.replace(/<p><\/p>/g, '');
    // 行内格式
    html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/\*(.+?)\*/g, '<em>$1</em>');
    html = html.replace(/_(.+?)_/g, '<u>$1</u>');
    return html;
  }
};

