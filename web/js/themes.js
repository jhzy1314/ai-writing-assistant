/* ============ themes.js：多主题系统 ============
 * 主题注册表 + 应用/持久化 + 顶部栏下拉 + 外观面板选择器
 * 兼容旧版二元主题：经典主题 id 保持 dark / light，旧 localStorage 值无缝兼容
 * ============================================== */
var Themes = {
  list: [
    { id: 'dark',      name: '经典深色', icon: '🌙', mode: 'dark',  isClassic: true, swatch: 'linear-gradient(135deg,#1d2230,#0e1018)' },
    { id: 'light',     name: '经典浅色', icon: '☀️', mode: 'light', isClassic: true, swatch: 'linear-gradient(135deg,#f8f6f3,#ede8e0)' },
    { id: 'ink-study', name: '墨韵书斋', icon: '🖌️', mode: 'dark',  swatch: 'linear-gradient(135deg,#5c3a1e,#2d1409)' },
    { id: 'gold',      name: '鎏金',     icon: '👑', mode: 'dark',  swatch: 'linear-gradient(135deg,#2c2c2c,#1a1a1a)' },
    { id: 'focus',     name: '沉浸专注', icon: '🌌', mode: 'dark',  swatch: 'linear-gradient(135deg,#23262f,#10131a)' },
    { id: 'paper-ink', name: '纸墨书香', icon: '📜', mode: 'light', swatch: 'linear-gradient(135deg,#f7f2e7,#efe5d2)' }
  ],
  current: 'dark',
  defaultId: 'dark',

  get: function (id) {
    for (var i = 0; i < this.list.length; i++) {
      if (this.list[i].id === id) return this.list[i];
    }
    return null;
  },
  mode: function () {
    var t = this.get(this.current);
    return t ? t.mode : 'dark';
  },
  name: function () {
    var t = this.get(this.current);
    return t ? t.name : '经典深色';
  },
  loadSaved: function () {
    var saved = null;
    try { saved = Store.get('theme', null); } catch (e) { saved = null; }
    if (saved && this.get(saved)) { this.current = saved; }
    else { this.current = this.defaultId; }
    return this.current;
  },
  apply: function (id, silent) {
    var t = this.get(id) || this.get(this.defaultId);
    if (!t) return;
    this.current = t.id;
    document.documentElement.setAttribute('data-theme', t.id);
    document.body.classList.toggle('theme-light', t.mode === 'light');
    document.body.classList.toggle('theme-dark', t.mode === 'dark');
    this.syncLabel();
    // 背景系统联动：按明暗模式复用对应背景变量（appearance.js 已在 applyCurrent 设置 CSS 变量）
    if (typeof Appearance !== 'undefined' && Appearance.applyTheme) {
      try { Appearance.applyTheme(); } catch (e) {}
    }
    if (!silent && typeof UI !== 'undefined' && UI.toast) {
      UI.toast('已切换主题：' + t.name, 'success');
    }
    try { Store.set('theme', t.id); } catch (e) {}
    var grid = document.getElementById('themeGrid');
    if (grid && grid.children.length) this.renderPanelGrid(grid);
  },
  syncLabel: function () {
    var t = this.get(this.current);
    var btn = document.getElementById('themeToggle');
    if (btn && t) btn.textContent = t.icon + ' ' + t.name;
  },
  toggle: function () {
    // 快捷切换：经典深色 <-> 经典浅色；其他主题 -> 经典深色
    var next = this.current === 'dark' ? 'light' : 'dark';
    this.apply(next);
  },
  toggleMenu: function (ev) {
    ev = ev || window.event;
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var old = document.getElementById('themeMenu');
    if (old) { old.remove(); return; }
    var btn = document.getElementById('themeToggle');
    if (!btn) return;
    var menu = document.createElement('div');
    menu.className = 'theme-menu';
    menu.id = 'themeMenu';
    var self = this;
    var html = '<div class="theme-menu-title">🎨 选择主题</div>';
    this.list.forEach(function (t) {
      var active = t.id === self.current ? ' active' : '';
      html += '<div class="theme-menu-item' + active + '" data-theme-id="' + t.id + '">' +
        '<span class="tm-icon">' + t.icon + '</span>' +
        '<span class="tm-name">' + t.name + '</span>' +
        '<span class="tm-mode">' + (t.mode === 'dark' ? '深色' : '浅色') + '</span>' +
        (active ? '<span class="tm-check">✓</span>' : '') +
        '</div>';
    });
    menu.innerHTML = html;
    menu.querySelectorAll('.theme-menu-item').forEach(function (el) {
      el.onclick = function () {
        self.apply(el.getAttribute('data-theme-id'));
        var m = document.getElementById('themeMenu');
        if (m) m.remove();
      };
    });
    var r = btn.getBoundingClientRect();
    menu.style.top = (r.bottom + 6) + 'px';
    menu.style.right = Math.max(6, window.innerWidth - r.right) + 'px';
    document.body.appendChild(menu);
    setTimeout(function () {
      function close(e) {
        if (!menu.contains(e.target)) {
          menu.remove();
          document.removeEventListener('click', close, true);
        }
      }
      document.addEventListener('click', close, true);
    }, 0);
  },
  renderPanel: function () {
    var grid = document.getElementById('themeGrid');
    if (grid) this.renderPanelGrid(grid);
  },
  renderPanelGrid: function (grid) {
    var self = this;
    var html = '';
    this.list.forEach(function (t) {
      var active = t.id === self.current ? ' active' : '';
      html += '<div class="theme-pick' + active + '" data-theme-id="' + t.id + '" title="' + t.name + '">' +
        '<div class="theme-pick-swatch" style="background:' + t.swatch + '">' + t.icon + '</div>' +
        '<div class="theme-pick-name">' + t.name + '</div>' +
        '<div class="theme-pick-check">✓</div>' +
        '</div>';
    });
    grid.innerHTML = html;
    grid.querySelectorAll('.theme-pick').forEach(function (el) {
      el.onclick = function () { self.apply(el.getAttribute('data-theme-id')); };
    });
  },
  init: function () {
    this.loadSaved();
    document.documentElement.setAttribute('data-theme', this.current);
    document.body.classList.toggle('theme-light', this.mode() === 'light');
    document.body.classList.toggle('theme-dark', this.mode() === 'dark');
    this.syncLabel();
    this.renderPanel();
  }
};
