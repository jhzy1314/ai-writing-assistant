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
  modal: function (opts) {
    var root = document.getElementById('modalRoot');
    var ov = document.createElement('div');
    ov.className = 'modal-overlay';
    var m = document.createElement('div');
    m.className = 'modal';
    if (opts.wide) m.style.width = opts.wide;
    var html = '<h3>' + esc(opts.title) + '</h3>';
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
    ov.appendChild(m);
    ov.onclick = function (e) { if (e.target === ov) ov.remove(); };
    if (opts.actions) {
      m.querySelectorAll('[data-act]').forEach(function (btn) {
        btn.onclick = function () {
          var a = opts.actions.find(function (x) { return x.id === btn.dataset.act; });
          if (a && a.onClick) { a.onClick(m, ov); }
          else if (a && a.id === 'cancel') { ov.remove(); }
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
  },
  toggleRight: function () {
    var right = document.getElementById('rightPanel');
    right.classList.toggle('collapsed');
    Store.set('rightCollapsed', right.classList.contains('collapsed'));
  },
  toggleTheme: function () {
    var cur = document.documentElement.getAttribute('data-theme');
    var next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    var btn = document.getElementById('themeToggle');
    if (btn) btn.textContent = next === 'dark' ? '☀ 浅色' : '◑ 深色';
    Store.set('theme', next);
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
      {el:'#page-pipeline .pipe-intro', tip:'📊 右侧面板实时展示AI流水线进度、大纲和审核结果', pos:'left'},
      {el:'#contextScope', tip:'📖 调整上下文范围：当前章/最近内容/含摘要', pos:'bottom'},
      {el:'#quotaTokens', tip:'📈 悬停查看各角色（构思/写作/审稿）的 Token 消耗明细', pos:'bottom'},
      {el:'button[title~="专注模式"]', tip:'🎯 专注模式：折叠侧边栏，全屏沉浸创作', pos:'bottom'}
    ];
    var idx = 0;
    var self = this;
    function showStep() {
      if (idx >= steps.length) { Store.set('onboarded', true); return; }
      var s = steps[idx];
      var el = document.querySelector(s.el);
      if (!el) { idx++; showStep(); return; }
      var rect = el.getBoundingClientRect();
      var tip = document.createElement('div');
      tip.className = 'onboard-tip';
      tip.textContent = s.tip;
      tip.style.left = rect.left + 'px';
      tip.style.top = (s.pos === 'top' ? rect.top - 40 : rect.bottom + 8) + 'px';
      tip.onclick = function () { tip.remove(); idx++; showStep(); };
      document.body.appendChild(tip);
    }
    setTimeout(showStep, 800);
  },
  showKeyboardRef: function () {
    UI.modal({
      title: '⌨️ 快捷键参考',
      wide: '480px',
      body: '<table style="width:100%;font-size:11px;line-height:2"><tr><td style="width:40%"><kbd>Ctrl+Enter</kbd></td><td>从当前光标位置续写</td></tr>' +
        '<tr><td><kbd>Ctrl+Shift+Enter</kbd></td><td>重新生成全文</td></tr>' +
        '<tr><td><kbd>Ctrl+Shift+P</kbd></td><td>专注模式（全屏创作）</td></tr>' +
        '<tr><td><kbd>Ctrl+S</kbd></td><td>保存当前章节</td></tr>' +
        '<tr><td><kbd>Ctrl+Shift+S</kbd></td><td>另存为版本快照</td></tr>' +
        '<tr><td><kbd>Ctrl+Z</kbd> / <kbd>Ctrl+Y</kbd></td><td>撤销 / 重做</td></tr>' +
        '<tr><td><kbd>Ctrl+B</kbd> / <kbd>Ctrl+I</kbd> / <kbd>Ctrl+U</kbd></td><td>粗体 / 斜体 / 下划线</td></tr>' +
        '<tr><td><kbd>Ctrl+Shift+F</kbd></td><td>选中文字一键润色</td></tr>' +
        '<tr><td><kbd>?</kbd></td><td>显示此快捷键参考</td></tr>' +
        '<tr><td><kbd>Esc</kbd></td><td>关闭弹窗 / 取消选中</td></tr></table>',
      actions: [{ id: 'close', label: '关闭' }]
    });
  }
};
document.addEventListener('click', function () { UI.closeCtx(); });
document.addEventListener('scroll', function () { UI.closeCtx(); }, true);
