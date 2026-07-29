/* ============ app.js：应用初始化与全局事件绑定 ============ */
document.addEventListener('DOMContentLoaded', function () {
  // 1. 加载本地偏好（主题 / 模式 / 编辑器模式等）
  Store.loadPrefs();
  // 1.1 恢复侧栏折叠状态
  if (Store.get('sidebarCollapsed', false)) { document.getElementById('sidebar').classList.add('collapsed'); }
  if (Store.get('rightCollapsed', false)) { document.getElementById('rightPanel').classList.add('collapsed'); }
  // 1.2 恢复字体大小
  var fs = Store.get('fontSize', 0);
  if (fs > 0) { setTimeout(function () { Editor._fs = fs; Editor.adjustFontSize(0); }, 500); }
  // 1.3 首次使用引导
  setTimeout(function () { UI.showOnboarding(); }, 1200);
  // 2. 初始化编辑器
  Editor.init();
  // 3. 初始化创作模式选择器
  Composer.init();
  // 4. 渲染流水线初始态
  PipelineUI.render();
  // 5. 加载项目列表，加载完成后自动打开上次项目
  console.log('[init] Starting project load...');
  ProjectUI.loadAll().then(function () {
    console.log('[init] Projects loaded: ' + Store.state.projects.length);
    // 自动打开上次选中的项目
    var lastId = Store.get('lastProjectId', '');
    if (lastId && Store.state.projects.some(function (p) { return p.id === lastId; })) {
      console.log('[init] Auto-opening last project: ' + lastId);
      ProjectUI.select(lastId);
    }
  }).catch(function (e) {
    console.error('[init] loadAll failed: ' + (e && e.message));
  });
  // 兜底轮询
  var pollCount = 0;
  var pollInterval = setInterval(function () {
    pollCount++;
    if (Store.state.projects.length > 0) { console.log('[init] Poll success at ' + pollCount); clearInterval(pollInterval); return; }
    if (pollCount >= 10) { console.log('[init] Poll exhausted'); clearInterval(pollInterval); return; }
    if (pollCount % 2 === 0) ProjectUI.loadAll().catch(function (e) { console.error('[init] Retry ' + pollCount + ' failed: ' + (e && e.message)); });
  }, 300);
  // 6. 加载模板（后端 + 内置）
  loadTemplates();
  // 7. 刷新额度
  Usage.refresh();
  // 8. 启动 30 秒自动草稿
  ManualSave.startAutosave();
  // 9. 主题切换
  document.getElementById('themeToggle').onclick = function () { UI.toggleTheme(); };
  // 10. 阻止鼠标侧键导致页面跳出（必须在异步请求前设置）
  if (history && history.replaceState) history.replaceState('novel', '', '/');
  window.addEventListener('popstate', function (e) {
    if (history && history.replaceState) history.replaceState('novel', '', '/');
    var txt = Editor.getText();
    if (txt && txt.trim()) {
      UI.toast('页面跳转已被拦截（侧键导致），请勿使用浏览器前进/后退', 'warn');
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
});

async function loadTemplates() {
  try {
    Store.state.templates = await API.listTemplates();
  } catch (e) { /* 静默：内置模板仍可用 */ }
  TemplateUI.init();
}
