/* ============ composer.js：创作模式 / 生成 / 终止 ============ */
var Composer = {
  init: function () {
    var mode = Store.state.composer.runMode;
    document.getElementById('modeSelect').value = mode;
    var twm = document.getElementById('targetWordMini');
    if (twm) twm.value = Store.state.composer.targetWord;
    var slider = document.getElementById('targetWordSlider');
    if (slider) slider.value = Store.state.composer.targetWord;
    this.onModeChange(mode, true);
    var tw = document.getElementById('targetWord');
    var twSlider = document.getElementById('targetWordSlider');
    if (tw) {
      tw.addEventListener('change', function () {
        Store.state.composer.targetWord = parseInt(tw.value) || 1000;
        if (twm) twm.value = Store.state.composer.targetWord;
        if (twSlider) twSlider.value = Store.state.composer.targetWord;
        Store.savePrefs();
      });
      tw.addEventListener('input', function () { if (twSlider) twSlider.value = parseInt(tw.value) || 1000; });
    }
    if (twm) {
      twm.addEventListener('change', function () {
        Store.state.composer.targetWord = parseInt(twm.value) || 1000;
        if (tw) tw.value = Store.state.composer.targetWord;
        if (twSlider) twSlider.value = Store.state.composer.targetWord;
        Store.savePrefs();
      });
      twm.addEventListener('input', function () { if (twSlider) twSlider.value = parseInt(twm.value) || 1000; });
    }
    // 上下文范围
    var scopeEl = document.getElementById('contextScope');
    if (scopeEl) scopeEl.value = Store.state.composer.contextScope || 'current';
    Store.state.composer.contextScope = Store.state.composer.contextScope || 'current';
    // 禁改写
    var nrToggle = document.getElementById('noRewriteToggle');
    if (nrToggle) nrToggle.checked = Store.state.composer.noRewrite || false;
    Store.state.composer.noRewrite = Store.state.composer.noRewrite || false;
    // 跳过字数校验
    var swToggle = document.getElementById('skipWordCheck');
    if (swToggle) swToggle.checked = Store.state.composer.skipWordCheck || false;
    Store.state.composer.skipWordCheck = Store.state.composer.skipWordCheck || false;
    // 深度思考：全局开关（一键全开/全关）+ 角色级开关（默认推荐配置：仅规划师开，写作/审稿/轻活关）
    var thToggle = document.getElementById('thinkingToggle');
    var rt = Store.state.composer.roleThinking;
    if (!rt || typeof rt !== 'object') { rt = { thinker: true, worker: false, verifier: false, helper: false }; Store.state.composer.roleThinking = rt; }
    var allOn = rt.thinker !== false && rt.worker !== false && rt.verifier !== false && rt.helper !== false;
    if (thToggle) thToggle.checked = allOn;
    var map = { thinker: 'thinkThinker', worker: 'thinkWorker', verifier: 'thinkVerifier', helper: 'thinkHelper' };
    Object.keys(map).forEach(function (k) {
      var el = document.getElementById(map[k]);
      if (el) el.checked = rt[k] !== false;
    });
    // 自动追加
    var aaToggle = document.getElementById('autoAppendToggle');
    if (aaToggle) aaToggle.checked = Store.state.composer.autoAppend !== false;
    Store.state.composer.autoAppend = Store.state.composer.autoAppend !== false;
    // 大纲
    var goEl = document.getElementById('genOutline');
    if (goEl) goEl.value = Store.state.composer.outline || '';
  },
  onSliderChange: function () {
    var val = parseInt(document.getElementById('targetWordSlider').value) || 1000;
    Store.state.composer.targetWord = val;
    document.getElementById('targetWord').value = val;
    document.getElementById('targetWordMini').value = val;
    Store.savePrefs();
  },
  onScopeChange: function () {
    Store.state.composer.contextScope = document.getElementById('contextScope').value;
    Store.savePrefs();
  },
  onNoRewriteChange: function () {
    Store.state.composer.noRewrite = document.getElementById('noRewriteToggle').checked;
    Store.savePrefs();
  },
  onSkipWordCheck: function (checked) {
    Store.state.composer.skipWordCheck = checked;
    Store.savePrefs();
  },
  onThinkingChange: function (checked) {
    // 全局开关：true=全部角色开启思考（全开模式）；false=全部关闭
    var rt = Store.state.composer.roleThinking || {};
    if (!checked) {
      rt.thinker = false; rt.worker = false; rt.verifier = false; rt.helper = false;
    } else {
      rt.thinker = true; rt.worker = true; rt.verifier = true; rt.helper = true;
    }
    Store.state.composer.roleThinking = rt;
    var map = { thinker: 'thinkThinker', worker: 'thinkWorker', verifier: 'thinkVerifier', helper: 'thinkHelper' };
    Object.keys(map).forEach(function (k) {
      var el = document.getElementById(map[k]);
      if (el) el.checked = rt[k] !== false;
    });
    Store.savePrefs();
    UI.toast(checked ? '🧠 深度思考已开启（全部角色，更慢但质量最稳）' : '⚡ 深度思考已全部关闭（最快）', '');
  },
  onRoleThinkingChange: function (role, checked) {
    var rt = Store.state.composer.roleThinking || { thinker: true, worker: false, verifier: false, helper: false };
    rt[role] = checked;
    Store.state.composer.roleThinking = rt;
    // 同步全局开关状态
    var thToggle = document.getElementById('thinkingToggle');
    if (thToggle) thToggle.checked = rt.thinker !== false && rt.worker !== false && rt.verifier !== false && rt.helper !== false;
    Store.savePrefs();
    var label = { thinker: '规划师', worker: '写作', verifier: '审稿', helper: '轻活' }[role] || role;
    UI.toast((checked ? '🧠 ' : '⚡ ') + label + '角色' + (checked ? '已开启' : '已关闭') + '思考', '');
  },
  onModeChange: function (mode, skipPersist) {
    Store.state.composer.runMode = mode;
    document.getElementById('modeSelect').value = mode;
    var hints = {
      auto: '智能协同：自动判定任务并匹配流水线',
      collab: '协同闭环：Thinker规划→Worker写作→Verifier审查→Thinker重规划→Worker重写',
      orchestrated: '为每个角色手动指定模型，跑完整 Thinker→Worker→Verifier 流程',
      draft: '快速草稿：跳过 Thinker+Verifier，Worker 直出初稿（可后续深度优化）',
      strict: '严谨模式：Thinker 初稿 → Worker 润色 → Verifier 严校',
      art: '文艺创作：极简框架 → Worker 高度自由创作 → 宽松审查',
      light: '轻量化快速：直接调用 Helper（选中文本超 500 字请切换模式）',
      manual: '手动自选模型：跳过流水线，直接调用指定模型'
    };
    document.getElementById('modeHint').textContent = hints[mode] || '';
    // 模式切换反馈：toast 提示（用户能感知切换成功）
    if (typeof UI !== 'undefined' && UI.toast && !skipPersist) {
      UI.toast('已切换：' + (hints[mode] || mode), 'info');
    }
    var modelSel = document.getElementById('modelSelect');
    var orchPanel = document.getElementById('orchestratedModels');
    if (mode === 'orchestrated') {
      modelSel.style.display = 'none';
      if (orchPanel) orchPanel.style.display = '';
      this.refreshOrchestratedModels();
    } else if (mode === 'manual') {
      modelSel.style.display = '';
      this.refreshModels();
    } else {
      modelSel.style.display = 'none';
      if (orchPanel) orchPanel.style.display = 'none';
    }
    if (!skipPersist) Store.savePrefs();
  },
  refreshModels: async function () {
    try {
      var models = await API.listModels();
      Store.state.models = models;
      var sel = document.getElementById('modelSelect');
      var cur = Store.state.composer.modelName;
      var active = models.filter(function (m) { return m.status === 'active'; });
      var html = active.map(function (m) {
        return '<option value="' + esc(m.name) + '" data-id="' + esc(m.id) + '"' + (m.name === cur ? ' selected' : '') + '>' + esc(m.name) + (m.vendor ? ' · ' + esc(m.vendor) : '') + (m.is_default === 1 ? ' ⭐' : '') + '</option>';
      }).join('');
      if (!html) html = '<option value="">未配置自定义 API 模型</option>';
      sel.innerHTML = html;
      sel.onchange = function () {
        Store.state.composer.modelName = sel.value;
        var opt = sel.options[sel.selectedIndex];
        Store.state.composer.modelConfigId = opt ? (opt.getAttribute('data-id') || '') : '';
        Store.savePrefs();
      };
      if (!Store.state.composer.modelName && active.length) {
        Store.state.composer.modelName = active[0].name;
        sel.value = active[0].name;
        Store.state.composer.modelConfigId = active[0].id;
        Store.savePrefs();
      }
    } catch (e) { UI.toast('模型列表加载失败', 'error'); }
  },
  refreshOrchestratedModels: async function () {
    try {
      var models = await API.listModels();
      Store.state.models = models;
      var active = models.filter(function (m) { return m.status === 'active'; });
      var roles = [
        { key: 'thinker',  label: '🖊️ 构思大纲',  storeKey: 'orchThinker'  },
        { key: 'worker',   label: '📝 动笔写作',  storeKey: 'orchWorker'   },
        { key: 'verifier', label: '🔍 品质审稿',  storeKey: 'orchVerifier' }
      ];
      var panel = document.getElementById('orchestratedModels');
      var body = roles.map(function (r) {
        var cur = Store.state.composer[r.storeKey] || '';
        var opts = active.map(function (m) {
          return '<option value="' + esc(m.name) + '"' + (m.name === cur ? ' selected' : '') + '>' + esc(m.name) + '</option>';
        }).join('');
        if (!opts) opts = '<option value="">无可用模型</option>';
        return '<div style="display:flex;align-items:center;gap:6px;margin:4px 0;font-size:11px">' +
          '<span style="width:80px;color:var(--muted)">' + r.label + '</span>' +
          '<select id="orch-' + r.key + '" style="flex:1;font-size:11px;padding:3px 5px" onchange="Composer.onOrchModelChange()">' + opts + '</select></div>';
      }).join('');
      panel.innerHTML = '<div id="orchToggle" onclick="Composer.toggleOrchPanel(this)" style="cursor:pointer;font-size:11px;color:var(--accent);padding:2px 4px;user-select:none;white-space:nowrap" title="展开/收起各角色模型配置">⚙️ 模型配置 <span class="orch-arrow">▾</span></div>' +
        '<div id="orchBody" style="display:none">' + body + '</div>';
      // auto-select defaults (only once, avoid recursion)
      if (active.length && !this._orchDefaultsSet) {
        this._orchDefaultsSet = true;
        if (!Store.state.composer.orchThinker) { Store.state.composer.orchThinker = active[0].name; }
        if (!Store.state.composer.orchWorker) { Store.state.composer.orchWorker = active[0].name; }
        if (!Store.state.composer.orchVerifier) { Store.state.composer.orchVerifier = active.length > 1 ? active[1].name : active[0].name; }
        this._orchDefaultsSet = false;
      }
    } catch (e) { /* silent */ }
  },
  // 折叠/展开「指派Agent模型」的角色模型配置
  toggleOrchPanel: function (el) {
    var body = document.getElementById('orchBody');
    if (!body) return;
    var show = body.style.display !== 'block';
    body.style.display = show ? 'block' : 'none';
    var arrow = el.querySelector('.orch-arrow');
    if (arrow) arrow.textContent = show ? '▴' : '▾';
  },
  onOrchModelChange: function () {
    Store.state.composer.orchThinker = document.getElementById('orch-thinker')?.value || '';
    Store.state.composer.orchWorker = document.getElementById('orch-worker')?.value || '';
    Store.state.composer.orchVerifier = document.getElementById('orch-verifier')?.value || '';
    Store.savePrefs();
  },
  setGenerating: function (g) {
    var btn = document.getElementById('btnGenerate');
    btn.disabled = g;
    btn.textContent = g ? '⏳ 生成中…' : '✨ 生成';
    document.getElementById('genStatus').classList.toggle('show', g);
    document.getElementById('instructionInput').readOnly = g;
    if (g) {
      Editor.lock();
      Sidebar.lock();
    } else {
      Editor.unlock();
      Sidebar.unlock();
    }
  },
  buildPayload: function () {
    var p = Store.state.currentProject;
    var ch = Store.state.currentChapter;
    var instr = document.getElementById('instructionInput').value.trim();
    var sel = Editor.getSelectedText() || '';
    var tw = Store.state.composer.targetWord;
    var scope = Store.state.composer.contextScope || 'current';
    var cursorPos = Store.state.composer.cursorPosition || 0;
    var noRewrite = Store.state.composer.noRewrite || false;
    // context_scope: 如果选择含前面章节摘要，拼接摘要
    var summaries = '';
    if (scope === 'withSummary' && Store.state.chapterSummaries) {
      summaries = Store.state.chapterSummaries;
    }
    // 如果选中了文字，以选中文字为上下文
    var history = '';
    if (cursorPos > 0 && !sel) {
      history = (ch ? ch.content : Editor.getText()).substring(0, cursorPos);
    } else {
      history = ch ? ch.content : Editor.getText();
    }
    // 文风参考：选定章节内容注入 material_text
    var material = Context.materials();
    var styleChId = Store.state.composer.styleChapterId;
    if (styleChId) {
      var intensity = Store.state.composer.styleIntensity || 'medium';
      var intensityLabel = {light:'弱：仅参考语感', medium:'中：对齐句式节奏', strong:'强：严格模仿所有细节'}[intensity] || '中';
      var styleCh = Store.state.chapters.find(function (c) { return c.id === styleChId; });
      if (styleCh && styleCh.content) {
        material = '【文风参考样本（强度：' + intensityLabel + '）】\n' + styleCh.content + '\n\n' + material;
      }
    }
    return {
      project_id: p ? p.id : '',
      chapter_id: ch ? ch.id : '',
      user_demand: instr,
      selected_text: sel,
      outline: (document.getElementById('genOutline') || {}).value || Store.state.composer.outline || '',
      world_setting: Context.worldSetting(),
      character_setting: Context.characters(),
      history_content: history,
      material_text: material,
      target_word: tw,
      run_mode: Store.state.composer.runMode,
      model_name: Store.state.composer.runMode === 'manual' ? Store.state.composer.modelName : '',
      model_config_id: Store.state.composer.runMode === 'manual' ? Store.state.composer.modelConfigId : '',
      role_models: Store.state.composer.runMode === 'orchestrated' ? {
        thinker: Store.state.composer.orchThinker || '',
        worker: Store.state.composer.orchWorker || '',
        verifier: Store.state.composer.orchVerifier || ''
      } : null,
      cursor_position: cursorPos,
      no_rewrite: noRewrite,
      context_scope: scope,
      previous_summaries: summaries,
      skip_word_check: Store.state.composer.skipWordCheck,
      role_thinking: Store.state.composer.roleThinking || { thinker: true, worker: false, verifier: false, helper: false }
    };
  },
  validate: function (payload) {
    if (!Store.state.currentProject) { UI.toast('请先选择或创建项目', 'warn'); return false; }
    if (!payload.user_demand && !payload.selected_text) { UI.toast('请输入创作需求或选中文字', 'warn'); return false; }
    if (Store.state.composer.runMode === 'manual' && !payload.model_name) { UI.toast('手动模式请选择模型', 'warn'); return false; }
    if (Store.state.composer.runMode === 'light' && payload.selected_text && Array.from(payload.selected_text).length > 500) {
      UI.toast('选中文本超过 500 字，轻量模式可能不适合，建议切换为「智能协同」', 'warn');
    }
    if (!Usage.canGenerate()) {
      UI.toast('今日额度已用完，请明日再试或联系管理员调整限额', 'error');
      return false;
    }
    return true;
  },
  generate: async function () {
    var payload = this.buildPayload();
    if (!this.validate(payload)) return;
    // 自动创建章节
    var ch = Store.state.currentChapter;
    if (!ch) {
      var p = Store.state.currentProject;
      if (!p) return;
      try {
        var chapNum = (Store.state.chapters || []).length + 1;
        var title = '第' + chapNum + '章';
        ch = await API.createChapter({ project_id: p.id, volume_id: '', title: title, content: '' });
        Store.state.currentChapter = ch;
        Store.state.chapters = Store.state.chapters || [];
        Store.state.chapters.push(ch);
        ChapterUI.renderTree();
        payload.chapter_id = ch.id;
        payload.history_content = '';
      } catch (e) { UI.toast('创建章节失败：' + e.message, 'error'); return; }
    }
    // 第一层预检：需求-字数匹配（除非用户关闭）
    if (!Store.state.composer.skipWordCheck && payload.target_word > 0 && payload.user_demand) {
      try {
        var pre = await API.post('/api/precheck', {
          user_demand: payload.user_demand,
          target_word: payload.target_word,
          world_setting: payload.world_setting,
          character_setting: payload.character_setting,
          history_content: payload.history_content
        });
        if (pre && pre.mismatch) {
          this.showPrecheckDialog(pre, payload);
          return;
        }
      } catch (e) { /* precheck 失败不阻塞，直接继续 */ }
    }
    this.doGenerate(payload);
  },

  showPrecheckDialog: function (pre, payload) {
    var mismatchLabel = pre.mismatch_type === 'too_low' ? '（预估参考）需求体量较大，当前目标字数偏低' : '（预估参考）需求体量较小，当前目标字数偏高';
    var body = '<div class="precheck-result">' +
      '<div class="precheck-banner ' + (pre.mismatch_type === 'too_low' ? 'warn-lo' : 'warn-hi') + '">' +
      '<span>' + mismatchLabel + '</span></div>' +
      '<div class="precheck-grid">' +
      '<div><span>预估场景</span><b>' + (pre.scene_count || '-') + '</b></div>' +
      '<div><span>登场人物</span><b>' + (pre.character_count || '-') + '</b></div>' +
      '<div><span>推荐字数区间</span><b>' + (pre.recommended_min || '?') + ' - ' + (pre.recommended_max || '?') + ' 字</b></div>' +
      '<div><span>设定目标</span><b class="' + (pre.mismatch ? 'mismatch' : '') + '">' + payload.target_word + ' 字</b></div>' +
      '</div>';
    if (pre.suggestion) body += '<div class="precheck-sug">💡 ' + esc(pre.suggestion) + '</div>';
    body += '<div class="precheck-foot"><span>模型: ' + esc(pre.model || '') + '</span><span>此为预估参考，可继续生成</span></div></div>';
    var self = this;
    UI.modal({
      title: '需求-字数匹配预检',
      sub: 'Helper 轻量分析：' + esc((pre.analysis || '').slice(0, 120)),
      body: body, wide: '500px',
      actions: [
        { id: 'adjust', label: '调整需求/字数', onClick: function (m, ov) { ov.remove(); } },
        { id: 'go', label: '仍然坚持生成', cls: 'btn-primary', onClick: function (m, ov) { ov.remove(); self.doGenerate(payload); } }
      ]
    });
  },

  doGenerate: function (payload) {
    var estInput = (payload.user_demand + payload.world_setting + payload.character_setting + payload.history_content + payload.material_text + payload.selected_text).length;
    var estTokens = Math.ceil(estInput / 1.5) + Math.ceil(payload.target_word * 1.3);
    var self = this;
    if (estTokens > 4000 || payload.target_word > 2000) {
      UI.confirm('长文本生成确认',
        '预估 Token 约 <b style="color:var(--accent)">' + estTokens.toLocaleString() + '</b>，目标字数 ' + payload.target_word + '。<br>生成可能耗时较长，是否继续？',
        function () { SSE.start(payload); });
    } else {
      SSE.start(payload);
    }
  },
  continueFromCursor: function () {
    var ch = Store.state.currentChapter;
    if (!ch) { UI.toast('请先选择章节', 'warn'); return; }
    var cursorPos = Editor.getCursorPosition();
    if (cursorPos < 0) { UI.toast('请将光标放在章节正文中', 'warn'); return; }
    Store.state.composer.cursorPosition = cursorPos;
    document.getElementById('instructionInput').value = '续写';
    this.generate();
  },
  refreshStyleChapters: function () {
    var sel = document.getElementById('styleChapter');
    if (!sel) return;
    var chs = Store.state.chapters || [];
    var curStyleId = Store.state.composer.styleChapterId || '';
    sel.innerHTML = '<option value="">文风参考：无</option>' + chs.map(function (c) {
      return '<option value="' + c.id + '"' + (c.id === curStyleId ? ' selected' : '') + '>' + esc(c.title) + '</option>';
    }).join('');
  },
  onStyleChapterChange: function () {
    var sel = document.getElementById('styleChapter');
    var chId = sel ? sel.value : '';
    Store.state.composer.styleChapterId = chId;
    Store.savePrefs();
    if (chId) UI.toast('已选定风格参考章节', 'success');
  },
  onStyleIntensityChange: function () {
    var sel = document.getElementById('styleIntensity');
    Store.state.composer.styleIntensity = sel ? sel.value : 'medium';
    Store.savePrefs();
  },
  onAutoAppendChange: function () {
    var cb = document.getElementById('autoAppendToggle');
    Store.state.composer.autoAppend = cb ? cb.checked : true;
    Store.savePrefs();
  },
  toggleGenOptions: function (ev) {
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var box = document.getElementById('genOpts');
    if (!box) return;
    var show = box.style.display === 'none';
    box.style.display = show ? 'flex' : 'none';
    var btn = ev && ev.currentTarget;
    if (btn) btn.classList.toggle('active', show);
  },
  previewRAG: async function (ev) {
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    var box = document.getElementById('ragPreview');
    var demand = (document.getElementById('instructionInput') || {}).value || '';
    var demand2 = Store.state.composer && Store.state.composer.demand || '';
    demand = demand || demand2 || '当前章节续写';
    var chId = Store.state.currentChapter ? Store.state.currentChapter.id : '';
    box.style.display = '';
    box.innerHTML = '<div class="ragp-head">🧠 AI 将参考的相关记忆 <span class="ragp-load">检索中…</span></div>';
    try {
      var r = await fetch('/api/rag/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project_id: p.id, chapter_id: chId, user_demand: demand, selected_text: '' })
      });
      var d = await r.json();
      var chunks = (d && d.chunks) || [];
      if (!chunks.length) {
        box.innerHTML = '<div class="ragp-head">🧠 相关记忆</div><div class="ragp-empty">未检索到明显相关的历史章节，AI 将只参考最近章节。</div>';
        return;
      }
      var html = '<div class="ragp-head">🧠 AI 将参考这些书中记忆 <button class="ragp-close" onclick="document.getElementById(\'ragPreview\').style.display=\'none\'">✕</button></div>';
      chunks.forEach(function (c) {
        var txt = c.text || '';
        if (txt.length > 140) txt = txt.slice(0, 140) + '…';
        html += '<div class="ragp-item"><div class="ragp-meta">第' + c.chapter_no + '章《' + (c.title || '未命名') + '》 <span class="ragp-score">' + Math.round((c.score || 0) * 100) + '%</span></div>' +
          '<div class="ragp-text">' + esc(txt) + '</div></div>';
      });
      box.innerHTML = html;
    } catch (e) {
      box.innerHTML = '<div class="ragp-head">🧠 相关记忆</div><div class="ragp-empty">检索失败：' + esc(e.message) + '</div>';
    }
  },
  onOutlineChange: function () {
    var el = document.getElementById('genOutline');
    Store.state.composer.outline = el ? el.value.trim() : '';
    Store.savePrefs();
    // 自动保存大纲到后端（2秒防抖）
    if (this._outlineTimer) clearTimeout(this._outlineTimer);
    var self = this;
    this._outlineTimer = setTimeout(function () {
      var p = Store.state.currentProject;
      if (p && Store.state.composer.outline !== (p.outline || '')) {
        API.updateProject(p.id, { outline: Store.state.composer.outline }).catch(function () {});
      }
    }, 2000);
  },
  autoTitle: function () {
    var ch = Store.state.currentChapter;
    if (!ch) { UI.toast('请先选择章节', 'warn'); return; }
    var text = Editor.getText();
    if (!text || text.length < 30) { UI.toast('章节内容太少（至少30字）', 'warn'); return; }
    if (SSE.active) { UI.toast('请先完成当前生成', 'warn'); return; }
    // 取前3000字作分析
    var sample = text.substring(0, 3000);
    Store.state.composer._titleMode = true;
    var payload = {
      project_id: (Store.state.currentProject || {}).id || '',
      chapter_id: ch.id,
      user_demand: '为以下小说章节生成一个简短精炼的标题（5-10个汉字，不含引号和标点），只输出标题，不要任何解释',
      history_content: sample,
      target_word: 10,
      run_mode: 'light',
      model_name: '',
      model_config_id: '',
      selected_text: '',
      world_setting: '',
      character_setting: '',
      material_text: '',
      cursor_position: 0,
      no_rewrite: false,
      context_scope: 'current',
      previous_summaries: ''
    };
    SSE.start(payload);
  }
};
