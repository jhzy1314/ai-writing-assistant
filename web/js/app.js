/* ============ app.js：应用初始化与全局事件绑定 ============ */
document.addEventListener('DOMContentLoaded', function () {
  // 0. 认证检查
  Auth.check().then(function (result) {
    if (result.required && !result.authenticated) { Auth.showLogin(); return; }
    initApp();
  });
});

function initApp() {
  // 无障碍增强：纯图标按钮用 title 兜底 aria-label；div/span 点击支持键盘操作
  enhanceA11y();
  Store.loadPrefs();
  UI.initTheme();
  UI.initFontSize();
  Sidebar.restoreResourcesState();
  Sidebar.restoreNavMoreState();
  // 恢复侧栏/右栏折叠状态（body 类同步，保证折叠按钮显示正确）
  try {
    var sb = document.getElementById('sidebar');
    if (sb && Store.get('sidebarCollapsed', false)) {
      sb.classList.add('collapsed');
      document.body.classList.add('sidebar-hidden');
    }
    var rp = document.getElementById('rightPanel');
    if (rp && Store.get('rightCollapsed', false)) {
      rp.classList.add('collapsed');
      document.body.classList.add('right-hidden');
    }
  } catch (e) {}
  decorateShortcuts();
  // 1.5 初始化路由
  Router.init();
  // 2. 初始化编辑器
  Editor.init();
  // 3. 初始化创作模式选择器
  Composer.init();
  // 4. 渲染流水线初始态
  PipelineUI.render();
  // 5. 加载项目列表，加载完成后自动打开上次项目
  console.log('[init] Starting...');
  ProjectUI.loadAll().then(function () {
    // Auto-open last project
    var lastId = Store.get('lastProjectId', '');
    if (lastId && Store.state.projects.some(function (p) { return p.id === lastId; })) {
      ProjectUI.select(lastId);
    }
  }).catch(function (e) { /* silent */ });
  // 兜底轮询
  var pollCount = 0;
  var pollInterval = setInterval(function () {
    pollCount++;
    if (Store.state.projects.length > 0) { clearInterval(pollInterval); return; }
    if (pollCount >= 10) { clearInterval(pollInterval); return; }
    if (pollCount % 2 === 0) ProjectUI.loadAll().catch(function () {});
  }, 300);
  // 5.5 初始化外观设置
  Appearance.init();
  // 6. 加载模板（后端 + 内置）
  loadTemplates();
  // 7. 刷新额度
  Usage.refresh();
  // 8. 启动 30 秒自动草稿
  ManualSave.startAutosave();
  // 9. 主题切换（多主题系统）
  if (typeof Themes !== 'undefined' && Themes.init) {
    Themes.init();
    document.getElementById('themeToggle').onclick = function (e) { Themes.toggleMenu(e); };
  } else {
    document.getElementById('themeToggle').onclick = function () { UI.toggleTheme(); };
  }
  // 10. 阻止鼠标侧键导致页面跳出（必须在异步请求前设置）
  if (history && history.replaceState) history.replaceState('novel', '', '/');
  window.__appInternalNav = false;
  // 鼠标侧键（XButton1=3 后退 / XButton2=4 前进）在 mousedown 阶段直接拦截，不触发 popstate
  window.addEventListener('mousedown', function (e) {
    if (e.button === 3 || e.button === 4) {
      e.preventDefault();
      var txt = Editor.getText();
      if (txt && txt.trim()) {
        UI.toast('已拦截鼠标侧键的前进/后退（避免丢失未保存内容）', 'warn');
      }
    }
  });
  window.addEventListener('popstate', function (e) {
    // 应用自身的 hash 导航（侧栏/页面切换）不是浏览器前进/后退：放行并保留 hash
    if (window.__appInternalNav) { window.__appInternalNav = false; return; }
    if (history && history.replaceState) history.replaceState('novel', '', '/');
    var txt = Editor.getText();
    if (txt && txt.trim()) {
      UI.toast('页面跳转已被拦截（浏览器前进/后退），请勿离开当前页面', 'warn');
    }
  });
  // 11. 全局快捷键
  document.addEventListener('keydown', function (e) {
    // Escape 终止生成
    if (e.key === 'Escape') {
      if (SSE.active) { SSE.stop(); }
      else {
        document.querySelectorAll('.modal-overlay').forEach(function (m) { m.remove(); });
        UI.closeCtx();
        Editor.hideSelToolbar();
      }
    }
    // ? 显示快捷键参考
    if (e.key === '?' && !e.ctrlKey && !e.metaKey && !e.altKey) {
      var activeEl = document.activeElement;
      if (!activeEl || (activeEl.tagName !== 'INPUT' && activeEl.tagName !== 'TEXTAREA')) {
        e.preventDefault();
        UI.showKeyboardRef();
      }
    }
    // Ctrl+S 保存版本
    if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) {
      e.preventDefault();
      if (Store.state.currentProject) ManualSave.saveDraft();
    }
    // Ctrl+Z 撤销 / Ctrl+Y 重做（Tiptap 内置）
    if ((e.ctrlKey || e.metaKey) && (e.key === 'z' || e.key === 'Z') && !e.shiftKey && Editor.mode === 'rich' && Editor.tiptap) {
      Editor.tiptap.commands.undo(); e.preventDefault();
    }
    if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || e.key === 'Y') && Editor.mode === 'rich' && Editor.tiptap) {
      Editor.tiptap.commands.redo(); e.preventDefault();
    }
    // Ctrl+F 全文搜索章节
    if ((e.ctrlKey || e.metaKey) && (e.key === 'f' || e.key === 'F')) {
      e.preventDefault();
      if (Store.state.currentProject) ChapterUI.searchAllChapters();
    }
  });
  // 12. 未保存内容离开确认
  window.addEventListener('beforeunload', function (e) {
    var txt = Editor.getText();
    var p = Store.state.currentProject;
    if (txt && txt.trim() && p) {
      var saved = Store.getDraft(p.id);
      if (!saved || saved.text !== txt) {
        e.preventDefault();
        e.returnValue = '您有未保存的内容，确定离开？';
        return e.returnValue;
      }
    }
  });
  // 13. 首次启动新手引导（仅显示一次）
  UI.showOnboarding();
  // 14. 离线状态监听
  function updateOfflineBar() {
    var bar = document.getElementById('offlineBar');
    if (bar) bar.style.display = navigator.onLine ? 'none' : '';
  }
  window.addEventListener('online', updateOfflineBar);
  window.addEventListener('offline', updateOfflineBar);
  updateOfflineBar();
  // 15. 设计模式（按需在 DesignMode 定义后调用）
  if (typeof DesignMode !== 'undefined') DesignMode.init();
}


