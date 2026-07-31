/* ============ router.js：前端路由管理，页面切换 + 草稿保持 + 主题联动 ============ */
var Router = {
  pages: ['editor', 'characters', 'worldbuilding', 'toolbox', 'dashboard', 'outline'],
  current: 'editor',
  cache: {},
  init: function () {
    var self = this;
    // 读取 hash 或默认到编辑器
    var hash = window.location.hash.replace('#', '') || Store.get('lastPage', 'editor');
    if (this.pages.indexOf(hash) === -1) hash = 'editor';
    this.current = hash;
    // 注册 hashchange
    window.addEventListener('hashchange', function () {
      var h = window.location.hash.replace('#', '');
      if (self.pages.indexOf(h) === -1) h = 'editor';
      self.navigate(h, false);
    });
    // 初始渲染
    this.show(hash);
    this.updateNav(hash);
  },
  navigate: function (page, pushState) {
    if (pushState === undefined) pushState = true;
    if (page === this.current) return;
    // 保存当前页草稿状态
    this.saveDraft(this.current);
    this.show(page);
    if (pushState) {
      // 标记为应用自身导航，popstate 拦截器应放行（不弹“跳转被拦截”提示、不抹掉 hash）
      window.__appInternalNav = true;
      window.location.hash = page;
    }
    Store.set('lastPage', page);
  },
  show: function (page) {
    this.current = page;
    // 隐藏所有页面
    document.querySelectorAll('.router-page').forEach(function (el) { el.classList.remove('show'); });
    // 显示目标页面
    var target = document.getElementById('routerPage-' + page);
    if (target) target.classList.add('show');
    // 更新导航高亮
    this.updateNav(page);
    // 恢复草稿
    this.restoreDraft(page);
    // 页面初始化钩子
    this.initPage(page);
  },
  updateNav: function (page) {
    document.querySelectorAll('.nav-link').forEach(function (el) {
      el.classList.toggle('active', el.getAttribute('data-page') === page);
    });
  },
  saveDraft: function (page) {
    if (!this.cache[page]) this.cache[page] = {};
    if (page === 'editor') {
      this.cache[page].text = Editor.getText();
    }
  },
  restoreDraft: function (page) {
    // Editor draft is handled by ManualSave autosave
  },
  initPage: function (page) {
    switch (page) {
      case 'characters': if (typeof CharacterPage !== 'undefined') CharacterPage.init(); break;
      case 'worldbuilding': if (typeof WorldPage !== 'undefined') WorldPage.init(); break;
      case 'toolbox': if (typeof ToolboxPage !== 'undefined') ToolboxPage.init(); break;
      case 'dashboard': if (typeof DashboardPage !== 'undefined') DashboardPage.init(); break;
      case 'outline': if (typeof OutlinePage !== 'undefined') OutlinePage.init(); break;
    }
  },
  getParam: function (name) {
    var params = new URLSearchParams(window.location.hash.split('?')[1] || '');
    return params.get(name);
  }
};
