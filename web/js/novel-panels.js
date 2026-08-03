/* ============ novel-panels.js：伏笔看板 / 素材库 / 构思Agent / 关系图谱 / 场景节拍 ============ */

/* ---------- 1. 伏笔看板 ---------- */
var ForeshadowPage = {
  STATUS: { pending: { label: '待回收', cls: 'fs-pending' }, recollected: { label: '已回收', cls: 'fs-recollected' }, dropped: { label: '已放弃', cls: 'fs-dropped' } },
  init: function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty(); return; }
    this.load();
  },
  showEmpty: function (msg) {
    var el = document.getElementById('foreshadowList');
    el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">🔮</div><div>' + (msg || '请先在左侧选中一个项目') + '</div></div>';
  },
  load: async function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty(); return; }
    var el = document.getElementById('foreshadowList');
    el.innerHTML = '<div class="loading">加载中</div>';
    try {
      var items = await API.listForeshadows(p.id);
      var chs = await API.listChapters(p.id);
      this._items = items; // edit()/mark() 依赖；缺失会导致编辑按钮静默失效
      this.chapterMap = {};
      chs.forEach(function (c, i) { this[c.id] = { no: i + 1, title: c.title }; }, this.chapterMap);
      this.render(items);
    } catch (e) {
      el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>' + esc(e.message) + '</div></div>';
    }
  },
  chLabel: function (cid) {
    if (!cid || !this.chapterMap[cid]) return '未指定';
    return '第' + this.chapterMap[cid].no + '章 ' + this.chapterMap[cid].title;
  },
  render: function (items) {
    var el = document.getElementById('foreshadowList');
    var stats = document.getElementById('foreshadowStats');
    var pending = items.filter(function (f) { return f.status === 'pending'; }).length;
    stats.textContent = items.length + ' 条伏笔 · ' + pending + ' 条待回收';
    if (!items.length) {
      el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">🔮</div><div>暂无伏笔<br><small style="color:var(--muted)">用「🤖 AI 扫描」通读全书识别伏笔，或手动添加</small></div></div>';
      return;
    }
    var html = '';
    items.forEach(function (f) {
      var st = ForeshadowPage.STATUS[f.status] || ForeshadowPage.STATUS.pending;
      html += '<div class="fs-card ' + st.cls + '">' +
        '<div class="fs-head"><strong>' + esc(f.title) + '</strong>' +
        '<span class="fs-badge">' + st.label + '</span></div>' +
        (f.description ? '<div class="fs-desc">' + esc(f.description) + '</div>' : '') +
        '<div class="fs-meta">🔒 埋设于 ' + esc(ForeshadowPage.chLabel(f.setup_chapter_id)) +
        (f.payoff_chapter_id ? ' · ✅ 回收于 ' + esc(ForeshadowPage.chLabel(f.payoff_chapter_id)) : '') + '</div>' +
        '<div class="fs-acts">' +
        (f.status === 'pending' ? '<button class="btn btn-ghost btn-sm" onclick="ForeshadowPage.mark(\'' + f.id + '\',\'recollected\')">✅ 标记回收</button>' : '') +
        '<button class="btn btn-ghost btn-sm" onclick="ForeshadowPage.edit(\'' + f.id + '\')">✏️ 编辑</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="ForeshadowPage.mark(\'' + f.id + '\',\'dropped\')">💤 放弃</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="ForeshadowPage.del(\'' + f.id + '\')">🗑 删除</button>' +
        '</div></div>';
    });
    el.innerHTML = html;
  },
  create: function () {
    var self = this;
    var chs = Object.keys(this.chapterMap || {});
    var opts = '<option value="">未指定</option>' + chs.map(function (cid) {
      return '<option value="' + cid + '">' + esc(self.chapterMap[cid].title || '第' + self.chapterMap[cid].no + '章') + '</option>';
    }).join('');
    UI.modal({
      title: '＋ 添加伏笔',
      body: '<div class="form-group"><label>伏笔名称 *</label><input id="fsTitle" class="form-input" placeholder="如：林云书包里的旧照片"></div>' +
        '<div class="form-group"><label>伏笔描述</label><textarea id="fsDesc" rows="3" placeholder="伏笔内容、预期回收方式"></textarea></div>' +
        '<div class="form-group"><label>埋设章节</label><select id="fsSetup">' + opts + '</select></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var title = document.getElementById('fsTitle').value.trim();
          if (!title) { UI.toast('请输入伏笔名称', 'warn'); return; }
          ov.remove();
          API.createForeshadow({
            project_id: Store.state.currentProject.id,
            title: title,
            description: document.getElementById('fsDesc').value.trim(),
            setup_chapter_id: document.getElementById('fsSetup').value
          }).then(function () { UI.toast('伏笔已添加', 'success'); self.load(); })
            .catch(function (e) { UI.toast('添加失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  edit: function (id) {
    var self = this;
    var items = this._items || [];
    var f = items.filter(function (x) { return x.id === id; })[0];
    if (!f) return;
    UI.modal({
      title: '✏️ 编辑伏笔',
      body: '<div class="form-group"><label>伏笔名称</label><input id="fsTitle2" class="form-input" value="' + esc(f.title) + '"></div>' +
        '<div class="form-group"><label>伏笔描述</label><textarea id="fsDesc2" rows="3">' + esc(f.description || '') + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
          ov.remove();
          API.updateForeshadow(id, {
            title: document.getElementById('fsTitle2').value.trim(),
            description: document.getElementById('fsDesc2').value.trim()
          }).then(function () { UI.toast('已保存', 'success'); self.load(); })
            .catch(function (e) { UI.toast('保存失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  mark: function (id, status) {
    var self = this;
    API.updateForeshadow(id, { status: status }).then(function () {
      UI.toast(status === 'recollected' ? '已标记回收 🎉' : '已标记放弃', 'success');
      self.load();
    }).catch(function (e) { UI.toast('操作失败: ' + e.message, 'error'); });
  },
  del: function (id) {
    var self = this;
    UI.confirm('删除伏笔', '确定删除这条伏笔记录？', function () {
      API.deleteForeshadow(id).then(function () { UI.toast('已删除', 'success'); self.load(); })
        .catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  scan: function () {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var self = this;
    UI.confirm('🤖 AI 扫描全书', '将通读全书章节，识别潜在的伏笔/铺垫/悬念（最多 10 条候选）。\n扫描需要 30-90 秒，确认开始？', function () {
      UI.toast('正在扫描全书，请稍候…', 'warn');
      API.scanForeshadows(p.id).then(function (items) {
        if (!items.length) { UI.toast('未识别到伏笔，可手动添加', 'warn'); return; }
        var rows = items.map(function (s, i) {
          return '<div class="fs-scan-item"><strong>' + (i + 1) + '. ' + esc(s.title) + '</strong>' +
            (s.description ? '<div>' + esc(s.description) + '</div>' : '') +
            '<div class="fs-meta">埋设于 ' + esc(self.chLabel(s.chapter_id)) + '</div>' +
            '<button class="btn btn-primary btn-sm" onclick="ForeshadowPage.scanAdd(\'' + i + '\')">✓ 入库</button></div>';
        }).join('');
        self._scanItems = items;
        UI.modal({
          title: '🤖 扫描结果（点击「入库」保存）',
          body: rows,
          actions: [{ id: 'ok', label: '关闭', cls: 'btn-ghost' }]
        });
      }).catch(function (e) { UI.toast('扫描失败: ' + e.message, 'error'); });
    });
  },
  scanAdd: function (i) {
    var s = this._scanItems[i];
    if (!s) return;
    var self = this;
    API.createForeshadow({
      project_id: Store.state.currentProject.id,
      title: s.title,
      description: s.description,
      setup_chapter_id: s.chapter_id
    }).then(function () { UI.toast('「' + s.title + '」已入库', 'success'); self.load(); })
      .catch(function (e) { UI.toast('入库失败: ' + e.message, 'error'); });
  }
};

/* ---------- 2. 素材库 ---------- */
var MaterialsPage = {
  CATS: ['句式', '动作描写', '对话标签', '环境描写', '词汇', '其他'],
  CAT_COLOR: ['#e11d48', '#0891b2', '#7c3aed', '#059669', '#d97706', '#64748b'],
  init: function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty(); return; }
    this.load();
  },
  showEmpty: function (msg) {
    var el = document.getElementById('materialsList');
    el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">📚</div><div>' + (msg || '请先在左侧选中一个项目') + '</div></div>';
  },
  load: async function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty(); return; }
    var el = document.getElementById('materialsList');
    var cat = (document.getElementById('matCatFilter') || {}).value || '';
    el.innerHTML = '<div class="loading">加载中</div>';
    try {
      var items = await API.listWritingMaterials(p.id, cat);
      this._items = items;
      this.render(items);
    } catch (e) {
      el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>' + esc(e.message) + '</div></div>';
    }
  },
  catColor: function (cat) {
    var i = this.CATS.indexOf(cat);
    return this.CAT_COLOR[i >= 0 ? i : this.CATS.length - 1];
  },
  render: function (items) {
    var el = document.getElementById('materialsList');
    var stats = document.getElementById('materialsStats');
    stats.textContent = items.length + ' 条素材';
    if (!items.length) {
      el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">📚</div><div>暂无素材<br><small style="color:var(--muted)">用「🤖 拆书提取」从已有章节提炼表达素材，生成时自动融合仿写</small></div></div>';
      return;
    }
    var html = '';
    items.forEach(function (m) {
      var color = MaterialsPage.catColor(m.category);
      html += '<div class="mat-card" style="border-left:3px solid ' + color + '">' +
        '<div class="mat-head"><span class="mat-cat" style="background:' + color + '">' + esc(m.category) + '</span>' +
        '<span class="mat-src">' + esc(m.source || '') + '</span>' +
        '<span class="mat-acts"><button class="btn btn-ghost btn-sm" onclick="MaterialsPage.edit(\'' + m.id + '\')">✏️</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="MaterialsPage.del(\'' + m.id + '\')">🗑</button></span></div>' +
        '<div class="mat-content">' + esc(m.content) + '</div></div>';
    });
    el.innerHTML = html;
  },
  create: function () {
    var self = this;
    var catOpts = this.CATS.map(function (c) { return '<option>' + c + '</option>'; }).join('');
    UI.modal({
      title: '＋ 手动添加素材',
      body: '<div class="form-group"><label>类别</label><select id="matCat">' + catOpts + '</select></div>' +
        '<div class="form-group"><label>素材内容 *</label><textarea id="matContent" rows="4" placeholder="如：她攥紧衣角，指节泛白"></textarea></div>' +
        '<div class="form-group"><label>来源（可选）</label><input id="matSource" class="form-input" placeholder="如：第3章 / 某本书 / 自己积累"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var content = document.getElementById('matContent').value.trim();
          if (!content) { UI.toast('请输入素材内容', 'warn'); return; }
          ov.remove();
          API.createWritingMaterial({
            project_id: Store.state.currentProject.id,
            category: document.getElementById('matCat').value,
            content: content,
            source: document.getElementById('matSource').value.trim()
          }).then(function () { UI.toast('素材已添加', 'success'); self.load(); })
            .catch(function (e) { UI.toast('添加失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  edit: function (id) {
    var self = this;
    var m = this._items.filter(function (x) { return x.id === id; })[0];
    if (!m) return;
    var catOpts = this.CATS.map(function (c) { return '<option' + (c === m.category ? ' selected' : '') + '>' + c + '</option>'; }).join('');
    UI.modal({
      title: '✏️ 编辑素材',
      body: '<div class="form-group"><label>类别</label><select id="matCat2">' + catOpts + '</select></div>' +
        '<div class="form-group"><label>素材内容</label><textarea id="matContent2" rows="4">' + esc(m.content) + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (mm, ov) {
          ov.remove();
          API.updateWritingMaterial(id, {
            category: document.getElementById('matCat2').value,
            content: document.getElementById('matContent2').value.trim()
          }).then(function () { UI.toast('已保存', 'success'); self.load(); })
            .catch(function (e) { UI.toast('保存失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  del: function (id) {
    var self = this;
    UI.confirm('删除素材', '确定删除这条素材？', function () {
      API.deleteWritingMaterial(id).then(function () { UI.toast('已删除', 'success'); self.load(); })
        .catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  extract: function () {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var self = this;
    UI.modal({
      title: '🤖 拆书提取素材',
      body: '<p style="font-size:12px;color:var(--muted);margin-bottom:10px">AI 将按「句式 / 动作描写 / 对话标签 / 环境描写 / 词汇」从文本中提取表达素材并入库（生成时自动融合，去AI味）。</p>' +
        '<div class="form-group"><label>提取范围</label><select id="matExtractScope">' +
        '<option value="book">全书全部章节（逐章提取，耗时较长）</option>' +
        '<option value="paste">粘贴文本（自定义）</option></select></div>' +
        '<div class="form-group" id="matExtractPasteWrap" style="display:none"><label>粘贴文本</label><textarea id="matExtractPaste" rows="6" placeholder="粘贴要拆解的文本"></textarea></div>' +
        '<label style="font-size:12px"><input type="checkbox" id="matExtractClear"> 清空现有素材后重新提取（整书重拆）</label>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '开始提取', cls: 'btn-primary', onClick: function (m, ov) {
          var scope = document.getElementById('matExtractScope').value;
          var clear = document.getElementById('matExtractClear').checked;
          ov.remove();
          UI.toast('提取中，请稍候…', 'warn');
          if (scope === 'paste') {
            var content = document.getElementById('matExtractPaste').value;
            if (!content.trim()) { UI.toast('请粘贴文本', 'warn'); return; }
            API.extractMaterials(p.id, '', content, clear).then(function (d) {
              UI.toast('提取完成：新增 ' + d.count + ' 条素材', 'success');
              self.load();
            }).catch(function (e) { UI.toast('提取失败: ' + e.message, 'error'); });
          } else {
            API.listChapters(p.id).then(function (chs) {
              var seq = Promise.resolve();
              var total = 0;
              chs.forEach(function (ch) {
                if (!ch.content) return;
                seq = seq.then(function () {
                  return API.extractMaterials(p.id, ch.id, '', false).then(function (d) { total += d.count || 0; })
                    .catch(function () {});
                });
              });
              seq.then(function () {
                UI.toast('全书拆解完成：新增 ' + total + ' 条素材', 'success');
                self.load();
              });
            }).catch(function (e) { UI.toast('加载章节失败: ' + e.message, 'error'); });
          }
        }}
      ]
    });
  },
  search: function (q) {
    var self = this;
    clearTimeout(this._searchTimer);
    this._searchTimer = setTimeout(function () {
      var p = Store.state.currentProject;
      if (!p) return;
      if (!q.trim()) { self.load(); return; }
      API.searchMaterials(p.id, q).then(function (items) {
        if (!items.length) {
          document.getElementById('materialsList').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">🔍</div><div>无相似素材</div></div>';
          return;
        }
        self.render(items);
      }).catch(function () {});
    }, 300);
  }
};

/* ---------- 3. 构思 Agent ---------- */
var ConceptAgent = {
  idea: '',
  start: function () {
    var self = this;
    UI.modal({
      title: '💡 小说构思 Agent',
      body: '<p style="font-size:12px;color:var(--muted);margin-bottom:10px">把你的创意告诉 AI 编辑，它会追问关键问题，帮你打磨出完整构思方案（定位/主角/冲突/世界观/路线图/开篇钩子/伏笔建议）。</p>' +
        '<div class="form-group"><label>创意描述 *</label><textarea id="caIdea" rows="3" placeholder="如：一个少年在山洞捡到神秘戒指，踏上修仙之路"></textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '下一步：AI 追问', cls: 'btn-primary', onClick: function (m, ov) {
          var idea = document.getElementById('caIdea').value.trim();
          if (!idea) { UI.toast('请输入创意描述', 'warn'); return; }
          self.idea = idea;
          ov.remove();
          self.ask(idea);
        }}
      ]
    });
  },
  ask: function (idea) {
    var self = this;
    UI.modal({
      title: '💡 构思追问（回答越具体，方案越好）',
      body: '<div id="caAskBody" class="loading">AI 正在提问…</div>',
      actions: [{ id: 'cancel', label: '取消' }]
    });
    API.conceptAsk(idea).then(function (d) {
      var qs = (d.questions || '').split('\n').filter(function (l) { return l.trim(); });
      var qHtml = qs.map(function (q) { return '<div style="font-size:12px;padding:6px 10px;background:var(--panel2);border-radius:6px;margin-bottom:6px">' + esc(q) + '</div>'; }).join('');
      var body = document.getElementById('caAskBody');
      body.innerHTML = qHtml + '<div class="form-group" style="margin-top:10px"><label>我的回答（逐条编号回答）</label><textarea id="caAnswers" rows="6" placeholder="1. ...&#10;2. ...&#10;3. ..."></textarea></div>' +
        '<button class="btn btn-primary btn-block" onclick="ConceptAgent.complete()">✨ 生成构思方案</button>';
    }).catch(function (e) {
      var body = document.getElementById('caAskBody');
      if (body) body.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>' + esc(e.message) + '</div></div>';
    });
  },
  complete: function () {
    var answers = document.getElementById('caAnswers').value.trim();
    if (!answers) { UI.toast('请先回答至少一个问题', 'warn'); return; }
    var self = this;
    var body = document.getElementById('caAskBody');
    body.innerHTML = '<div class="loading">AI 正在生成构思方案…</div>';
    API.conceptComplete(this.idea, answers).then(function (d) {
      self._concept = d.concept || '';
      body.innerHTML = '<div style="max-height:400px;overflow:auto;font-size:12px;line-height:1.8;white-space:pre-wrap;background:var(--panel2);padding:12px;border-radius:8px">' + esc(self._concept) + '</div>' +
        '<div style="margin-top:10px;display:flex;gap:8px">' +
        '<button class="btn btn-primary btn-sm" onclick="ConceptAgent.saveToOutline()">📋 存入大纲</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="ConceptAgent.copy()">📋 复制</button></div>';
    }).catch(function (e) {
      body.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>' + esc(e.message) + '</div></div>';
    });
  },
  saveToOutline: function () {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var self = this;
    API.updateProject(p.id, { outline: this._concept }).then(function () {
      UI.toast('构思方案已存入项目大纲', 'success');
      if (typeof OutlinePage !== 'undefined') OutlinePage.load();
    }).catch(function (e) { UI.toast('保存失败: ' + e.message, 'error'); });
  },
  copy: function () {
    navigator.clipboard.writeText(this._concept || '').then(function () { UI.toast('已复制', 'success'); }).catch(function () {});
  }
};

/* ---------- 4. 角色关系图谱 ---------- */
var RelationGraph = {
  show: function (force) {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    UI.modal({
      title: '🕸 人物关系图谱',
      wide: 'min(94vw, 1000px)',
      body: '<div id="rgBody" class="loading">AI 正在分析正文与人物卡…</div>',
      actions: [
        { id: 'refresh', label: '🔄 重新分析', cls: 'btn-ghost', onClick: function (m, ov) { ov.remove(); RelationGraph.show(true); } },
        { id: 'ok', label: '关闭', cls: 'btn-ghost' }
      ]
    });
    API.characterRelations(p.id, '', force).then(function (d) {
      var body = document.getElementById('rgBody');
      if (!body) return;
      var data = d.data;
      if (!data || !data.relations || !data.relations.length) {
        body.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">🕸</div><div>未提取到关系，请先写几章正文或建立人物卡</div></div>';
        return;
      }
      body.innerHTML = (d.cached ? '<div style="font-size:11px;color:var(--muted);margin-bottom:6px">⚡ 缓存结果（10分钟内），点右上角「重新分析」获取最新</div>' : '') +
        RelationGraph.renderSVG(data);
    }).catch(function (e) {
      var body = document.getElementById('rgBody');
      if (body) body.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>' + esc(e.message) + '</div></div>';
    });
  },
  renderSVG: function (data) {
    var chars = data.characters || [];
    var relations = data.relations || [];
    if (!chars.length) {
      var names = {};
      relations.forEach(function (r) { names[r.from] = 1; names[r.to] = 1; });
      chars = Object.keys(names).map(function (n) { return { name: n, desc: '' }; });
    }
    // 大画布 + 大字号：viewBox 900×560，弹窗已加宽至 ~1000px，桌面端 1:1 显示清晰可读
    var W = 900, H = 560, cx = W / 2, cy = H / 2, R = Math.min(W, H) / 2 - 100;
    var pos = {};
    chars.forEach(function (c, i) {
      var ang = (i / Math.max(chars.length, 1)) * Math.PI * 2 - Math.PI / 2;
      pos[c.name] = { x: cx + R * Math.cos(ang), y: cy + R * Math.sin(ang) };
    });
    var colors = ['#f59e0b', '#ef4444', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316'];
    var colorOf = {};
    chars.forEach(function (c, i) { colorOf[c.name] = colors[i % colors.length]; });

    var lines = relations.map(function (r) {
      var a = pos[r.from], b = pos[r.to];
      if (!a || !b) return '';
      var mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2;
      return '<line x1="' + a.x + '" y1="' + a.y + '" x2="' + b.x + '" y2="' + b.y + '" stroke="' + colorOf[r.from] + '" stroke-width="1.6" stroke-opacity="0.5"/>' +
        '<text x="' + mx + '" y="' + (my - 6) + '" text-anchor="middle" font-size="12" fill="var(--muted)">' + esc(r.type) + '</text>';
    }).join('');
    var nodes = chars.map(function (c) {
      var p = pos[c.name];
      if (!p) return '';
      return '<g><circle cx="' + p.x + '" cy="' + p.y + '" r="30" fill="' + colorOf[c.name] + '" fill-opacity="0.15" stroke="' + colorOf[c.name] + '" stroke-width="2"/>' +
        '<text x="' + p.x + '" y="' + p.y + '" text-anchor="middle" dominant-baseline="central" font-size="16" font-weight="600" fill="var(--text)">' + esc(c.name) + '</text>' +
        (c.desc ? '<text x="' + p.x + '" y="' + (p.y + 48) + '" text-anchor="middle" font-size="11" fill="var(--muted)">' + esc(c.desc.length > 14 ? c.desc.substring(0, 14) + '…' : c.desc) + '</text>' : '') + '</g>';
    }).join('');
    return '<svg viewBox="0 0 ' + W + ' ' + H + '" style="width:100%;height:auto">' + lines + nodes + '</svg>' +
      '<div style="font-size:12px;color:var(--muted);margin-top:8px">线条颜色 = 发起方；悬停放大可查看细节</div>';
  }
};

/* ---------- 5. 场景节拍（细纲） ---------- */
var SceneBeatUI = {
  show: function (chapterId, chapterTitle) {
    var p = Store.state.currentProject;
    if (!p) return UI.toast('请先选中一个项目', 'warn');
    var self = this;
    this._chapterId = chapterId;
    UI.modal({
      title: '🎬 场景节拍 · ' + chapterTitle,
      body: '<div id="sbBody" class="loading">加载中…</div>',
      actions: [
        { id: 'add', label: '＋ 添加场景', cls: 'btn-primary', onClick: function (m, ov) { self.add(); } },
        { id: 'ok', label: '关闭', cls: 'btn-ghost' }
      ]
    });
    this.load();
  },
  load: function () {
    var self = this;
    API.listSceneBeats(this._chapterId).then(function (items) {
      self._items = items;
      var body = document.getElementById('sbBody');
      if (!body) return;
      if (!items.length) {
        body.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">🎬</div><div>本场景节拍：把一章拆成多个场景卡，写作前理清节奏</div></div>';
        return;
      }
      var html = items.map(function (b, i) {
        return '<div class="sb-card"><div class="sb-head"><strong>' + (i + 1) + '. ' + esc(b.title) + '</strong>' +
          '<span class="sb-acts"><button class="btn btn-ghost btn-sm" onclick="SceneBeatUI.edit(\'' + b.id + '\')">✏️</button>' +
          '<button class="btn btn-ghost btn-sm" onclick="SceneBeatUI.del(\'' + b.id + '\')">🗑</button></span></div>' +
          (b.summary ? '<div class="sb-summary">' + esc(b.summary) + '</div>' : '') + '</div>';
      }).join('');
      body.innerHTML = html;
    }).catch(function (e) {
      var body = document.getElementById('sbBody');
      if (body) body.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>' + esc(e.message) + '</div></div>';
    });
  },
  add: function () {
    var self = this;
    UI.modal({
      title: '＋ 添加场景卡',
      body: '<div class="form-group"><label>场景标题 *</label><input id="sbTitle" class="form-input" placeholder="如：深夜跟踪 / 对峙揭露"></div>' +
        '<div class="form-group"><label>场景摘要 / 节拍</label><textarea id="sbSummary" rows="4" placeholder="这个场景发生什么？推进什么剧情/人物弧光？"></textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var title = document.getElementById('sbTitle').value.trim();
          if (!title) { UI.toast('请输入场景标题', 'warn'); return; }
          ov.remove();
          API.createSceneBeat({
            project_id: Store.state.currentProject.id,
            chapter_id: self._chapterId,
            title: title,
            summary: document.getElementById('sbSummary').value.trim()
          }).then(function () { UI.toast('场景已添加', 'success'); self.load(); })
            .catch(function (e) { UI.toast('添加失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  edit: function (id) {
    var self = this;
    var b = this._items.filter(function (x) { return x.id === id; })[0];
    if (!b) return;
    UI.modal({
      title: '✏️ 编辑场景',
      body: '<div class="form-group"><label>场景标题</label><input id="sbTitle2" class="form-input" value="' + esc(b.title) + '"></div>' +
        '<div class="form-group"><label>场景摘要</label><textarea id="sbSummary2" rows="4">' + esc(b.summary || '') + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
          ov.remove();
          API.updateSceneBeat(id, {
            title: document.getElementById('sbTitle2').value.trim(),
            summary: document.getElementById('sbSummary2').value.trim()
          }).then(function () { UI.toast('已保存', 'success'); self.load(); })
            .catch(function (e) { UI.toast('保存失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  del: function (id) {
    var self = this;
    UI.confirm('删除场景', '确定删除这个场景卡？', function () {
      API.deleteSceneBeat(id).then(function () { UI.toast('已删除', 'success'); self.load(); })
        .catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  }
};

/* ---------- 6. 文风样本库（本地知识库：用户自购书风格参考） ---------- */
var StyleBankPage = {
  CATS: ['热血燃向', '克苏鲁悬疑', '无限流惊悚', '校园青春', '史诗奇幻', '悬疑暗黑', '其他'],
  init: function () {
    this.load();
  },
  load: async function () {
    var el = document.getElementById('stylebankList');
    if (!el) return;
    var cat = (document.getElementById('sbCatFilter') || {}).value || '';
    el.innerHTML = '<div class="loading">加载中</div>';
    try {
      var items = await API.listStyleSamples(cat);
      this._items = items;
      this.render(items);
    } catch (e) {
      el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>' + esc(e.message) + '</div></div>';
    }
  },
  selectedIds: function () {
    return (Store.state.composer && Store.state.composer.styleSampleIds) || [];
  },
  isSelected: function (id) {
    return this.selectedIds().indexOf(id) >= 0;
  },
  render: function (items) {
    var el = document.getElementById('stylebankList');
    var stats = document.getElementById('stylebankStats');
    var sel = this.selectedIds();
    if (stats) stats.textContent = items.length + ' 段 · 已选 ' + sel.length + ' 段作参考';
    if (!items.length) {
      el.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">📚</div><div>文风样本库为空<br><small style="color:var(--muted)">用导入工具将自购书籍片段入库，或手动添加</small></div></div>';
      return;
    }
    var self = this;
    var html = items.map(function (m) {
      var brief = m.content.replace(/\s+/g, ' ');
      var r = Array.from(brief);
      var preview = r.length > 120 ? r.slice(0, 120).join('') + '…' : brief;
      var on = self.isSelected(m.id);
      return '<div class="mat-card" style="border-left:3px solid #8b5cf6">' +
        '<div class="mat-head"><strong>' + esc(m.title) + '</strong>' +
        (m.author ? '<span style="font-size:11px;color:var(--muted)"> ' + esc(m.author) + '</span>' : '') +
        '<span class="mat-cat" style="background:#8b5cf6;margin-left:6px">' + esc(m.category || '其他') + '</span>' +
        '<span class="mat-src">' + esc(m.source_file || '') + '</span>' +
        '<span class="mat-acts">' +
        '<button class="btn ' + (on ? 'btn-primary' : 'btn-ghost') + ' btn-sm" onclick="StyleBankPage.toggleSelect(\'' + m.id + '\')">' + (on ? '✓ 已选' : '选作文风参考') + '</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="StyleBankPage.edit(\'' + m.id + '\')">✏️</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="StyleBankPage.del(\'' + m.id + '\')">🗑</button></span></div>' +
        '<div class="mat-content" style="font-size:12px;line-height:1.7;color:var(--muted)">' + esc(preview) + '</div></div>';
    }).join('');
    el.innerHTML = html;
  },
  toggleSelect: function (id) {
    var ids = this.selectedIds();
    var i = ids.indexOf(id);
    if (i >= 0) {
      ids.splice(i, 1);
      UI.toast('已取消该文风参考', '');
    } else {
      if (ids.length >= 3) { UI.toast('最多同时选 3 段文风参考', 'warn'); return; }
      ids.push(id);
      UI.toast('已选作文风参考（生成时注入）', 'success');
    }
    Store.state.composer.styleSampleIds = ids.slice();
    this.load();
  },
  create: function () {
    var self = this;
    var catOpts = this.CATS.map(function (c) { return '<option>' + c + '</option>'; }).join('');
    UI.modal({
      title: '＋ 手动添加文风样本',
      body: '<div class="form-group"><label>作品名 *</label><input id="sbTitle" class="form-input"></div>' +
        '<div class="form-group"><label>作者</label><input id="sbAuthor" class="form-input"></div>' +
        '<div class="form-group"><label>风格标签</label><select id="sbCat">' + catOpts + '</select></div>' +
        '<div class="form-group"><label>片段正文 *</label><textarea id="sbContent" rows="6" placeholder="粘贴你收藏的好文段（300-700 字最佳）"></textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var title = document.getElementById('sbTitle').value.trim();
          var content = document.getElementById('sbContent').value.trim();
          if (!title || !content) { UI.toast('作品名与片段正文必填', 'warn'); return; }
          ov.remove();
          API.createStyleSample({
            title: title,
            author: document.getElementById('sbAuthor').value.trim(),
            category: document.getElementById('sbCat').value,
            content: content
          }).then(function () { UI.toast('样本已添加', 'success'); self.load(); })
            .catch(function (e) { UI.toast('添加失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  edit: function (id) {
    var self = this;
    var m = (this._items || []).filter(function (x) { return x.id === id; })[0];
    if (!m) return;
    UI.modal({
      title: '✏️ 编辑文风样本',
      body: '<div class="form-group"><label>作品名</label><input id="sbTitle2" class="form-input" value="' + esc(m.title) + '"></div>' +
        '<div class="form-group"><label>作者</label><input id="sbAuthor2" class="form-input" value="' + esc(m.author || '') + '"></div>' +
        '<div class="form-group"><label>风格标签</label><input id="sbCat2" class="form-input" value="' + esc(m.category || '') + '"></div>' +
        '<div class="form-group"><label>片段正文</label><textarea id="sbContent2" rows="6">' + esc(m.content) + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (mm, ov) {
          ov.remove();
          API.updateStyleSample(id, {
            title: document.getElementById('sbTitle2').value.trim(),
            author: document.getElementById('sbAuthor2').value.trim(),
            category: document.getElementById('sbCat2').value.trim(),
            content: document.getElementById('sbContent2').value.trim()
          }).then(function () { UI.toast('已保存', 'success'); self.load(); })
            .catch(function (e) { UI.toast('保存失败: ' + e.message, 'error'); });
        }}
      ]
    });
  },
  del: function (id) {
    var self = this;
    UI.confirm('删除文风样本', '确定删除这段样本？', function () {
      API.deleteStyleSample(id).then(function () { UI.toast('已删除', 'success'); self.load(); })
        .catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  }
};
