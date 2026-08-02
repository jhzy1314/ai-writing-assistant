/* ============ resources.js：人物卡 / 世界观 / 素材 管理 + 上下文拼接 ============ */
/* 人物卡字段（姓名、外貌、性格、背景、行为底线、备注）打包进后端 description 字段
   世界观字段（标题、世界规则、势力分布、地理设定、力量体系）打包进后端 content 字段 */

var Context = {
  worldSetting: function () {
    return Store.state.worldSettings
      .filter(function (w) { return Store.state.selection.worldSettings.has(w.id); })
      .map(function (w) { return '《' + w.title + '》\n' + w.content; })
      .join('\n\n');
  },
  characters: function () {
    return Store.state.characters
      .filter(function (c) { return Store.state.selection.characters.has(c.id); })
      .map(function (c) { return '【' + c.name + '】\n' + c.description; })
      .join('\n\n');
  },
  materials: function () {
    return Store.state.materials
      .filter(function (m) { return Store.state.selection.materials.has(m.id); })
      .map(function (m) { return '《' + m.name + '》\n' + m.content; })
      .join('\n\n');
  }
};

var ResourceUI = {
  /* ---- AI 一键总结：从编辑器内容提取人物卡 + 世界观 ---- */
  /* ---- 自动导入：AI 读完全部章节后自动提取并保存人物卡/世界观（无需手动确认） ---- */
  autoImportSettings: async function () {
    if (ResourceUI._autoImportRunning) { UI.toast('自动导入正在进行中，请稍候…', 'warn'); return; }
    ResourceUI._autoImportRunning = true;
    try {
      await ResourceUI._autoImportImpl();
    } finally {
      ResourceUI._autoImportRunning = false;
    }
  },
  _autoImportImpl: async function () {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先选择项目', 'warn'); return; }
    var chapters = Store.state.chapters || [];
    if (!chapters.length) { UI.toast('当前项目还没有章节，先写几章再自动导入', 'warn'); return; }
    // 进度弹窗（不阻塞，纯展示）
    var pid = 'ai_' + uid();
    var setStage = function (stage, pct, flowing) {
      var el = document.getElementById(pid + '_stage');
      var bar = document.getElementById(pid + '_bar');
      if (el) el.textContent = stage;
      if (bar) {
        bar.style.width = (pct || 0) + '%';
        // 长耗时阶段（AI 分析）加流动条纹，视觉上“在动”
        if (flowing) bar.classList.add('v2-progress-flow');
        else bar.classList.remove('v2-progress-flow');
      }
    };
    var closeProgress = function () {
      document.querySelectorAll('#modalRoot .modal-overlay').forEach(function (o) { o.remove(); });
    };
    UI.modal({
      id: pid,
      title: '🤖 AI 自动导入设定',
      sub: '正在通读本项目全部章节，自动提取人物卡与世界观',
      body: '<div style="padding:4px 0 12px">' +
        '<div id="' + pid + '_stage" style="font-size:13px;color:var(--text2);margin-bottom:8px">准备中…</div>' +
        '<div style="height:8px;background:var(--panel3);border-radius:4px;overflow:hidden"><div id="' + pid + '_bar" style="height:100%;width:2%;background:linear-gradient(90deg,var(--accent),var(--accent2));border-radius:4px;transition:width .3s ease; /* impeccable-disable-line layout-transition: 进度条状态动画 */"></div></div>' +
        '<div id="' + pid + '_count" style="font-size:11px;color:var(--muted);margin-top:8px"></div>' +
        '</div>',
      actions: []
    });
    try {
      // 阶段1：分批读取章节——每批约 1.5 万字，全部分批处理完后合并，10 万字长文也能完整覆盖不丢角色
      setStage('📚 读取章节中…（' + chapters.length + ' 章）', 5, false);
      // 优先使用快速总结模型（实测 deepseek-v4-flash 最快；glm 系推理慢。按用户要求：哪个快用哪个）
      var fastModel = null;
      var models = Store.state.models || [];
      var prefer = ['deepseek-v4-flash', 'deepseek-v4-pro', 'glm-5.2', 'glm-5-turbo', 'glm-4.5-air'];
      for (var pi = 0; pi < prefer.length; pi++) {
        var m = models.find(function (x) { return x.name === prefer[pi] && x.status === 'active'; });
        if (m) { fastModel = m.name; break; }
      }
      // 按章节分批：每批累计到 1.5 万字就封批，保证每批在模型舒适长度内且不丢任何章节
      var batches = [];
      var cur = '';
      var curChapters = 0;
      chapters.forEach(function (c) {
        var block = '【' + (c.title || '未命名') + '】\n' + (c.content || '');
        if (cur.length + block.length > 15000 && curChapters > 0) {
          batches.push(cur);
          cur = block;
          curChapters = 1;
        } else {
          cur = cur ? cur + '\n\n' + block : block;
          curChapters++;
        }
      });
      if (cur) batches.push(cur);
      if (!batches.length) { UI.toast('当前项目没有可读章节', 'warn'); closeProgress(); return; }
      // 逐批调用 AI 总结，合并所有批次结果
      var allParsed = { characters: [], worlds: [] };
      for (var bi = 0; bi < batches.length; bi++) {
        var bText = batches[bi];
        setStage('🧠 AI 分析第 ' + (bi + 1) + '/' + batches.length + ' 批…（' + bText.length + ' 字）', 15 + Math.round(bi / batches.length * 55), true);
        var r = await API.post('/api/tools/execute', { tool: 'summarize', content: bText, model: fastModel || '' });
        var batchParsed = ResourceUI.parseSummaryResult(r.result || '');
        allParsed.characters = allParsed.characters.concat(batchParsed.characters || []);
        allParsed.worlds = allParsed.worlds.concat(batchParsed.worlds || []);
      }
      var parsed = allParsed;
      if (!parsed.characters.length && !parsed.worlds.length) {
        setStage('⚠️ AI 未从正文识别到可导入的设定', 100, false);
        setTimeout(closeProgress, 1500);
        UI.toast('AI 未从正文识别到可导入的设定', '');
        return;
      }
      // 阶段2：保存
      setStage('💾 保存中…', 55, false);
      // 模糊去重：规范化名字（去空白/引号/括号/重复字）后比对，避免"天一"vs"天一天一"、"我"vs"叙述者我"等重复
      var norm = function (s) {
        return String(s || '')
          .replace(/[\s"'"'“”‘’（）()《》【】\[\]：:，,。.！!？?]/g, '')
          .replace(/(.+)\1$/, '$1'); // 去掉尾部重复段（天一天一 -> 天一）
      };
      var existingCharsNorm = (Store.state.characters || []).map(function (c) { return norm(c.name); });
      var existingWorldsNorm = (Store.state.worldSettings || []).map(function (w) { return norm(w.title); });
      var newChars = parsed.characters.filter(function (c) { return existingCharsNorm.indexOf(norm(c.name)) < 0; });
      var newWorlds = parsed.worlds.filter(function (w) { return existingWorldsNorm.indexOf(norm(w.title)) < 0; });
      // 新提取的彼此之间也去重（保留第一个）
      var seenNew = {};
      newChars = newChars.filter(function (c) {
        var k = norm(c.name);
        if (!k || seenNew[k]) return false;
        seenNew[k] = true;
        return true;
      });
      var saved = 0, failed = 0, skipped = parsed.characters.length + parsed.worlds.length - newChars.length - newWorlds.length;
      var total = newChars.length + newWorlds.length;
      for (var i = 0; i < newChars.length; i++) {
        try { await ResourceUI.saveCharacter(null, newChars[i].name, newChars[i]); saved++; }
        catch (e) { failed++; }
        var countEl = document.getElementById(pid + '_count');
        if (countEl) countEl.textContent = '人物 ' + (i + 1) + '/' + newChars.length;
        setStage('💾 保存人物卡 ' + (i + 1) + '/' + newChars.length, 55 + Math.round((i + 1) / (total || 1) * 35), false);
      }
      for (var j = 0; j < newWorlds.length; j++) {
        try { await ResourceUI.saveWorld(null, newWorlds[j].title, newWorlds[j]); saved++; }
        catch (e) { failed++; }
        setStage('💾 保存世界观 ' + (j + 1) + '/' + newWorlds.length, 55 + Math.round((newChars.length + j + 1) / (total || 1) * 35), false);
      }
      setStage('✅ 完成', 100, false);
      setTimeout(closeProgress, 1200);
      Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
      if (saved > 0) UI.toast('✅ 已自动导入 ' + saved + ' 条设定（新增 ' + newChars.length + ' 人物 / ' + newWorlds.length + ' 世界观' + (skipped > 0 ? '，跳过重复 ' + skipped + ' 条' : '') + '）' + (failed ? '，' + failed + ' 条保存失败' : ''), 'success');
      else if (skipped > 0) UI.toast('提取到 ' + (parsed.characters.length + parsed.worlds.length) + ' 条设定，但都已存在（自动跳过重复 ' + skipped + ' 条）', 'info');
      else UI.toast('AI 未从正文识别到可导入的设定', '');
    } catch (e) {
      setStage('❌ 失败：' + e.message, 100, false);
      setTimeout(closeProgress, 2000);
      UI.toast('自动导入失败：' + e.message, 'error');
    }
  },

  summarizeSettings: async function () {
    var text = Editor.getText();
    if (!text || !text.trim()) { UI.toast('编辑器内容为空，请先撰写部分章节再总结', 'warn'); return; }
    var pid = Store.state.currentProject;
    if (!pid) { UI.toast('请先选择项目', 'warn'); return; }
    UI.toast('正在分析已写内容，提取人物和世界观…', '');
    RightPanel.switch('tools');
    var el = document.getElementById('toolOutput');
    el.innerHTML = '<div class="res-check-empty"><span class="loading">AI 正在从正文中提取人物卡和世界观设定</span></div>';
    try {
      var r = await API.post('/api/tools/execute', {
        tool: 'summarize',
        content: text.slice(0, 20000)
      });
      var result = r.result || '';
      var parsed = ResourceUI.parseSummaryResult(result);
      if (!parsed.characters.length && !parsed.worlds.length) {
        el.innerHTML = '<div class="res-check-empty">AI 未从正文中识别到可提取的人物或世界观信息</div>';
        UI.toast('未找到可提取的设定', '');
        return;
      }
      ResourceUI.showSummaryResultModal(parsed);
    } catch (e) {
      el.innerHTML = '<div class="res-check-empty" style="color:var(--danger)">AI 总结失败：' + esc(e.message) + '</div>';
      UI.toast('AI 总结失败：' + e.message, 'error');
    }
  },

  parseSummaryResult: function (text) {
    var chars = [], worlds = [];
    // 按【人物卡列表】和【世界观设定】分割
    var charSection = '', worldSection = '';
    var charMatch = text.match(/【人物卡列表】\s*([\s\S]*?)(?=【世界观设定】|$)/);
    var worldMatch = text.match(/【世界观设定】\s*([\s\S]*)/);
    if (charMatch) charSection = charMatch[1].trim();
    if (worldMatch) worldSection = worldMatch[1].trim();

    // 解析人物卡（按 --- 分隔）
    var charBlocks = charSection.split(/---+/);
    charBlocks.forEach(function (block) {
      block = block.trim();
      if (!block) return;
      var f = { name: '', gender: '', tags: '', appearance: '', personality: '', background: '', bottomline: '', relations: '', notes: '' };
      var map = { '姓名': 'name', '性别': 'gender', '标签': 'tags', '外貌': 'appearance', '性格': 'personality', '背景': 'background', '行为底线': 'bottomline', '人际关系': 'relations', '备注': 'notes' };
      block.split('\n').forEach(function (line) {
        var m = line.match(/^(姓名|性别|标签|外貌|性格|背景|行为底线|人际关系|备注)[：:]\s*(.*)/);
        if (m && map[m[1]]) f[map[m[1]]] = m[2].trim();
      });
      if (f.name && f.name !== '未知') chars.push(f);
    });

    // 解析世界观（按 --- 分隔）
    var worldBlocks = worldSection.split(/---+/);
    worldBlocks.forEach(function (block) {
      block = block.trim();
      if (!block) return;
      var f = { title: '', era: '', tags: '', rules: '', forces: '', geography: '', powers: '' };
      var map = { '标题': 'title', '时代背景': 'era', '标签': 'tags', '世界规则': 'rules', '势力分布': 'forces', '地理设定': 'geography', '力量体系': 'powers' };
      block.split('\n').forEach(function (line) {
        var m = line.match(/^(标题|时代背景|标签|世界规则|势力分布|地理设定|力量体系)[：:]\s*(.*)/);
        if (m && map[m[1]]) f[map[m[1]]] = m[2].trim();
      });
      if (f.title && f.title !== '未知') worlds.push(f);
    });

    // 回退：如果段落分割失败，尝试按单行匹配
    if (!chars.length) {
      var nameRe = /姓名[：:]\s*(.+)/g, mn;
      while ((mn = nameRe.exec(charSection)) !== null) {
        if (mn[1].trim() && mn[1].trim() !== '未知') chars.push({ name: mn[1].trim() });
      }
    }
    return { characters: chars, worlds: worlds };
  },

  showSummaryResultModal: function (parsed) {
    var self = this;
    var checkedChars = {};
    var checkedWorlds = {};
    parsed.characters.forEach(function (c) { checkedChars[c.name] = true; });
    parsed.worlds.forEach(function (w) { checkedWorlds[w.title] = true; });

    var charsHTML = '';
    var worldsHTML = '';

    if (parsed.characters.length) {
      charsHTML = '<div style="margin-bottom:14px"><div class="ghead">人物卡（' + parsed.characters.length + ' 人）</div>';
      parsed.characters.forEach(function (c, i) {
        var desc = [c.gender, c.tags, c.personality].filter(function (x) { return x && x !== '未知'; }).join(' · ');
        charsHTML += '<div class="summary-check-item" onclick="event.currentTarget.classList.toggle(\'unchecked\');var cb=event.currentTarget.querySelector(\'input\');cb.checked=!cb.checked;if(cb.checked){checkedChars[\'' + esc(c.name) + '\']=true}else{checkedChars[\'' + esc(c.name) + '\']=false}">' +
          '<input type="checkbox" checked onchange="if(this.checked){checkedChars[\'' + esc(c.name) + '\']=true}else{checkedChars[\'' + esc(c.name) + '\']=false}" onclick="event.stopPropagation()">' +
          '<span class="n">' + esc(c.name) + '</span><span class="d">' + esc(desc) + '</span></div>';
      });
      charsHTML += '</div>';
    }

    if (parsed.worlds.length) {
      worldsHTML = '<div style="margin-bottom:8px"><div class="ghead">世界观（' + parsed.worlds.length + ' 项）</div>';
      parsed.worlds.forEach(function (w, i) {
        var desc = [w.era, w.tags, w.rules].filter(function (x) { return x && x !== '未知'; }).map(function (s) { return s.slice(0, 40); }).join(' · ');
        worldsHTML += '<div class="summary-check-item" onclick="event.currentTarget.classList.toggle(\'unchecked\');var cb=event.currentTarget.querySelector(\'input\');cb.checked=!cb.checked;if(cb.checked){checkedWorlds[\'' + esc(w.title) + '\']=true}else{checkedWorlds[\'' + esc(w.title) + '\']=false}"">' +
          '<input type="checkbox" checked onchange="if(this.checked){checkedWorlds[\'' + esc(w.title) + '\']=true}else{checkedWorlds[\'' + esc(w.title) + '\']=false}" onclick="event.stopPropagation()">' +
          '<span class="n">' + esc(w.title) + '</span><span class="d">' + esc(desc) + '</span></div>';
      });
      worldsHTML += '</div>';
    }

    UI.modal({
      title: '🤖 AI 自动总结结果',
      sub: '已从正文中提取 ' + parsed.characters.length + ' 个人物和 ' + parsed.worlds.length + ' 个世界观。可勾选需要的条目保存。',
      body: charsHTML + worldsHTML + '<div style="font-size:10px;color:var(--faint);margin-top:4px">提示：已勾选的条目将自动创建。可点击后编辑修改。</div>',
      wide: '520px',
      actions: [
        { id: 'cancel', label: '放弃全部', cls: 'btn-ghost' },
        { id: 'all-edit', label: '逐个修改', cls: 'btn-ghost', onClick: function () {
          document.querySelectorAll('.modal-overlay').forEach(function (o) { o.remove(); });
          ResourceUI.batchEditSummary(parsed);
        }},
        { id: 'ok', label: '保存全部勾选', cls: 'btn-primary', onClick: async function () {
          document.querySelectorAll('.modal-overlay').forEach(function (o) { o.remove(); });
          var savedCount = 0;
          for (var k in checkedChars) {
            if (checkedChars[k]) {
              var c = parsed.characters.find(function (x) { return x.name === k; });
              if (c) {
                try { await ResourceUI.saveCharacter(null, c.name, c); savedCount++; } catch (e) {}
              }
            }
          }
          for (var wk in checkedWorlds) {
            if (checkedWorlds[wk]) {
              var w = parsed.worlds.find(function (x) { return x.title === wk; });
              if (w) {
                try { await ResourceUI.saveWorld(null, w.title, w); savedCount++; } catch (e) {}
              }
            }
          }
          Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
          UI.toast('已保存 ' + savedCount + ' 条设定', 'success');
        }}
      ]
    });
  },

  batchEditSummary: async function (parsed) {
    // 逐个弹窗编辑
    var all = [];
    parsed.characters.forEach(function (c) { all.push({ type: 'character', data: c }); });
    parsed.worlds.forEach(function (w) { all.push({ type: 'world', data: w }); });
    if (!all.length) return;
    var idx = 0;
    var self = this;
    function next() {
      if (idx >= all.length) { UI.toast('全部完成', 'success'); Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta(); return; }
      var item = all[idx++];
      if (item.type === 'character') {
        self.editCharacterWithFields(item.data.name, item.data);
      } else {
        self.editWorldWithFields(item.data.title, item.data);
      }
      // 监听 modal 关闭后继续下一个
      var timer = setInterval(function () {
        if (!document.querySelector('.modal-overlay')) { clearInterval(timer); next(); }
      }, 500);
    }
    next();
  },

  /* ---- 人物卡 ---- */
  editCharacter: function (id) {
    var c = id ? Store.state.characters.find(function (x) { return x.id === id; }) : null;
    var f = c ? this.unpackChar(c.description) : { gender: '', tags: '', appearance: '', personality: '', background: '', bottomline: '', relations: '', notes: '' };
    var ids = 'c_' + uid();
    UI.modal({
      title: c ? '编辑人物卡' : '新建人物卡', wide: '560px',
      body: '<div class="form-row" style="display:flex;gap:8px">' +
        '<div class="form-group" style="flex:2"><label>姓名 *</label><input id="' + ids + '_name" value="' + esc(c ? c.name : '') + '"></div>' +
        '<div class="form-group" style="flex:1"><label>性别</label><select id="' + ids + '_gender"><option value=""' + (!f.gender ? ' selected' : '') + '>未设定</option><option value="男"' + (f.gender === '男' ? ' selected' : '') + '>男</option><option value="女"' + (f.gender === '女' ? ' selected' : '') + '>女</option><option value="其他"' + (f.gender === '其他' ? ' selected' : '') + '>其他</option></select></div>' +
        '<div class="form-group" style="flex:1"><label>标签</label><input id="' + ids + '_tags" value="' + esc(f.tags) + '" placeholder="主角,反派,配角"></div>' +
        '</div>' +
        '<div class="form-group"><label>外貌</label><textarea id="' + ids + '_appearance" rows="2">' + esc(f.appearance) + '</textarea></div>' +
        '<div class="form-group"><label>性格</label><textarea id="' + ids + '_personality" rows="2">' + esc(f.personality) + '</textarea></div>' +
        '<div class="form-group"><label>背景</label><textarea id="' + ids + '_background" rows="2">' + esc(f.background) + '</textarea></div>' +
        '<div class="form-row" style="display:flex;gap:8px">' +
        '<div class="form-group" style="flex:1"><label>行为底线</label><textarea id="' + ids + '_bottomline" rows="2">' + esc(f.bottomline) + '</textarea></div>' +
        '<div class="form-group" style="flex:1"><label>人际关系</label><textarea id="' + ids + '_relations" rows="2" placeholder="与张三角色：师徒兼对手">' + esc(f.relations) + '</textarea></div>' +
        '</div>' +
        '<div class="form-group"><label>备注</label><textarea id="' + ids + '_notes" rows="2">' + esc(f.notes) + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' }
      ].concat(c ? [{ id: 'del', label: '删除', cls: 'btn-danger', onClick: function (m, ov) { ov.remove(); ResourceUI.delCharacter(id); } }] : []).concat([
        {
          id: 'ai-gen', label: '🤖 AI生成', cls: 'btn-ghost', onClick: function (m, ov) {
            var name = document.getElementById(ids + '_name').value.trim();
            if (!name) { UI.toast('请先输入姓名', 'warn'); return; }
            ov.remove();
            ResourceUI.aiGenerateCharacter(name);
          }
        },
        {
          id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
            var name = document.getElementById(ids + '_name').value.trim();
            if (!name) { UI.toast('请输入姓名', 'warn'); return; }
            var fields = {
              gender: document.getElementById(ids + '_gender').value,
              tags: document.getElementById(ids + '_tags').value,
              appearance: document.getElementById(ids + '_appearance').value,
              personality: document.getElementById(ids + '_personality').value,
              background: document.getElementById(ids + '_background').value,
              bottomline: document.getElementById(ids + '_bottomline').value,
              relations: document.getElementById(ids + '_relations').value,
              notes: document.getElementById(ids + '_notes').value
            };
            ov.remove();
            ResourceUI.saveCharacter(id, name, fields);
          }
        }
      ])
    });
  },
  packChar: function (f) {
    var parts = [];
    if (f.gender) parts.push('性别：' + f.gender);
    if (f.tags) parts.push('标签：' + f.tags);
    if (f.appearance) parts.push('外貌：' + f.appearance);
    if (f.personality) parts.push('性格：' + f.personality);
    if (f.background) parts.push('背景：' + f.background);
    if (f.bottomline) parts.push('行为底线：' + f.bottomline);
    if (f.relations) parts.push('人际关系：' + f.relations);
    if (f.notes) parts.push('备注：' + f.notes);
    return parts.join('\n');
  },
  unpackChar: function (desc) {
    var f = { gender: '', tags: '', appearance: '', personality: '', background: '', bottomline: '', relations: '', notes: '' };
    var map = { '性别': 'gender', '标签': 'tags', '角色定位': 'tags', '外貌': 'appearance', '外貌特征': 'appearance', '性格': 'personality', '性格描述': 'personality', '背景': 'background', '背景故事': 'background', '行为底线': 'bottomline', '人际关系': 'relations', '关系网络': 'relations', '能力/技能': 'notes', '能力技能': 'notes', '备注': 'notes' };
    (desc || '').split('\n').forEach(function (line) {
      var m = line.match(/^(性别|标签|角色定位|外貌|外貌特征|性格|性格描述|背景|背景故事|行为底线|人际关系|关系网络|能力\/技能|能力技能|备注)：(.*)$/);
      if (m && map[m[1]]) f[map[m[1]]] = m[2];
    });
    return f;
  },
  aiGenerateCharacter: async function (name) {
    UI.toast('正在为「' + name + '」生成人物设定…', '');
    try {
      var resp = await fetch('/api/generate', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_demand: '请为小说角色「' + name + '」生成完整的人物卡。包含：性别、外貌描述、性格特征（优缺点）、个人背景故事、行为底线原则、人际关系。格式：\n性别：\n外貌：\n性格：\n背景：\n行为底线：\n人际关系：\n备注：',
          run_mode: 'light',
          model_name: Store.state.composer.modelName || ''
        })
      });
      var data = await resp.json();
      if (data.error) { UI.toast(data.error, 'error'); return; }
      // 流式读取
      if (data.channel) { UI.toast('请使用轻量模式生成…', 'warn'); return; }
      // 非流式结果直接返回
      if (data.text) {
        var f = ResourceUI.unpackChar(data.text);
        ResourceUI.editCharacterWithFields(name, f);
        UI.toast('AI 已生成人物设定，请检查并保存', 'success');
      }
    } catch (e) { UI.toast('AI生成失败：' + e.message, 'error'); }
  },
  editCharacterWithFields: function (name, fields) {
    var ids = 'c_' + uid();
    UI.modal({
      title: 'AI生成人物卡 · ' + name, wide: '560px',
      body: '<div class="form-row" style="display:flex;gap:8px">' +
        '<div class="form-group" style="flex:2"><label>姓名 *</label><input id="' + ids + '_name" value="' + esc(name) + '"></div>' +
        '<div class="form-group" style="flex:1"><label>性别</label><select id="' + ids + '_gender"><option value=""' + (!fields.gender ? ' selected' : '') + '>未设定</option><option value="男"' + (fields.gender === '男' ? ' selected' : '') + '>男</option><option value="女"' + (fields.gender === '女' ? ' selected' : '') + '>女</option><option value="其他"' + (fields.gender === '其他' ? ' selected' : '') + '>其他</option></select></div>' +
        '<div class="form-group" style="flex:1"><label>标签</label><input id="' + ids + '_tags" value="' + esc(fields.tags || '') + '"></div>' +
        '</div>' +
        '<div class="form-group"><label>外貌</label><textarea id="' + ids + '_appearance" rows="2">' + esc(fields.appearance || '') + '</textarea></div>' +
        '<div class="form-group"><label>性格</label><textarea id="' + ids + '_personality" rows="2">' + esc(fields.personality || '') + '</textarea></div>' +
        '<div class="form-group"><label>背景</label><textarea id="' + ids + '_background" rows="2">' + esc(fields.background || '') + '</textarea></div>' +
        '<div class="form-row" style="display:flex;gap:8px">' +
        '<div class="form-group" style="flex:1"><label>行为底线</label><textarea id="' + ids + '_bottomline" rows="2">' + esc(fields.bottomline || '') + '</textarea></div>' +
        '<div class="form-group" style="flex:1"><label>人际关系</label><textarea id="' + ids + '_relations" rows="2">' + esc(fields.relations || '') + '</textarea></div>' +
        '</div>' +
        '<div class="form-group"><label>备注</label><textarea id="' + ids + '_notes" rows="2">' + esc(fields.notes || '') + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
            var n = document.getElementById(ids + '_name').value.trim();
            if (!n) { UI.toast('请输入姓名', 'warn'); return; }
            var ff = {
              gender: document.getElementById(ids + '_gender').value,
              tags: document.getElementById(ids + '_tags').value,
              appearance: document.getElementById(ids + '_appearance').value,
              personality: document.getElementById(ids + '_personality').value,
              background: document.getElementById(ids + '_background').value,
              bottomline: document.getElementById(ids + '_bottomline').value,
              relations: document.getElementById(ids + '_relations').value,
              notes: document.getElementById(ids + '_notes').value
            };
            ov.remove();
            ResourceUI.saveCharacter(null, n, ff);
          }
        }
      ]
    });
  },
  saveCharacter: async function (id, name, fields) {
    var desc = this.packChar(fields);
    var pid = Store.state.currentProject.id;
    try {
      if (id) {
        var c = await API.updateCharacter(id, { name: name, description: desc });
        Object.assign(Store.state.characters.find(function (x) { return x.id === id; }) || {}, c);
      } else {
        var nc = await API.createCharacter({ project_id: pid, name: name, description: desc });
        Store.state.characters.push(nc);
      }
      Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
      UI.toast('人物卡已保存', 'success');
    } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
  },
  delCharacter: function (id) {
    var c = Store.state.characters.find(function (x) { return x.id === id; });
    if (!c) return;
    UI.confirm('删除人物卡', '确认删除「' + c.name + '」？', async function () {
      try {
        await API.deleteCharacter(id);
        Store.state.characters = Store.state.characters.filter(function (x) { return x.id !== id; });
        Store.state.selection.characters.delete(id);
        Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
        Store.saveSelection(Store.state.currentProject.id);
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  },

  /* ---- 世界观 ---- */
  editWorld: function (id) {
    var w = id ? Store.state.worldSettings.find(function (x) { return x.id === id; }) : null;
    var f = w ? this.unpackWorld(w.content) : { era: '', tags: '', rules: '', forces: '', geography: '', powers: '' };
    var ids = 'w_' + uid();
    UI.modal({
      title: w ? '编辑世界观' : '新建世界观', wide: '560px',
      body: '<div class="form-row" style="display:flex;gap:8px">' +
        '<div class="form-group" style="flex:2"><label>标题 *</label><input id="' + ids + '_title" value="' + esc(w ? w.title : '') + '"></div>' +
        '<div class="form-group" style="flex:1"><label>时代背景</label><input id="' + ids + '_era" value="' + esc(f.era) + '" placeholder="古代/现代/未来"></div>' +
        '<div class="form-group" style="flex:1"><label>标签</label><input id="' + ids + '_tags" value="' + esc(f.tags) + '" placeholder="玄幻,修仙,东方"></div>' +
        '</div>' +
        '<div class="form-group"><label>世界规则</label><textarea id="' + ids + '_rules" rows="2">' + esc(f.rules) + '</textarea></div>' +
        '<div class="form-group"><label>势力分布</label><textarea id="' + ids + '_forces" rows="2">' + esc(f.forces) + '</textarea></div>' +
        '<div class="form-group"><label>地理设定</label><textarea id="' + ids + '_geography" rows="2">' + esc(f.geography) + '</textarea></div>' +
        '<div class="form-group"><label>力量体系</label><textarea id="' + ids + '_powers" rows="2">' + esc(f.powers) + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' }
      ].concat(w ? [{ id: 'del', label: '删除', cls: 'btn-danger', onClick: function (m, ov) { ov.remove(); ResourceUI.delWorld(id); } }] : []).concat([
        {
          id: 'ai-gen', label: '🤖 AI生成', cls: 'btn-ghost', onClick: function (m, ov) {
            var title = document.getElementById(ids + '_title').value.trim();
            if (!title) { UI.toast('请先输入标题', 'warn'); return; }
            var era = document.getElementById(ids + '_era').value.trim();
            var tags = document.getElementById(ids + '_tags').value.trim();
            ov.remove();
            ResourceUI.aiGenerateWorld(title, era, tags);
          }
        },
        {
          id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
            var title = document.getElementById(ids + '_title').value.trim();
            if (!title) { UI.toast('请输入标题', 'warn'); return; }
            var fields = {
              era: document.getElementById(ids + '_era').value,
              tags: document.getElementById(ids + '_tags').value,
              rules: document.getElementById(ids + '_rules').value,
              forces: document.getElementById(ids + '_forces').value,
              geography: document.getElementById(ids + '_geography').value,
              powers: document.getElementById(ids + '_powers').value
            };
            ov.remove();
            ResourceUI.saveWorld(id, title, fields);
          }
        }
      ])
    });
  },
  packWorld: function (f) {
    var parts = [];
    if (f.era) parts.push('时代背景：' + f.era);
    if (f.tags) parts.push('标签：' + f.tags);
    if (f.rules) parts.push('世界规则：' + f.rules);
    if (f.forces) parts.push('势力分布：' + f.forces);
    if (f.geography) parts.push('地理设定：' + f.geography);
    if (f.powers) parts.push('力量体系：' + f.powers);
    return parts.join('\n');
  },
  unpackWorld: function (content) {
    var f = { era: '', tags: '', rules: '', forces: '', geography: '', powers: '' };
    var map = { '时代背景': 'era', '标签': 'tags', '世界规则': 'rules', '势力分布': 'forces', '地理设定': 'geography', '力量体系': 'powers' };
    (content || '').split('\n').forEach(function (line) {
      var m = line.match(/^(时代背景|标签|世界规则|势力分布|地理设定|力量体系)：(.*)$/);
      if (m) f[map[m[1]]] = m[2];
    });
    return f;
  },
  aiGenerateWorld: async function (title, era, tags) {
    UI.toast('正在为「' + title + '」生成世界观设定…', '');
    try {
      var resp = await fetch('/api/generate', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_demand: '请为小说世界观「' + title + '」' + (era ? '（时代：' + era + '）' : '') + (tags ? '（标签：' + tags + '）' : '') + '生成完整世界观设定。按以下格式输出：\n时代背景：\n标签：\n世界规则：\n势力分布：\n地理设定：\n力量体系：',
          run_mode: 'light',
          model_name: Store.state.composer.modelName || ''
        })
      });
      var data = await resp.json();
      if (data.error) { UI.toast(data.error, 'error'); return; }
      if (data.text) {
        var f = ResourceUI.unpackWorld(data.text);
        ResourceUI.editWorldWithFields(title, f);
        UI.toast('AI 已生成世界观设定，请检查并保存', 'success');
      }
    } catch (e) { UI.toast('AI生成失败：' + e.message, 'error'); }
  },
  editWorldWithFields: function (title, fields) {
    var ids = 'w_' + uid();
    UI.modal({
      title: 'AI生成世界观 · ' + title, wide: '560px',
      body: '<div class="form-row" style="display:flex;gap:8px">' +
        '<div class="form-group" style="flex:2"><label>标题 *</label><input id="' + ids + '_title" value="' + esc(title) + '"></div>' +
        '<div class="form-group" style="flex:1"><label>时代背景</label><input id="' + ids + '_era" value="' + esc(fields.era || '') + '"></div>' +
        '<div class="form-group" style="flex:1"><label>标签</label><input id="' + ids + '_tags" value="' + esc(fields.tags || '') + '"></div>' +
        '</div>' +
        '<div class="form-group"><label>世界规则</label><textarea id="' + ids + '_rules" rows="2">' + esc(fields.rules || '') + '</textarea></div>' +
        '<div class="form-group"><label>势力分布</label><textarea id="' + ids + '_forces" rows="2">' + esc(fields.forces || '') + '</textarea></div>' +
        '<div class="form-group"><label>地理设定</label><textarea id="' + ids + '_geography" rows="2">' + esc(fields.geography || '') + '</textarea></div>' +
        '<div class="form-group"><label>力量体系</label><textarea id="' + ids + '_powers" rows="2">' + esc(fields.powers || '') + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
            var t = document.getElementById(ids + '_title').value.trim();
            if (!t) { UI.toast('请输入标题', 'warn'); return; }
            var ff = {
              era: document.getElementById(ids + '_era').value,
              tags: document.getElementById(ids + '_tags').value,
              rules: document.getElementById(ids + '_rules').value,
              forces: document.getElementById(ids + '_forces').value,
              geography: document.getElementById(ids + '_geography').value,
              powers: document.getElementById(ids + '_powers').value
            };
            ov.remove();
            ResourceUI.saveWorld(null, t, ff);
          }
        }
      ]
    });
  },
  saveWorld: async function (id, title, fields) {
    var content = this.packWorld(fields);
    var pid = Store.state.currentProject.id;
    try {
      if (id) {
        var w = await API.updateWorldSetting(id, { title: title, content: content });
        Object.assign(Store.state.worldSettings.find(function (x) { return x.id === id; }) || {}, w);
      } else {
        var nw = await API.createWorldSetting({ project_id: pid, title: title, content: content });
        Store.state.worldSettings.push(nw);
      }
      Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
      UI.toast('世界观已保存', 'success');
    } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
  },
  delWorld: function (id) {
    var w = Store.state.worldSettings.find(function (x) { return x.id === id; });
    if (!w) return;
    UI.confirm('删除世界观', '确认删除「' + w.title + '」？', async function () {
      try {
        await API.deleteWorldSetting(id);
        Store.state.worldSettings = Store.state.worldSettings.filter(function (x) { return x.id !== id; });
        Store.state.selection.worldSettings.delete(id);
        Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
        Store.saveSelection(Store.state.currentProject.id);
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  },

  /* ---- 素材 ---- */
  uploadMaterial: function () {
    if (!Store.state.currentProject) { UI.toast('请先选择项目', 'warn'); return; }
    var idf = 'mf_' + uid();
    UI.modal({
      title: '上传素材文档',
      sub: '支持 TXT / Markdown / Word(.docx)。上传后可勾选作为生成上下文。',
      body: '<div class="form-group">' +
        '<label class="btn btn-ghost btn-block" style="cursor:pointer;text-align:center">选择文件…' +
        '<input type="file" id="' + idf + '" accept=".txt,.md,.markdown,.docx" style="display:none" onchange="document.getElementById(\'' + idf + '_n\').textContent=this.files[0]?this.files[0].name:\'\'">' +
        '</label><div id="' + idf + '_n" style="font-size:11.5px;color:var(--muted);margin-top:6px"></div></div>' +
        '<div class="form-group"><label>素材名称（可选）</label><input id="' + idf + '_name" placeholder="留空使用文件名"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '上传', cls: 'btn-primary', onClick: function (m, ov) {
            var f = document.getElementById(idf).files[0];
            if (!f) { UI.toast('请选择文件', 'warn'); return; }
            ov.remove();
            ResourceUI.doUpload(f, document.getElementById(idf + '_name').value);
          }
        }
      ]
    });
  },
  doUpload: async function (file, name) {
    var pid = Store.state.currentProject.id;
    var fd = new FormData();
    fd.append('project_id', pid);
    fd.append('file', file);
    if (name) fd.append('name', name);
    try {
      UI.toast('上传中…', '');
      var m = await API.uploadMaterial(fd);
      Store.state.materials.push(m);
      Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
      UI.toast('素材已上传', 'success');
    } catch (e) { UI.toast('上传失败：' + e.message, 'error'); }
  },
  previewMaterial: function (id) {
    var m = Store.state.materials.find(function (x) { return x.id === id; });
    if (!m) return;
    UI.modal({ title: '素材预览 · ' + m.name, body: '<div class="result-box">' + esc(m.content || '(空)') + '</div>', wide: '640px' });
  },
  delMaterial: function (id) {
    var m = Store.state.materials.find(function (x) { return x.id === id; });
    if (!m) return;
    UI.confirm('删除素材', '确认删除「' + m.name + '」？', async function () {
      try {
        await API.deleteMaterial(id);
        Store.state.materials = Store.state.materials.filter(function (x) { return x.id !== id; });
        Store.state.selection.materials.delete(id);
        Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
        Store.saveSelection(Store.state.currentProject.id);
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  }
};
