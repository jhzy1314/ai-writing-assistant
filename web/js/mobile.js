/* ============ mobile.js：移动端专属 UI（响应式切换/抽屉/面板/章节选择） ============ */
var MobileUI = {
  init: function () {
    // 检测设备：屏幕宽度 <= 768px 判定为移动端
    var isMobile = window.innerWidth <= 768;
    var stored = Store.get('forceDesktop', null);
    if (stored === '1') { document.body.classList.add('desktop'); return; }
    if (isMobile) { document.body.classList.remove('desktop'); } else { document.body.classList.add('desktop'); }
    // 监听窗口变化（平板旋转等）
    var self = this;
    window.addEventListener('resize', function () {
      if (Store.get('forceDesktop', null) === '1') return;
      var mob = window.innerWidth <= 768;
      document.body.classList.toggle('desktop', !mob);
      if (mob) self.syncDrawerList();
    });
    // 软键盘适配：visualViewport 变化时自动滚动聚焦元素到可见区
    if (window.visualViewport) {
      var initialVH = window.visualViewport.height;
      window.visualViewport.addEventListener('resize', function () {
        var vh = window.visualViewport.height;
        // 视口高度显著减小时判定键盘打开
        if (vh < initialVH * 0.75) {
          document.body.classList.add('kb-open');
          var ae = document.activeElement;
          if (ae && (ae.tagName === 'INPUT' || ae.tagName === 'TEXTAREA')) {
            setTimeout(function () { ae.scrollIntoView({ behavior: 'smooth', block: 'center' }); }, 150);
          }
        } else {
          document.body.classList.remove('kb-open');
        }
        initialVH = vh;
      });
    }
  },

  toggleView: function () {
    // 手动切换电脑版/手机版
    var forced = Store.get('forceDesktop', null);
    if (forced === '1') {
      Store.remove('forceDesktop');
      document.body.classList.remove('desktop');
      this.closeDrawer();
      this.closeRightSheet();
      this.closeModelPanel();
    } else {
      Store.set('forceDesktop', '1');
      document.body.classList.add('desktop');
    }
    UI.toast(forced === '1' ? '已切换到手机版' : '已切换到电脑版', 'info');
  },

  filter: function (q) {
    this.renderProjectList(q);
    this.renderResources(q);
  },

  // ===== 抽屉（侧边栏） =====
  openDrawer: function () {
    this.syncDrawerList();
    document.getElementById('mobileDrawerOverlay').classList.add('open');
    document.getElementById('mobileDrawer').classList.add('open');
  },
  closeDrawer: function () {
    document.getElementById('mobileDrawerOverlay').classList.remove('open');
    document.getElementById('mobileDrawer').classList.remove('open');
  },

  syncDrawerList: function () {
    // 同步 PC 侧栏项目列表到移动端抽屉
    var nl = document.getElementById('novelList');
    var mnl = document.getElementById('mNovelList');
    if (nl && mnl) mnl.innerHTML = nl.innerHTML;
    // 同步章节树
    var ct = document.getElementById('chapterTree');
    var mct = document.getElementById('mChapterTree');
    if (ct && mct) mct.innerHTML = ct.innerHTML;
    // 同步资源列表
    var rl = document.getElementById('resList');
    var mrl = document.getElementById('mResList');
    if (rl && mrl) mrl.innerHTML = rl.innerHTML;
    // 同步资源区标题
    var rpn = document.getElementById('resProjName');
    var mrpn = document.getElementById('mResProjName');
    if (rpn && mrpn) mrpn.textContent = rpn.textContent;
    // 同步资源区可见性
    var rs = document.getElementById('resourceSection');
    var mrs = document.getElementById('mResourceSection');
    if (rs && mrs) {
      if (rs.classList.contains('show')) mrs.classList.add('show'); else mrs.classList.remove('show');
    }
    // 同步顶部标题
    var dt = document.getElementById('docTitle');
    var mt = document.getElementById('mtTitle');
    if (dt && mt) mt.textContent = dt.textContent;
  },

  // ===== 底部右面板弹出 =====
  openRightSheet: function () {
    this.renderRightTabs();
    this.switchRightPage('pipeline');
    document.getElementById('mobileRightOverlay').classList.add('open');
    document.getElementById('mobileRightSheet').classList.add('open');
  },
  closeRightSheet: function () {
    document.getElementById('mobileRightOverlay').classList.remove('open');
    document.getElementById('mobileRightSheet').classList.remove('open');
  },

  renderRightTabs: function () {
    var tabs = [
      { id: 'pipeline', label: '📡' },
      { id: 'templates', label: '📋' },
      { id: 'context', label: '👤' },
      { id: 'models', label: '🤖' },
      { id: 'tools', label: '🔧' }
    ];
    var html = '';
    tabs.forEach(function (t, i) {
      html += '<span class="mrs-tab' + (i === 0 ? ' active' : '') + '" data-mpage="' + t.id + '" onclick="MobileUI.switchRightPage(\'' + t.id + '\')">' + t.label + '</span>';
    });
    document.getElementById('mrsTabs').innerHTML = html;
  },

  switchRightPage: function (page) {
    document.querySelectorAll('#mrsTabs .mrs-tab').forEach(function (t) {
      t.classList.toggle('active', t.dataset.mpage === page);
    });
    var body = document.getElementById('mrsBody');
    // 复用 PC 已有页面的内容（innerHTML 会保留 onclick 等内联事件）
    var pcPage = document.getElementById('page-' + page);
    if (pcPage) {
      body.innerHTML = pcPage.innerHTML;
    } else {
      body.innerHTML = '<div class="res-check-empty">加载中…</div>';
    }
  },

  // ===== 章节选择器（支持排序） =====
  showChapterPicker: function () {
    var chs = Store.state.chapters || [];
    var cur = Store.state.currentChapter;
    if (!chs.length) { UI.toast('暂无章节', 'warn'); return; }
    var html = '<div class="mobile-chapter-list">';
    chs.forEach(function (ch, idx) {
      var active = cur && cur.id === ch.id;
      var isFirst = idx === 0;
      var isLast = idx === chs.length - 1;
      html += '<div class="mcl-item' + (active ? ' active' : '') + '" data-cid="' + ch.id + '">' +
        '<span class="mcl-icon">📄</span>' +
        '<span class="mcl-name">' + esc(ch.title || '无标题') + '</span>' +
        '<span class="mcl-wc">' + (ch.word_count || 0) + '字</span>' +
        '<span class="mcl-moves">' +
          (!isFirst ? '<span class="mcl-move mcl-up" title="上移">▲</span>' : '<span class="mcl-move-placeholder"></span>') +
          (!isLast ? '<span class="mcl-move mcl-down" title="下移">▼</span>' : '<span class="mcl-move-placeholder"></span>') +
        '</span>' +
        '</div>';
    });
    html += '</div>';
    var modal = UI.modal({
      title: '选择章节',
      body: html,
      actions: [{ id: 'cancel', label: '关闭' }]
    });
    var self = this;
    setTimeout(function () {
      document.querySelectorAll('.mcl-item').forEach(function (el) {
        el.onclick = function (e) {
          if (e.target.classList.contains('mcl-move')) return;
          var cid = el.dataset.cid;
          var ch = Store.state.chapters.find(function (c) { return c.id === cid; });
          if (ch) {
            ChapterUI.selectChapter(ch);
            MobileUI.updateChapterBar();
            modal.overlay.remove();
          }
        };
      });
      document.querySelectorAll('.mcl-move').forEach(function (el) {
        el.onclick = function (e) {
          e.stopPropagation();
          var cid = el.dataset.cid;
          var dir = el.classList.contains('mcl-up') ? -1 : 1;
          self.moveChapterInPicker(cid, dir, modal);
        };
      });
    }, 60);
  },

  moveChapterInPicker: async function (cid, dir, modal) {
    var chs = Store.state.chapters;
    var c = chs.find(function (x) { return x.id === cid; });
    if (!c) return;
    var sameVol = chs.filter(function (x) { return x.volume_id === c.volume_id; });
    sameVol.sort(function (a, b) { return a.sort_order - b.sort_order; });
    var idx = sameVol.findIndex(function (x) { return x.id === cid; });
    if (idx < 0) return;
    var targetIdx = idx + dir;
    if (targetIdx < 0 || targetIdx >= sameVol.length) return;
    var tmp = sameVol[idx].sort_order;
    sameVol[idx].sort_order = sameVol[targetIdx].sort_order;
    sameVol[targetIdx].sort_order = tmp;
    var items = sameVol.map(function (x) { return { id: x.id, sort_order: x.sort_order }; });
    try {
      await API.reorderChapters(items);
      await ChapterUI.loadAll();
      ChapterUI.renderTree();
      MobileUI.syncDrawerList();
      UI.toast(dir < 0 ? '已上移' : '已下移', 'success');
      modal.overlay.remove();
    } catch (e) { UI.toast('移动失败', 'error'); }
  },

  updateChapterBar: function () {
    var cur = Store.state.currentChapter;
    var el = document.getElementById('mcbName');
    if (el) el.textContent = cur ? (cur.title || '无标题') : '未选择章节';
  },

  // ===== 模型管理全屏面板 =====
  openModelPanel: function () {
    document.getElementById('mobileModelPanel').classList.add('open');
    var body = document.getElementById('mmpBody');
    body.innerHTML = '<div class="res-check-empty">加载中…</div>';
    var self = this;
    if (typeof ModelSettings !== 'undefined') {
      ModelSettings.loadAll().then(function () {
        var pcPage = document.getElementById('page-models');
        if (pcPage) { body.innerHTML = pcPage.innerHTML; }
      }).catch(function () {
        body.innerHTML = '<div class="res-check-empty">加载失败</div>';
      });
    }
  },
  closeModelPanel: function () {
    document.getElementById('mobileModelPanel').classList.remove('open');
  }
};

// 【共用】monkey-patch ChapterUI.selectChapter 以同步移动端章节栏
(function () {
  var origSelect = ChapterUI.selectChapter;
  if (origSelect) {
    ChapterUI.selectChapter = function (ch) {
      var p = origSelect.call(ChapterUI, ch);
      if (p && p.then) {
        return p.then(function () {
          MobileUI.updateChapterBar();
          MobileUI.syncDrawerList();
        });
      }
      MobileUI.updateChapterBar();
      MobileUI.syncDrawerList();
      return p;
    };
  }
})();

// 【共用】monkey-patch ProjectUI.select 以同步移动端抽屉
(function () {
  var origProjectSelect = ProjectUI.select;
  if (origProjectSelect) {
    ProjectUI.select = function (id) {
      var p = origProjectSelect.call(ProjectUI, id);
      if (p && p.then) {
        return p.then(function () {
          MobileUI.syncDrawerList();
          MobileUI.updateChapterBar();
        });
      }
      MobileUI.syncDrawerList();
      MobileUI.updateChapterBar();
      return p;
    };
  }
})();
