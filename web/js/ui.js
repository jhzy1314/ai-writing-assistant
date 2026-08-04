/* ============ ui.js：通用 UI（toast / modal / 主题 / 折叠 / 右键菜单） ============ */
var UI = {
  toast: function (msg, type, opts) {
    var wrap = document.getElementById('toastWrap');
    var el = document.createElement('div');
    el.className = 'toast ' + (type || '');
    el.textContent = msg;
    wrap.appendChild(el);
    if (!this._toasts) this._toasts = [];
    this._toasts.unshift({ msg: msg, type: type || '', time: new Date() });
    if (this._toasts.length > 50) this._toasts.length = 50;
    var duration = (opts && opts.duration) || 3200;
    setTimeout(function () {
      el.style.opacity = '0'; el.style.transition = 'opacity .3s';
      setTimeout(function () { el.remove(); }, 300);
    }, duration);
    return el;
  },
  showToastHistory: function () {
    var toasts = this._toasts || [];
    if (!toasts.length) { this.toast('暂无通知历史', ''); return; }
    var html = '<div style="max-height:300px;overflow-y:auto">';
    toasts.forEach(function (t) {
      var icon = t.type === 'error' ? '❌' : t.type === 'success' ? '✅' : t.type === 'warn' ? '⚠️' : 'ℹ️';
      var time = t.time.toLocaleTimeString('zh-CN', { hour12: false });
      html += '<div style="padding:4px 0;border-bottom:1px solid var(--border);font-size:11px;display:flex;gap:6px"><span style="opacity:.6">' + time + '</span>' + icon + ' ' + esc(t.msg) + '</div>';
    });
    html += '</div>';
    this.modal({ title: '通知历史（最近 50 条）', body: html, actions: [{ id: 'close', label: '关闭' }] });
  },
  /* 新手指南：从零开始使用 */
  showGuide: function () {
    var step = function (n, t, d) {
      return '<div class="guide-step"><span class="guide-num">' + n + '</span><div><b>' + t + '</b><div class="guide-desc">' + d + '</div></div></div>';
    };
    var body =
      '<div class="guide-wrap">' +
      step('1', '配置 AI 模型（必做）', '点击右上角「🔑 API」，填入任一家 API 密钥并选择模型（推荐 DeepSeek / 智谱）。没有密钥时 AI 生成不可用，但本地写作、保存、导出不受影响。') +
      step('2', '新建项目', '左侧「＋ 新建项目」，填写书名与题材。项目支持随时切换、归档，全部数据保存在本地数据库。') +
      step('3', '写下需求，一键生成', '在底部输入框描述本章剧情（例如“主角在拍卖会上打脸反派”），点击「✨ 生成」。AI 自动完成 构思大纲 → 动笔写作 → 审稿修正 全流程。') +
      step('4', '专业模式：详细设定', '点「⚡ 专业模式」填写书名 / 主角 / 世界观 / 分卷等 8 项设定，每一项都有「✨AI」按钮可让 AI 按严格约束生成建议；也可一键「🤖 AI 自动生成大纲」。') +
      step('5', '管理设定与素材', '左侧导航：🌳 章节大纲 / 👤 人物卡 / 🌍 世界观 / 🧰 AI工具箱 / 📊 仪表盘。工具栏「⋯ 更多」还有 伏笔检查 / 角色互动 / 剧情分支 等 AI 辅助工具。') +
      step('6', '保存与导出', '内容自动保存；工具栏「📥 导入文档 / 📤 导出文档」支持 docx / md / txt。右上角「🧼 纯净」可切换专注模式。') +
      '</div>';
    this.modal({
      title: '📖 新手指南 · 三步上手 AI辅助写作助手',
      wide: '620px',
      body: body,
      actions: [{ id: 'ok', label: '开始创作 ✍️', cls: 'btn-primary' }]
    });
    // 已看过指南，不再自动弹出
    try { localStorage.setItem('guideSeen', '1'); } catch (e) {}
  },
  /* 首次运行自动弹出新手指南（仅当无项目且未看过） */
  maybeShowGuide: function () {
    try {
      if (localStorage.getItem('guideSeen')) return;
      if (Store.state.projects && Store.state.projects.length) return;
      var self = this;
      setTimeout(function () { self.showGuide(); }, 900);
    } catch (e) {}
  },
  modal: function (opts) {
    var root = document.getElementById('modalRoot');
    // 单例守卫：已有弹窗则先关闭（防止快速双击叠加多个重叠弹窗）
    if (root) {
      while (root.firstChild) root.removeChild(root.firstChild);
    }
    var ov = document.createElement('div');
    ov.className = 'modal-overlay';
    var m = document.createElement('div');
    m.className = 'modal';
    if (opts.wide) m.style.width = opts.wide;
    var html = '<h3>' + esc(opts.title) + '</h3>';
    // 统一右上角 ✕ 关闭按钮：所有弹窗都有显式关闭入口（点击有反馈）
    html += '<span class="modal-x" title="关闭">✕</span>';
    if (opts.sub) html += '<div class="modal-sub">' + opts.sub + '</div>';
    if (opts.body) html += '<div class="modal-body">' + opts.body + '</div>';
    if (opts.actions) {
      html += '<div class="modal-actions">';
      opts.actions.forEach(function (a) {
        html += '<button class="btn ' + (a.cls || 'btn-ghost') + '" data-act="' + esc(a.id || '') + '">' + esc(a.label) + '</button>';
      });
      html += '</div>';
    }
    m.innerHTML = html;
    // ✕ 关闭按钮：关闭弹窗并给出反馈
    var mx = m.querySelector('.modal-x');
    if (mx) {
      mx.onclick = function (e) {
        e.stopPropagation();
        ov.remove();
      };
    }
    ov.appendChild(m);
    // 默认点击遮罩关闭；opts.noOverlayClose=true 时禁止（等待类弹窗防误关）
    if (!opts.noOverlayClose) {
      ov.onclick = function (e) { if (e.target === ov) ov.remove(); };
    }
    if (opts.actions) {
      m.querySelectorAll('[data-act]').forEach(function (btn) {
        btn.onclick = function () {
          var a = opts.actions.find(function (x) { return x.id === btn.dataset.act; });
          if (a && a.onClick) { a.onClick(m, ov); }
          // 没有自定义逻辑的按钮（取消/关闭等）默认关闭弹窗
          else { ov.remove(); }
        };
      });
    }
    root.appendChild(ov);
    return { overlay: ov, modal: m };
  },
  confirm: function (title, msg, onOk) {
    this.modal({
      title: title, sub: msg,
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '确认', cls: 'btn-primary', onClick: function (m, ov) { ov.remove(); if (onOk) onOk(); } }
      ]
    });
  },
  prompt: function (title, label, defVal, onOk) {
    var id = 'p_' + uid();
    this.modal({
      title: title,
      body: '<div class="form-group"><label>' + esc(label) + '</label><input id="' + id + '" value="' + esc(defVal || '') + '"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '确认', cls: 'btn-primary', onClick: function (m, ov) { var v = document.getElementById(id).value.trim(); ov.remove(); if (onOk) onOk(v); } }
      ]
    });
    setTimeout(function () { var el = document.getElementById(id); if (el) { el.focus(); el.select(); } }, 60);
  },
  toggleSidebar: function () {
    var sidebar = document.getElementById('sidebar');
    sidebar.classList.toggle('collapsed');
    document.body.classList.toggle('sidebar-hidden', sidebar.classList.contains('collapsed'));
    Store.set('sidebarCollapsed', sidebar.classList.contains('collapsed'));
    // 明确的折叠/展开反馈（用户可感知）
    try {
      UI.toast(sidebar.classList.contains('collapsed') ? '已收起侧栏，点击左侧橙色按钮可展开' : '已展开侧栏', 'info');
    } catch (e) {}
  },
  toggleRight: function () {
    var right = document.getElementById('rightPanel');
    right.classList.toggle('collapsed');
    document.body.classList.toggle('right-hidden', right.classList.contains('collapsed'));
    Store.set('rightCollapsed', right.classList.contains('collapsed'));
    // 明确的折叠/展开反馈（用户可感知）
    try {
      UI.toast(right.classList.contains('collapsed') ? '已收起右侧面板（编辑区已加宽）' : '已展开右侧面板', 'info');
    } catch (e) {}
  },
  toggleTheme: function () {
    // 快捷切换：经典深色 <-> 经典浅色（多主题系统由 Themes 接管）
    if (typeof Themes !== 'undefined' && Themes.toggle) { Themes.toggle(); return; }
    var cur = document.documentElement.getAttribute('data-theme');
    var next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    var btn = document.getElementById('themeToggle');
    if (btn) btn.textContent = next === 'dark' ? '☀ 浅色' : '◑ 深色';
    Store.set('theme', next);
  },
  initTheme: function () {
    if (typeof Themes !== 'undefined' && Themes.init) { Themes.init(); return; }
    var saved = Store.get('theme', 'light');
    if (saved === 'dark') {
      document.documentElement.setAttribute('data-theme', 'dark');
      var btn = document.getElementById('themeToggle');
      if (btn) btn.textContent = '☀ 浅色';
    }
  },
  setFontSize: function (size) {
    document.documentElement.style.setProperty('--font-size', size + 'px');
    var lbl = document.getElementById('fontSizeLabel');
    if (lbl) lbl.textContent = size;
    Store.set('fontSize', size);
  },
  initFontSize: function () {
    var size = Store.get('fontSize', '15');
    var slider = document.getElementById('fontSizeSlider');
    if (slider) slider.value = size;
    document.documentElement.style.setProperty('--font-size', size + 'px');
    var lbl = document.getElementById('fontSizeLabel');
    if (lbl) lbl.textContent = size;
  },
  toggleClean: function () {
    var body = document.body;
    var btn = document.getElementById('cleanToggle');
    var isClean = body.classList.toggle('clean-mode');
    if (btn) {
      btn.textContent = isClean ? '🧼 装饰' : '🧼 纯净';
      btn.style.opacity = isClean ? '.6' : '1';
    }
    Store.set('cleanMode', isClean);
  },
  closeCtx: function () { var m = document.getElementById('ctxMenu'); if (m) m.remove(); },
  ctxMenu: function (e, items) {
    e.preventDefault();
    this.closeCtx();
    var m = document.createElement('div');
    m.className = 'ctx-menu'; m.id = 'ctxMenu';
    m.style.left = e.clientX + 'px'; m.style.top = e.clientY + 'px';
    var html = '';
    items.forEach(function (it) {
      if (it.divider) { html += '<div class="ctx-menu-divider"></div>'; }
      else { html += '<div class="ctx-menu-item' + (it.danger ? ' danger' : '') + '" data-act="' + esc(it.id || '') + '">' + esc(it.label) + '</div>'; }
    });
    m.innerHTML = html;
    if (e.clientX + 170 > window.innerWidth) m.style.left = (e.clientX - 170) + 'px';
    if (e.clientY + items.length * 34 + 20 > window.innerHeight) m.style.top = (e.clientY - items.length * 34) + 'px';
    document.body.appendChild(m);
    m.querySelectorAll('[data-act]').forEach(function (el) {
      el.onclick = function () {
        var it = items.find(function (x) { return x.id === el.dataset.act; });
        UI.closeCtx();
        if (it && it.onClick) it.onClick();
      };
    });
    return false;
  },
  showOnboarding: function () {
    if (Store.get('onboarded', false)) return;
    var steps = [
      {el:'.side-new button', tip:'📁 从这里创建你的第一本小说', pos:'bottom'},
      {el:'#searchInput', tip:'🔍 在这里搜索项目、章节和资源', pos:'bottom'},
      {el:'#instructionInput', tip:'💡 输入创作需求，Ctrl+Enter 即可启动 AI 生成', pos:'top'},
      {el:'#modeSelect', tip:'⚙️ 选择创作模式：智能协同适合95%的场景', pos:'bottom'},
      {el:'#pipeIntro', tip:'📊 右侧面板实时展示AI流水线进度、大纲和审核结果', pos:'left'},
      {el:'#contextScope', tip:'📖 调整上下文范围：当前章/最近内容/含摘要', pos:'bottom'},
      {el:'#quotaTokens', tip:'📈 悬停查看各角色（构思/写作/审稿）的 Token 消耗明细', pos:'bottom'},
      {el:'button[title~="专注模式"]', tip:'🎯 专注模式：折叠侧边栏，全屏沉浸创作', pos:'bottom'}
    ];
    var idx = 0;
    var self = this;
    var tipEl = null;
    function dismiss() {
      Store.set('onboarded', true);
      if (tipEl) { tipEl.remove(); tipEl = null; }
    }
    function showStep() {
      if (idx >= steps.length) { Store.set('onboarded', true); return; }
      var s = steps[idx];
      var el = document.querySelector(s.el);
      if (!el) { idx++; showStep(); return; }
      if (tipEl) tipEl.remove();
      var rect = el.getBoundingClientRect();
      tipEl = document.createElement('div');
      tipEl.className = 'onboard-tip';
      var stepNum = (idx + 1) + '/' + steps.length;
      tipEl.innerHTML = '<span class="ob-step">' + stepNum + '</span> ' + s.tip +
        '<span class="ob-skip" style="display:block;margin-top:6px;font-size:10px;opacity:0.7;text-decoration:underline;cursor:pointer">不再提示</span>';
      tipEl.querySelector('.ob-skip').onclick = function (e) { e.stopPropagation(); dismiss(); };
      tipEl.style.left = rect.left + 'px';
      tipEl.style.top = (s.pos === 'top' ? rect.top - 60 : rect.bottom + 8) + 'px';
      tipEl.onclick = function () { idx++; showStep(); };
      document.body.appendChild(tipEl);
    }
    setTimeout(showStep, 800);
  },
  showKeyboardRef: function () {
    UI.modal({
      title: '⌨️ 快捷键参考',
      wide: '520px',
      body: '<table style="width:100%;font-size:11px;line-height:2.1">' +
        '<tr style="color:var(--accent)"><td colspan="2"><b>🚀 创作</b></td></tr>' +
        '<tr><td style="width:45%"><kbd>Ctrl+Enter</kbd></td><td>发送创作需求 / 开始生成</td></tr>' +
        '<tr><td><kbd>Esc</kbd></td><td>终止生成 / 关闭弹窗</td></tr>' +
        '<tr style="color:var(--accent)"><td colspan="2"><b>💾 保存与管理</b></td></tr>' +
        '<tr><td><kbd>Ctrl+S</kbd></td><td>保存当前章节到数据库</td></tr>' +
        '<tr><td><kbd>Ctrl+Shift+S</kbd></td><td>另存为版本快照</td></tr>' +
        '<tr style="color:var(--accent)"><td colspan="2"><b>✏️ 编辑</b></td></tr>' +
        '<tr><td><kbd>Ctrl+Z</kbd> / <kbd>Ctrl+Y</kbd></td><td>撤销 / 重做</td></tr>' +
        '<tr><td><kbd>Ctrl+B</kbd> / <kbd>I</kbd> / <kbd>U</kbd></td><td>粗体 / 斜体 / 下划线</td></tr>' +
        '<tr style="color:var(--accent)"><td colspan="2"><b>🔍 导航</b></td></tr>' +
        '<tr><td><kbd>Ctrl+F</kbd></td><td>全文搜索章节</td></tr>' +
        '<tr><td><kbd>Ctrl+Shift+F</kbd></td><td>选中文字一键润色</td></tr>' +
        '<tr style="color:var(--accent)"><td colspan="2"><b>🎨 界面</b></td></tr>' +
        '<tr><td><kbd>Ctrl+Shift+P</kbd></td><td>专注模式（折叠侧栏）</td></tr>' +
        '<tr><td><kbd>?</kbd></td><td>显示此快捷键参考</td></tr></table>' +
        '<div style="margin-top:8px;font-size:10px;color:var(--muted);text-align:center">💡 更多功能请查看右侧工具面板</div>',
      actions: [{ id: 'close', label: '关闭' }]
    });
  }
};
document.addEventListener('click', function () { UI.closeCtx(); });
document.addEventListener('scroll', function () { UI.closeCtx(); }, true);
