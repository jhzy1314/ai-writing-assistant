/* ============ composer.js：创作模式 / 生成 / 终止 ============ */
var Composer = {
  init: function () {
    // 一次性迁移：旧版本默认选中了列表第一个模型(deepseek-chat)，新逻辑应优先角色绑定模型(deepseek-v4-flash)
    try {
      var legacyName = Store.state.composer.modelName;
      if (legacyName === 'deepseek-chat' && Store.get('runMode', 'auto') === 'auto') {
        Store.state.composer.modelName = '';
        Store.remove('modelName');
      }
    } catch (e) {}
    var mode = Store.state.composer.runMode;
    document.getElementById('modeSelect').value = mode;
    var twm = document.getElementById('targetWordMini');
    if (twm) twm.value = Store.state.composer.targetWord;
    var wst = document.getElementById('webSearchToggle');
    if (wst) wst.checked = !!Store.state.composer.webSearch;
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
    Store.state.composer.contextScope = Store.state.composer.contextScope || 'full';
    // 禁改写
    var nrToggle = document.getElementById('noRewriteToggle');
    if (nrToggle) nrToggle.checked = Store.state.composer.noRewrite || false;
    Store.state.composer.noRewrite = Store.state.composer.noRewrite || false;
    // 跳过字数校验
    var swToggle = document.getElementById('skipWordCheck');
    if (swToggle) swToggle.checked = Store.state.composer.skipWordCheck || false;
    Store.state.composer.skipWordCheck = Store.state.composer.skipWordCheck || false;
    var roChk = document.getElementById('rewriteOutlineChk');
    if (roChk) roChk.checked = Store.state.composer.rewriteOutline !== false;
    // 深度思考：全局开关（一键全开/全关）+ 角色级开关（默认推荐配置：仅规划师开，写作/审稿/轻活关）
    var thToggle = document.getElementById('thinkingToggle');
    var rt = Store.state.composer.roleThinking;
    if (!rt || typeof rt !== 'object') { rt = { thinker: true, worker: true, verifier: false, helper: false }; Store.state.composer.roleThinking = rt; }
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
    // 新章节写入
    var ncToggle = document.getElementById('newChapterToggle');
    if (ncToggle) ncToggle.checked = Store.state.composer.newChapterWrite === true;
    Store.state.composer.newChapterWrite = Store.state.composer.newChapterWrite === true;
    // 大纲
    var goEl = document.getElementById('genOutline');
    if (goEl) goEl.value = Store.state.composer.outline || '';
    // 专业模式：恢复上次状态
    try { this.restoreProMode(); } catch (e) {}
    // 窗口尺寸变化时重算专业模式面板位置（若展开）
    var self = this;
    window.addEventListener('resize', function () {
      var p = document.getElementById('proModePanel');
      if (p && p.style.display !== 'none') self._positionProPanel();
    });
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
    var rt = Store.state.composer.roleThinking || { thinker: true, worker: true, verifier: false, helper: false };
    rt[role] = checked;
    Store.state.composer.roleThinking = rt;
    // 同步全局开关状态
    var thToggle = document.getElementById('thinkingToggle');
    if (thToggle) thToggle.checked = rt.thinker !== false && rt.worker !== false && rt.verifier !== false && rt.helper !== false;
    Store.savePrefs();
    var label = { thinker: '规划师', worker: '写作', verifier: '审稿', helper: '轻活' }[role] || role;
    UI.toast((checked ? '🧠 ' : '⚡ ') + label + '角色' + (checked ? '已开启' : '已关闭') + '思考', '');
  },
  showModeHelp: function (e) {
    if (e) e.stopPropagation();
    var modes = [
      { v: 'auto', n: '智能协同（推荐）', d: '自动判断任务类型匹配创作模式：续写/润色/审稿等分别走最合适的流程，无需手动选择。' },
      { v: 'draft', n: '快速草稿', d: '跳过构思和审稿环节，由写作 Agent 直接产出初稿，速度最快，适合灵感记录。' },
      { v: 'orchestrated', n: '指派Agent模型', d: '为规划师/写作/审稿三个角色分别手动指定模型，跑完整协同流程，适合对质量要求高的用户。' },
      { v: 'manual', n: '手动自选模型', d: '跳过流水线，直接调用指定模型生成，所见即所得。' },
      { v: 'strict', n: '严谨模式', d: 'Thinker 初稿 → Worker 润色 → Verifier 严格审校，层层把关，适合正式章节。' },
      { v: 'art', n: '文艺创作', d: '极简框架 → Worker 高度自由创作 → 宽松审查，文风更自然有灵气。' },
      { v: 'collab', n: '协同闭环', d: 'Thinker 规划 → Worker 写作 → Verifier 审查 → 发现问题自动返回重规划重写，直到通过，质量最稳但耗时较长。' },
      { v: 'light', n: '轻量快捷', d: '直接调用 Helper 处理轻量任务（缩写/摘要/改写等），选中文本超 500 字请切换其他模式。' }
    ];
    var html = '<div style="max-height:62vh;overflow-y:auto">' + modes.map(function (m) {
      return '<div style="padding:10px 12px;border-bottom:1px solid var(--border);border-radius:8px;margin-bottom:6px;background:var(--panel2)">' +
        '<div style="font-weight:600;color:var(--accent);font-size:12.5px;margin-bottom:3px">' + m.n + '</div>' +
        '<div style="font-size:11.5px;color:var(--text2);line-height:1.7">' + m.d + '</div></div>';
    }).join('') + '</div>';
    UI.modal({ title: '🎨 创作模式说明', body: html, actions: [{ id: 'ok', label: '知道了', cls: 'btn-primary' }] });
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
        // 默认模型优先：角色绑定（thinker）> is_default 标记 > 列表第一个
        var preferred = null;
        try {
          var rm = await API.getRoleModels('thinker');
          var bound = (rm && rm.item && rm.item.models) || [];
          if (bound.length) preferred = bound[0].name;
        } catch (e) {}
        if (!preferred) {
          var def = active.find(function (m) { return m.is_default === 1 || m.is_default === true; });
          if (def) preferred = def.name;
        }
        if (!preferred) preferred = active[0].name;
        Store.state.composer.modelName = preferred;
        sel.value = preferred;
        var opt = sel.querySelector('option[value="' + preferred.replace(/"/g, '\\"') + '"]');
        Store.state.composer.modelConfigId = opt ? (opt.getAttribute('data-id') || '') : (active[0].id || '');
        Store.savePrefs();
      } else if (Store.state.composer.modelName === 'deepseek-chat' && active.some(function (m) { return m.name === 'deepseek-v4-flash' && m.is_default === 1; })) {
        // 迁移：旧版本默认选了列表第一个(deepseek-chat)，现默认模型已是 deepseek-v4-flash，纠正
        Store.state.composer.modelName = 'deepseek-v4-flash';
        sel.value = 'deepseek-v4-flash';
        var opt2 = sel.querySelector('option[value="deepseek-v4-flash"]');
        Store.state.composer.modelConfigId = opt2 ? (opt2.getAttribute('data-id') || '') : '';
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
      rewrite_outline: Store.state.composer.rewriteOutline !== false,
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
      style_sample_ids: (Store.state.composer.styleSampleIds || []).slice(),
      context_scope: scope,
      previous_summaries: summaries,
      skip_word_check: Store.state.composer.skipWordCheck,
      role_thinking: Store.state.composer.roleThinking || { thinker: true, worker: true, verifier: false, helper: false },
      web_search: !!Store.state.composer.webSearch
    };
  },
  validate: function (payload) {
    if (!Store.state.currentProject) { UI.toast('请先选择或创建项目', 'warn'); return false; }
    if (!payload.user_demand && !payload.selected_text) { UI.toast('请输入创作需求或选中文字', 'warn'); return false; }
    if (Store.state.composer.runMode === 'manual' && !payload.model_name) { UI.toast('手动模式请选择模型', 'warn'); return false; }
    if (Store.state.composer.runMode === 'light' && payload.selected_text && Array.from(payload.selected_text).length > 500) {
      UI.toast('选中文本超过 500 字，轻量模式可能不适合，建议切换为「智能协同」', 'warn');
    }
    // 额度仅作展示，不限制生成（单机本地工具，用户明确要求不做额度限制）
    return true;
  },
  onRewriteOutlineChange: function () {
    var chk = document.getElementById('rewriteOutlineChk');
    Store.state.composer.rewriteOutline = chk ? chk.checked : true;
  },
  onWebSearchChange: function () {
    var el = document.getElementById('webSearchToggle');
    Store.state.composer.webSearch = !!(el && el.checked);
    try { Store.savePrefs(); } catch (e) {}
    UI.toast(Store.state.composer.webSearch ? '🌐 已开启联网搜索：AI 将联网检索资料辅助创作' : '已关闭联网搜索', Store.state.composer.webSearch ? 'success' : '');
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
    // 需求-字数预检：改为后台非阻塞（不再弹窗阻断生成，结果以 toast 提示，可继续生成）
    if (!Store.state.composer.skipWordCheck && payload.target_word > 0 && payload.user_demand) {
      this.runBackgroundPrecheck(payload);
    }
    this.doGenerate(payload);
  },

  /* 后台预检：生成已开始，预检结果仅 toast 提示，不阻断 */
  runBackgroundPrecheck: function (payload) {
    API.post('/api/precheck', {
      user_demand: payload.user_demand,
      target_word: payload.target_word,
      world_setting: payload.world_setting,
      character_setting: payload.character_setting,
      history_content: payload.history_content
    }).then(function (pre) {
      if (!pre || !pre.mismatch) return;
      var label = pre.mismatch_type === 'too_low' ? '需求体量较大' : '需求体量较小';
      UI.toast('⚠️ ' + label + '：AI 预估 ' + pre.recommended_min + '~' + pre.recommended_max + ' 字，当前目标 ' + payload.target_word + ' 字——可在「📝 章节大纲」行调整 🎯 字数', 'warn', 6000);
    }).catch(function () { /* 预检失败不打扰 */ });
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
    if (estTokens > 4000 || payload.target_word > 2000) {
      // 长文本确认：立即弹出（即时反馈），确认后立刻开始生成
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
    var self = this;
    // 每次续写都让用户现场决定写入位置：本章追加 or 新建下一章
    UI.modal({
      title: '续写位置',
      body: '<div style="font-size:12px;line-height:2">续写内容写入哪里？</div>',
      actions: [
        { id: 'same', label: '📄 写在本章', onClick: function (m, ov) {
          ov.remove();
          Store.state.composer.newChapterWrite = false;
          self._doContinue();
        }},
        { id: 'next', label: '📄 写入下一章', cls: 'btn-primary', onClick: function (m, ov) {
          ov.remove();
          Store.state.composer.newChapterWrite = true;
          self._doContinue();
        }}
      ]
    });
  },
  _doContinue: function () {
    var ch = Store.state.currentChapter;
    if (!ch) { UI.toast('请先选择章节', 'warn'); return; }
    // 「新章」模式：不依赖光标，以整个当前章节为上下文，生成内容写入下一章
    if (Store.state.composer && Store.state.composer.newChapterWrite) {
      Store.state.composer.cursorPosition = -1; // 标记：不使用光标截断
      document.getElementById('instructionInput').value = '续写';
      this.generate();
      return;
    }
    var cursorPos = Editor.getCursorPosition();
    if (cursorPos < 0) { UI.toast('请将光标放在章节正文中', 'warn'); return; }
    Store.state.composer.cursorPosition = cursorPos;
    document.getElementById('instructionInput').value = '续写';
    this.generate();
  },
  // ========== 多候选续写：一次生成 3 个不同走向，供用户挑选 ==========
  generateCandidates: async function () {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择或创建项目', 'warn'); return; }
    var ch = Store.state.currentChapter;
    if (!ch) { UI.toast('请先选择章节', 'warn'); return; }
    var payload = this.buildPayload();
    if (!payload.user_demand && !payload.selected_text && !payload.history_content) {
      UI.toast('请先写一些内容或输入续写需求', 'warn'); return;
    }
    UI.toast('🎲 正在生成 3 个候选走向，请稍候…', 'warn');
    try {
      var d = await API.post('/api/generate/candidates', {
        project_id: payload.project_id,
        chapter_id: payload.chapter_id,
        user_demand: payload.user_demand,
        selected_text: payload.selected_text,
        history_content: payload.history_content,
        world_setting: payload.world_setting,
        character_setting: payload.character_setting,
        material_text: payload.material_text,
        target_word: 750
      });
      var cands = d.candidates || [];
      if (!cands.length) { UI.toast('未生成到候选，请重试', 'error'); return; }
      this._cands = cands;
      this.showCandidates(cands);
    } catch (e) {
      UI.toast('生成失败: ' + e.message, 'error');
    }
  },
  showCandidates: function (cands) {
    var self = this;
    var html = cands.map(function (c, i) {
      return '<div style="border:1px solid var(--border2);border-radius:10px;padding:10px 12px;margin-bottom:10px;background:var(--panel2)">' +
        '<div style="font-weight:600;margin-bottom:6px;color:#f59e0b;font-size:12px">候选 ' + (i + 1) +
        '<span style="color:var(--muted);font-weight:400;margin-left:6px">' + (i === 0 ? '平稳推进' : i === 1 ? '冲突/反转' : '新线索/伏笔') + '</span></div>' +
        '<div style="font-size:12.5px;line-height:1.7;white-space:pre-wrap;max-height:200px;overflow:auto">' + esc(c) + '</div>' +
        '<div style="margin-top:8px"><button class="btn btn-primary btn-sm" onclick="Composer.useCandidate(' + i + ')">✅ 采用此候选</button>' +
        '<button class="btn btn-ghost btn-sm" style="margin-left:6px" onclick="Composer.copyCandidate(' + i + ')">📋 复制</button></div></div>';
    }).join('');
    UI.modal({
      title: '🎲 多候选续写（点击「采用」插入光标处）',
      body: html,
      actions: [{ id: 'ok', label: '关闭', cls: 'btn-ghost' }]
    });
  },
  useCandidate: function (i) {
    var cands = this._cands || [];
    if (!cands[i]) return;
    Editor.insertAtCursor(cands[i]);
    UI.toast('已插入候选 ' + (i + 1), 'success');
    UI.closeModal();
  },
  copyCandidate: function (i) {
    var cands = this._cands || [];
    if (!cands[i]) return;
    navigator.clipboard.writeText(cands[i]).then(function () { UI.toast('已复制', 'success'); }).catch(function () {});
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
  onNewChapterChange: function () {
    var cb = document.getElementById('newChapterToggle');
    Store.state.composer.newChapterWrite = cb ? cb.checked : false;
    Store.savePrefs();
  },
  /* 🧠 思考面板：展开/收起每个 agent 的深度思考开关 */
  toggleThinkingPanel: function (ev) {
    if (ev) ev.stopPropagation();
    var p = document.getElementById('thinkingPanel');
    if (!p) return;
    p.style.display = p.style.display === 'none' ? 'flex' : 'none';
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
  /* ===== 专业模式：详细大纲 + AI 辅助设定 ===== */


  /* 按项目题材匹配的专业模式示例（动态 placeholder，覆盖多种类型；未匹配用中性示例） */
  GENRE_EXAMPLES: [
    { match: /校园|青春|高考|学生|高中|大学/, ph: {
      proBookName: '例如：盛夏的回声', proHero: '例如：林夜，高二学生，性格隐忍腹黑，观察力惊人',
      proWorld: '例如：市重点高中，明暗两派学生团体，日常与竞赛交织的校园氛围',
      proPlot: '例如：\n开局：转学第一天，主角与同桌的意外交集\n发展：社团竞选与月考压力下暗流涌动\n高潮：误会解开，友谊与成绩面临双重考验\n结局：各自成长，迎来新的学期' } },
    { match: /玄幻|修仙|仙侠|奇幻|修真|武侠/, ph: {
      proBookName: '例如：风起西陵', proHero: '例如：林夜，外门弟子，实为上古灵脉传人，性格隐忍腹黑',
      proWorld: '例如：灵气复苏的九州大陆，五大宗门暗中博弈', proPower: '例如：觉醒者九阶：E→D→C→B→A→S→SS→SSS→神阶',
      proPlot: '例如：\n开局：主角意外觉醒，卷入宗门斗争\n发展：逐步揭开家族秘辛\n高潮：决战幕后黑手，守护宗门\n结局：完成传承，开启新篇章' } },
    { match: /都市|异能|商战|职场|官场/, ph: {
      proBookName: '例如：城市之光', proHero: '例如：林夜，普通职员，实为低调的业界天才，性格隐忍腹黑',
      proWorld: '例如：现代都市，五大企业暗中博弈，表面光鲜暗流涌动',
      proPlot: '例如：\n开局：主角意外卷入商业风波\n发展：步步为营，揭开对手布局\n高潮：商战对决\n结局：尘埃落定，开启新篇章' } },
    { match: /悬疑|推理|探案|刑侦|惊悚|灵异/, ph: {
      proBookName: '例如：雾中谜案', proHero: '例如：林夜，刑警，观察力惊人，性格孤僻',
      proWorld: '例如：现代都市，连环案件与隐藏多年的真相',
      proPlot: '例如：\n开局：一桩离奇案件打破平静\n发展：线索交织，嫌疑人逐个浮现\n高潮：真相反转\n结局：案件告破，留下新的悬念' } },
    { match: /科幻|末世|星际|机甲|赛博/, ph: {
      proBookName: '例如：星海拾遗', proHero: '例如：林夜，废土幸存者，觉醒特殊能力',
      proWorld: '例如：末世废土，幸存者据点与变异危机并存',
      proPlot: '例如：\n开局：末世降临，主角挣扎求生\n发展：发现变异源头\n高潮：直面幕后组织\n结局：找到希望' } },
    { match: /古代|历史|宫斗|宅斗|种田|穿越/, ph: {
      proBookName: '例如：盛世长歌', proHero: '例如：林夜，世家庶子，聪慧隐忍',
      proWorld: '例如：架空古代，朝堂与世家盘根错节',
      proPlot: '例如：\n开局：家族变故，主角被迫入局\n发展：步步为营，卷入朝堂之争\n高潮：权谋对决\n结局：拨云见日' } }
  ],
  /* 根据项目题材设置专业模式字段的动态示例（空字段才生效） */
  _applyGenrePlaceholder: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    var t = (p.type || '') + ' ' + (p.name || '');
    var ex = null;
    for (var i = 0; i < this.GENRE_EXAMPLES.length; i++) {
      if (this.GENRE_EXAMPLES[i].match.test(t)) { ex = this.GENRE_EXAMPLES[i].ph; break; }
    }
    if (!ex) return; // 未匹配：保持通用中性示例
    Object.keys(ex).forEach(function (id) {
      var el = document.getElementById(id);
      if (el && !el.value.trim()) el.placeholder = ex[id];
    });
  },
  /* 打开专业模式时，自动用当前项目已有信息填充空字段（书名/题材/主角/世界观/力量体系） */
  fillProModeFromProject: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    var setIf = function (id, val) {
      if (!val) return;
      var el = document.getElementById(id);
      if (el && !el.value.trim()) el.value = val;
    };
    setIf('proBookName', p.name);
    setIf('proGenre', p.type || '');
    // 主角：人物卡第一条
    var chars = Store.state.characters || [];
    if (chars.length) {
      var hero = chars[0];
      var heroDesc = (hero.description || '').split('\n').filter(function (s) { return s.trim(); })[0] || '';
      setIf('proHero', hero.name + (heroDesc ? '：' + heroDesc : ''));
    }
    // 世界观：全部条目合并
    var ws = Store.state.worldSettings || [];
    if (ws.length) {
      setIf('proWorld', ws.map(function (w) { return w.title + '：' + w.content; }).join('\n'));
    }
    // 力量体系：从世界观中筛含力量/等级/修炼等关键词的条目
    var powerItems = ws.filter(function (w) {
      return /力量|等级|修为|修炼|境界|体系|能力|实力/.test((w.title || '') + (w.content || ''));
    });
    if (powerItems.length) {
      setIf('proPower', powerItems.map(function (w) { return w.title + '：' + w.content; }).join('\n'));
    }
    // 动态示例（按题材）
    this._applyGenrePlaceholder();
  },
  /* 打开专业模式时收起「更多」菜单（两者互斥，避免悬浮层互相遮挡） */
  toggleProMode: function (ev) {
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var panel = document.getElementById('proModePanel');
    var btn = document.getElementById('proModeBtn');
    if (!panel) return;
    var show = panel.style.display === 'none';
    panel.style.display = show ? 'flex' : 'none';
    if (show) {
      // 打开专业模式时收起「更多」菜单（两者互斥，避免悬浮层互相遮挡）
      var mm = document.getElementById('moreMenu');
      if (mm && mm.style.display !== 'none') mm.style.display = 'none';
      // 项目设定自动填入空字段
      this.fillProModeFromProject();
      this._positionProPanel();
      UI.toast('⚡ 专业模式已展开：项目设定已自动填入，填写「本章续写大纲」即可生成', 'success');
    } else {
      UI.toast('专业模式已关闭', '');
    }
    if (btn) btn.classList.toggle('on', show);
    // 同步大纲内容到 genOutline（生成时读取）
    if (show) this.syncProOutlineToGen();
    Store.set('proModeOpen', show);
  },
  /* 关闭专业模式面板（面板右上角 ✕ 关闭按钮） */
  closeProMode: function (ev) {
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var panel = document.getElementById('proModePanel');
    var btn = document.getElementById('proModeBtn');
    if (panel) panel.style.display = 'none';
    if (btn) btn.classList.remove('on');
    Store.set('proModeOpen', false);
    UI.toast('专业模式已关闭，可继续创作', '');
  },
  onProOutline: function () {
    // 聚合专业模式所有字段为结构化大纲（写入 genOutline 供生成使用）
    var fields = {
      bookName: document.getElementById('proBookName'),
      genre: document.getElementById('proGenre'),
      selling: document.getElementById('proSelling'),
      hero: document.getElementById('proHero'),
      world: document.getElementById('proWorld'),
      power: document.getElementById('proPower'),
      plot: document.getElementById('proPlot'),
      volumes: document.getElementById('proVolumes')
    };
    var parts = [];
    var book = (fields.bookName ? fields.bookName.value : '').trim();
    var genre = (fields.genre ? fields.genre.value : '').trim();
    var selling = (fields.selling ? fields.selling.value : '').trim();
    var hero = (fields.hero ? fields.hero.value : '').trim();
    var world = (fields.world ? fields.world.value : '').trim();
    var power = (fields.power ? fields.power.value : '').trim();
    var plot = (fields.plot ? fields.plot.value : '').trim();
    var volumes = (fields.volumes ? fields.volumes.value : '').trim();
    if (book) parts.push('书名：' + book);
    if (genre) parts.push('题材：' + genre);
    if (selling) parts.push('核心卖点：' + selling);
    if (hero) parts.push('主角设定：' + hero);
    if (world) parts.push('世界观：' + world);
    if (power) parts.push('力量体系：' + power);
    if (plot) parts.push('主线剧情：\n' + plot);
    if (volumes) parts.push('分卷规划：\n' + volumes);
    var outline = parts.join('\n\n');
    Store.state.composer.outline = outline;
    var gen = document.getElementById('genOutline');
    if (gen && gen.value !== outline) gen.value = outline;
    this.onOutlineChange();
  },
  syncProOutlineToGen: function () {
    var cur = Store.state.composer.outline || '';
    if (!cur) return;
    // 尝试把已有大纲文本解析回字段（简化：按“标签：”匹配）
    var lines = cur.split('\n');
    var curText = '';
    var mapping = { '书名': 'proBookName', '题材': 'proGenre', '核心卖点': 'proSelling', '主角设定': 'proHero', '世界观': 'proWorld', '力量体系': 'proPower' };
    var lastField = null;
    lines.forEach(function (l) {
      var m = l.match(/^([^：:]{2,6})[：:]\s*(.*)$/);
      if (m && mapping[m[1]]) {
        var el = document.getElementById(mapping[m[1]]);
        if (el && !el.value) el.value = m[2].trim();
        lastField = m[1];
      } else if (lastField === '主线剧情' || lastField === '分卷规划') {
        var ta = document.getElementById(lastField === '主线剧情' ? 'proPlot' : 'proVolumes');
        if (ta && !ta.value) ta.value += (ta.value ? '\n' : '') + l;
      }
    });
    // 主线/分卷单独尝试匹配
    var pm = cur.match(/主线剧情[：:]\s*\n?([\s\S]*?)(?=\n\s*分卷规划|$)/);
    if (pm) { var elp = document.getElementById('proPlot'); if (elp && !elp.value) elp.value = pm[1].trim(); }
    var vm = cur.match(/分卷规划[：:]\s*\n?([\s\S]*?)$/);
    if (vm) { var elv = document.getElementById('proVolumes'); if (elv && !elv.value) elv.value = vm[1].trim(); }
  },
  aiOutline: async function () {
    // 用聚合后的结构化大纲作为输入（含所有字段）
    var outline = Store.state.composer.outline || '';
    var demand = outline || (document.getElementById('instructionInput') || {}).value || '';
    if (!demand.trim()) { UI.toast('请先填写题材/灵感，或使用上面的需求输入框', 'warn'); return; }
    var pid = Store.state.currentProject;
    if (!pid) { UI.toast('请先选择项目', 'warn'); return; }
    var btn = event && event.currentTarget;
    if (btn) { btn.disabled = true; btn.textContent = '🤖 生成中…'; }
    try {
      var r = await API.post('/api/tools/execute', { tool: 'outline', content: demand.slice(0, 6000) });
      var result = (r && r.result) || '';
      if (!result.trim()) { UI.toast('AI 生成失败，请重试', 'error'); return; }
      // 结果写入专业模式弹窗 + 主线剧情字段
      this._runToolResult('🤖 AI 生成的大纲', result);
      UI.toast('✅ 大纲已生成，可继续编辑或直接生成正文', 'success');
    } catch (e) {
      UI.toast('大纲生成失败：' + e.message, 'error');
    } finally {
      if (btn) { btn.disabled = false; btn.textContent = '🤖 AI 自动生成大纲'; }
    }
  },
  restoreProMode: function () {
    // 专业模式默认收起，不再自动展开遮挡编辑器（用户需要时点 ⚡ 手动展开）
    var panel = document.getElementById('proModePanel');
    var btn = document.getElementById('proModeBtn');
    if (panel) panel.style.display = 'none';
    if (btn) btn.classList.remove('on');
    Store.set('proModeOpen', false);
  },
  /* 将专业模式面板定位为固定悬浮层：锚定在生成栏正上方，宽度对齐，不遮挡工具栏，超高内部滚动 */
  _positionProPanel: function () {
    var self = this;
    var run = function () {
      var panel = document.getElementById('proModePanel');
      var ci = document.getElementById('composerInner');
      if (!panel || !ci || panel.style.display === 'none') return;
      var r = ci.getBoundingClientRect();
      panel.style.position = 'fixed';
      panel.style.left = Math.round(r.left) + 'px';
      panel.style.width = Math.round(r.width) + 'px';
      panel.style.bottom = (window.innerHeight - Math.round(r.top) + 10) + 'px';
      // 可用高度 = 生成栏顶 到 编辑器工具栏底（避免悬浮层盖住工具栏/更多菜单）
      var tb = document.querySelector('.editor-toolbar');
      var topBound = tb ? tb.getBoundingClientRect().bottom : 120;
      var avail = Math.round(r.top) - Math.round(topBound) - 16;
      if (avail < 180) avail = 180;
      panel.style.maxHeight = Math.max(180, Math.min(avail, Math.round(window.innerHeight * 0.8))) + 'px';
      panel.scrollTop = 0;
    };
    run();
    // 页面布局可能仍在沉降（章节异步加载等），延迟再校准一次
    clearTimeout(this._proPosTimer);
    this._proPosTimer = setTimeout(run, 120);
  },
  /* 通用 AI 工具执行：展示结果弹窗 */
  _runTool: async function (tool, content, btn, label) {
    if (!content || !content.trim()) { UI.toast('请先填写内容', 'warn'); return; }
    if (btn) { btn.disabled = true; btn.textContent = '⏳ 生成中…'; }
    try {
      var r = await API.post('/api/tools/execute', { tool: tool, content: content.slice(0, 8000) });
      var result = (r && r.result) || '';
      if (!result.trim()) { UI.toast('AI 生成失败，请重试', 'error'); return; }
      // 结果弹窗，可复制
      this._runToolResult(label, result);
      return result;
    } catch (e) {
      UI.toast(label + '失败：' + e.message, 'error');
    } finally {
      if (btn) { btn.disabled = false; btn.textContent = label; }
    }
  },
  /* 通用 AI 结果弹窗（可编辑+复制） */
  _runToolResult: function (label, result) {
    var idn = 't_' + Date.now();
    UI.modal({
      title: label,
      width: '560px',
      body: '<div class="form-group"><textarea id="' + idn + '" rows="14" style="width:100%;font-size:12px;line-height:1.7;background:var(--panel2);border:1px solid var(--border);border-radius:8px;padding:10px;color:var(--text);font-family:var(--font)">' + esc(result) + '</textarea></div>' +
            '<div style="font-size:10.5px;color:var(--muted);margin-top:4px">💡 可编辑后复制到对应模块，或作为灵感参考</div>',
      actions: [
        { id: 'cancel', label: '关闭' },
        { id: 'copy', label: '📋 复制结果', cls: 'btn-primary', onClick: function (m, ov) {
          var ta = document.getElementById(idn);
          if (ta) { ta.select(); document.execCommand('copy'); UI.toast('已复制', 'success'); }
        } }
      ]
    });
  },
  aiWorldbuild: function () {
    var demand = Store.state.composer.outline || (document.getElementById('instructionInput') || {}).value || '';
    this._runTool('worldbuild', demand || '一个原创架空世界（请提供题材）', null, '🌍 AI 生成世界观');
  },
  aiNames: function () {
    var demand = Store.state.composer.outline || '';
    if (!demand.trim()) demand = '现代都市言情，男主：清冷克制型霸总；女主：独立飒爽型设计师';
    this._runTool('namegen', demand, null, '👤 AI 生成角色名');
  },
  /* 专业模式逐字段 AI 提示：严格限定只生成该字段 */
  _fieldMeta: {
    bookname: { el: 'proBookName', name: '书名', multi: false },
    genre: { el: 'proGenre', name: '题材', multi: false },
    selling: { el: 'proSelling', name: '核心卖点', multi: false },
    hero: { el: 'proHero', name: '主角设定', multi: false },
    world: { el: 'proWorld', name: '世界观/环境设定', multi: false },
    power: { el: 'proPower', name: '力量/等级体系', multi: false },
    plot: { el: 'proPlot', name: '主线剧情概述', multi: true },
    volumes: { el: 'proVolumes', name: '分卷规划', multi: true }
  },
  aiField: async function (ev, field) {
    if (ev && ev.stopPropagation) ev.stopPropagation();
    var meta = this._fieldMeta[field];
    if (!meta) { UI.toast('未知字段', 'warn'); return; }
    var btn = ev && ev.currentTarget;
    // 收集上下文：当前已填字段 + 底部需求输入
    var ctx = this._proContext(field);
    if (!ctx.trim()) { UI.toast('请先填写任一设定或底部需求输入框，AI 才能给出建议', 'warn'); return; }
    if (btn) { btn.disabled = true; btn.textContent = '⏳'; }
    try {
      var r = await API.post('/api/tools/execute', {
        tool: 'fieldgen',
        content: ctx.slice(0, 3000),
        params: { instruction: field }
      });
      var result = (r && r.result || '').trim();
      if (!result) { UI.toast('AI 生成失败，请重试', 'error'); return; }
      result = this._cleanFieldResult(field, result);
      if (!result) { UI.toast('AI 返回内容为空，请重试', 'error'); return; }
      var el = document.getElementById(meta.el);
      if (el) {
        el.value = result;
        el.dispatchEvent(new Event('input'));
        this.onProOutline();
      }
      UI.toast('✨ ' + meta.name + ' 已生成' + (meta.multi ? '' : '，可继续编辑'), 'success');
    } catch (e) {
      UI.toast('AI 生成失败：' + e.message, 'error');
    } finally {
      if (btn) { btn.disabled = false; btn.textContent = '✨AI'; }
    }
  },
  /* 聚合当前已填字段作为 AI 上下文（排除正在生成的字段本身） */
  _proContext: function (excludeField) {
    var parts = [];
    var ins = (document.getElementById('instructionInput') || {}).value || '';
    var self = this;
    Object.keys(this._fieldMeta).forEach(function (k) {
      if (k === excludeField) return;
      var m = self._fieldMeta[k];
      var el = document.getElementById(m.el);
      var v = el ? el.value.trim() : '';
      if (v) parts.push(m.name + '：' + v);
    });
    if (ins.trim()) parts.push('创作需求：' + ins.trim());
    if (!parts.length) {
      // 完全空白时给 AI 一个通用起点
      return '';
    }
    return parts.join('\n');
  },
  /* 清洗 AI 返回：去掉标签前缀/引号/序号/候选标记，只留字段内容 */
  _cleanFieldResult: function (field, result) {
    var meta = this._fieldMeta[field];
    var name = meta ? meta.name : field;
    var lines = result.split('\n').map(function (l) { return l.trim(); }).filter(Boolean);
    var cleaned = [];
    for (var i = 0; i < lines.length; i++) {
      var l = lines[i];
      // 去掉 “书名：” “【书名】” “书名：xxx” 等标签前缀
      l = l.replace(new RegExp('^【?' + name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '】?[：:]\\s*'), '');
      l = l.replace(/^[【\[]/, '').replace(/[】\]]$/, '');
      l = l.replace(/^\*{1,2}/, '').replace(/\*{1,2}$/, '');
      l = l.replace(/^[-•·]\s*/, '');
      l = l.replace(/^\d+[.、)]\s*/, '');
      // 去掉候选标记行（如 “方案A：”“1. 书名：”）
      if (/^(方案|候选|选项)[A-D0-9一二三四]?[：:]/.test(l)) continue;
      if (l === name) continue;
      if (l) cleaned.push(l);
    }
    var out = cleaned.join('\n').trim();
    out = out.replace(/^[「『“"']+/, '').replace(/[」』”"']+$/, '');
    // 单行字段只取第一行
    if (meta && !meta.multi) out = out.split('\n')[0].trim();
    return out;
  },
  /* 辅助工具：伏笔检查 / 角色互动 / 剧情分支 */
  aiPlotCheck: function () {
    var text = Editor.getText();
    if (!text || !text.trim()) { UI.toast('编辑器内容为空', 'warn'); return; }
    this._runTool('plotcheck', text.slice(0, 20000), null, '🔍 伏笔与逻辑检查');
  },
  aiRoleplay: function () {
    var p = Store.state.currentProject;
    var chars = Store.state.characters || [];
    if (!chars.length) { UI.toast('请先创建至少两个人物卡', 'warn'); return; }
    var brief = chars.slice(0, 4).map(function (c, i) { return '角色' + (i + 1) + '：' + (c.name || '未命名') + '\n' + (c.personality ? '性格：' + c.personality : '') + (c.appearance ? '外貌：' + c.appearance : ''); }).join('\n\n');
    var scene = '场景：两人在一场重要事件后相遇，请模拟他们的对话互动';
    this._runTool('roleplay', brief + '\n\n' + scene, null, '🎭 角色互动模拟');
  },
  aiBranch: function () {
    var demand = (document.getElementById('instructionInput') || {}).value || '';
    var ch = Store.state.currentChapter;
    var cur = ch ? (ch.content || '').slice(-800) : Editor.getText().slice(-800);
    var node = demand || cur || '主角刚刚发现被最信任的人背叛';
    this._runTool('branch', node, null, '🎲 剧情分支推演');
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
  /* 🤖 AI 预估本章字数：根据大纲+需求调预检接口，返回推荐区间并设为目标字数 */
  onEstimate: function () {
    var outline = (document.getElementById('genOutline') || {}).value || '';
    var demand = (document.getElementById('instructionInput') || {}).value || '';
    var tw = Store.state.composer.targetWord || 1000;
    var body = {
      user_demand: (demand || outline || '本章').slice(0, 2000),
      target_word: tw, world_setting: '', character_setting: '', history_content: '',
      project_id: (Store.state.currentProject || {}).id || ''
    };
    UI.toast('🤖 AI 预估字数中…', 'info');
    var self = this;
    API.post('/api/precheck', body).then(function (pre) {
      if (!pre || !pre.recommended_min) { UI.toast('预估失败：模型未返回结果', 'error'); return; }
      var rec = pre.recommended_max || pre.recommended_min;
      if (rec) {
        Store.state.composer.targetWord = rec;
        var twm = document.getElementById('targetWordMini');
        if (twm) twm.value = rec;
        Store.savePrefs();
        UI.toast('🤖 预估本章约 ' + rec + ' 字（基于本项目近几章平均 + 需求调整），已自动设为目标，可直接改 🎯 框', 'success', 5000);
      } else {
        UI.toast('预估失败：模型未返回结果', 'error');
      }
    }).catch(function (e) { UI.toast('预估失败：' + (e.message || e), 'error'); });
  },

  onOutlineChange: function () {
    var el = document.getElementById('genOutline');
    Store.state.composer.outline = el ? el.value.trim() : '';
    Store.savePrefs();
    // 自动保存大纲到后端（2秒防抖）
    if (this._outlineTimer) clearTimeout(this._outlineTimer);
    var self = this;
    this._outlineTimer = setTimeout(function () {
      var p = Store.state.currentProject;      if (p && Store.state.composer.outline !== (p.outline || '')) {
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