async function loadTemplates() {
  try {
    Store.state.templates = await API.listTemplates();
  } catch (e) { /* 静默：内置模板仍可用 */ }
  TemplateUI.init();
}

// ============ 无障碍增强 ============
// 1) 纯图标按钮（文字<=2字符）若无 aria-label，用 title 兜底
// 2) 带 onclick 的 div/span 模拟按钮：补 role/tabindex + Enter/Space 键盘触发
function enhanceA11y() {
  try {
    // 图标按钮兜底：仅处理无可见文字（纯图标/空）且无 aria-label 的按钮
    document.querySelectorAll('button').forEach(function (b) {
      var text = (b.innerText || '').trim();
      // 有可见文字（含 emoji/短中文）则不算缺失；纯图标（无文字）才需要 title 兜底
      var visibleText = text.replace(/[\u{1F300}-\u{1FAFF}\u2600-\u27BF\uFE0F]/gu, '').trim();
      if (visibleText.length === 0 && !b.getAttribute('aria-label')) {
        var t = b.getAttribute('title');
        if (t) b.setAttribute('aria-label', t);
      }
    });
    // div/span 模拟按钮键盘化
    document.querySelectorAll('[onclick]').forEach(function (el) {
      if (el.tagName !== 'DIV' && el.tagName !== 'SPAN') return;
      if (el.getAttribute('role') === 'button') return;
      if (el.closest('[contenteditable]')) return; // 编辑区内不处理
      el.setAttribute('role', 'button');
      el.setAttribute('tabindex', '0');
      if (!el.getAttribute('aria-label')) {
        var t = el.getAttribute('title') || (el.innerText || '').trim().slice(0, 20);
        if (t) el.setAttribute('aria-label', t);
      }
      el.addEventListener('keydown', function (e) {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          el.click();
        }
      });
    });
  } catch (e) { /* 无障碍增强失败不影响主流程 */ }
}
// 动态渲染的内容也会产生新按钮，用 MutationObserver 兜底
var _a11yObserved = false;
function watchA11y() {
  if (_a11yObserved) return;
  _a11yObserved = true;
  var tgt = document.getElementById('modalRoot') || document.body;
  var mo = new MutationObserver(function () { enhanceA11y(); });
  mo.observe(document.body, { childList: true, subtree: true });
}
document.addEventListener('DOMContentLoaded', function () { setTimeout(watchA11y, 500); });

/* ============ 快捷键 chip：给带 data-shortcut 的按钮注入 <kbd> 徽标 ============ */
function decorateShortcuts() {
  try {
    document.querySelectorAll('[data-shortcut]').forEach(function (btn) {
      if (btn.querySelector('.kbd-chip')) return; // 已注入过
      var sc = btn.getAttribute('data-shortcut');
      if (!sc) return;
      var chip = document.createElement('kbd');
      chip.className = 'kbd-chip';
      chip.textContent = sc;
      // 生成按钮等主要按钮：chip 放按钮内右侧
      btn.appendChild(chip);
    });
  } catch (e) { /* 注入失败不影响主流程 */ }
}
// 动态渲染的按钮也用 MutationObserver 兜底补 chip
var _scObserved = false;
function watchShortcuts() {
  if (_scObserved) return;
  _scObserved = true;
  var mo = new MutationObserver(function () { decorateShortcuts(); });
  mo.observe(document.body, { childList: true, subtree: true });
}
document.addEventListener('DOMContentLoaded', function () { setTimeout(watchShortcuts, 800); });
