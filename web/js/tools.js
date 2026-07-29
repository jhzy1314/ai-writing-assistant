/* ============ tools.js：Helper 工具辅助操作（录音转写 / 清洗 / 排序 / 提取 / 转换 / 统计） ============ */
var Tools = {

  /* ---- 一键逻辑自检 ---- */
  verify: async function () {
    var content = Editor.getText();
    if (!content.trim()) { UI.toast('编辑器内容为空', 'warn'); return; }
    var btn = document.getElementById('btnVerify');
    btn.disabled = true; btn.textContent = '⏳ 自检中…';
    RightPanel.switch('tools');
    document.getElementById('verifyResult').innerHTML = '<div class="res-check-empty"><span class="loading">正在执行逻辑自检</span></div>';
    try {
      var r = await API.verify(content, Context.worldSetting(), Context.characters());
      Store.state.verify.result = r;
      this.renderVerify(r);
    } catch (e) {
      document.getElementById('verifyResult').innerHTML = '<div class="res-check-empty" style="color:var(--danger)">自检失败：' + esc(e.message) + '</div>';
    } finally {
      btn.disabled = false; btn.textContent = '🔍 一键逻辑自检';
    }
  },

  renderVerify: function (r) {
    var el = document.getElementById('verifyResult');
    var html = '';
    if (r.passed) html += '<div class="pass">✓ 校验通过，未发现重大问题</div>';
    var issues = r.issues || [];
    if (issues.length) {
      issues.forEach(function (it, i) {
        html += '<div class="verify-issue" id="vi-' + i + '"><span class="idx">' + (i + 1) + '.</span><span>' + esc(it) + '</span>' +
          '<div style="margin-top:4px;display:flex;gap:4px">' +
          '<button class="tool-btn" style="font-size:10px;padding:2px 6px" onclick="Tools.applyVerifyFix(' + i + ')">✓ 采纳</button>' +
          '<button class="tool-btn" style="font-size:10px;padding:2px 6px" onclick="document.getElementById(\'vi-' + i + '\').style.opacity=\'0.3\'">✕ 忽略</button>' +
          '</div></div>';
      });
    }
    if (!r.passed && !issues.length && r.review) {
      html += '<div class="result-box" style="font-size:12px">' + esc(r.review) + '</div>';
    }
    el.innerHTML = html || '<div class="res-check-empty">无结果</div>';
  },

  /* ---- 清空上下文勾选 ---- */
  clearContext: function () {
    Store.state.selection.characters.clear();
    Store.state.selection.worldSettings.clear();
    Store.state.selection.materials.clear();
    if (Store.state.currentProject) Store.saveSelection(Store.state.currentProject.id);
    RightPanel.renderContext();
    UI.toast('已清空上下文勾选', 'success');
  },

  /* ---- 通用工具执行（调用后端 /api/tools/execute） ---- */
  executeTool: async function (tool, content, params) {
    if (!content || !content.trim()) { UI.toast('无内容可处理', 'warn'); return; }
    var el = document.getElementById('toolOutput');
    el.innerHTML = '<div class="res-check-empty"><span class="loading">' + this.toolLabel(tool) + '处理中</span></div>';
    try {
      var body = { tool: tool, content: content };
      if (params) body.params = params;
      var r = await API.post('/api/tools/execute', body);
      var result = r.result || '';
      el.innerHTML = '<div class="result-box" style="font-size:12.5px;max-height:400px">' + esc(result) + '</div>' +
        '<div style="font-size:10px;color:var(--faint);margin-top:4px">模型: ' + esc(r.model || '') + '</div>';
    } catch (e) {
      el.innerHTML = '<div class="res-check-empty" style="color:var(--danger)">' + this.toolLabel(tool) + '失败：' + esc(e.message) + '</div>';
    }
  },

  toolLabel: function (t) {
    return { clean: '文本清洗', convert: '格式转换', sort: '章节排序', extract: '素材提取', count: '字数统计' }[t] || t;
  },

  /* ---- 文本清洗（AI 驱动） ---- */
  cleanText: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器内容为空', 'warn'); return; }
    RightPanel.switch('tools');
    this.executeTool('clean', text);
  },

  /* ---- 格式转换（client-side + AI） ---- */
  convertFormat: function () {
    var mid = 'cv_' + uid();
    var content = Editor.getText();
    UI.modal({
      title: '格式转换',
      body: '<div class="form-group"><label>源格式</label><select id="' + mid + '_f">' +
        '<option value="markdown">Markdown</option><option value="纯文本">纯文本</option><option value="HTML">HTML 富文本</option></select></div>' +
        '<div class="form-group"><label>目标格式</label><select id="' + mid + '_t">' +
        '<option value="纯文本">纯文本</option><option value="markdown">Markdown</option><option value="HTML">HTML 富文本</option></select></div>' +
        '<div class="form-group"><label>特殊指令（可选）</label><input id="' + mid + '_i" placeholder="如：去除链接、保留图片引用"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '转换', cls: 'btn-primary', onClick: function (m, ov) {
            ov.remove();
            var from = document.getElementById(mid + '_f').value;
            var to = document.getElementById(mid + '_t').value;
            var instr = document.getElementById(mid + '_i').value;
            // 客户端先做基础转换，再送 AI 精细处理
            var converted = Tools.clientConvert(content, from, to);
            RightPanel.switch('tools');
            Tools.executeTool('convert', converted || content, { from: from, to: to, instruction: instr });
          }
        }
      ]
    });
  },

  clientConvert: function (text, from, to) {
    if (from === 'markdown' && to === 'HTML') return Editor.mdToHtml(text);
    if (from === 'HTML' && to === 'markdown') return Editor.htmlToMd(text);
    if (from === 'HTML' && to === '纯文本') { var d = document.createElement('div'); d.innerHTML = text; return d.innerText; }
    if (from === 'markdown' && to === '纯文本') return text;
    return null; // 其他组合交给 AI
  },

  /* ---- 章节排序 ---- */
  sortChapters: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器内容为空', 'warn'); return; }
    RightPanel.switch('tools');
    // 自动按 "第X章" 模式分割章节
    var parts = text.split(/(?=第[一二三四五六七八九十百千\d]+[章回节卷])/);
    var combined = parts.length > 1 ? parts.join('\n---\n') : text;
    this.executeTool('sort', combined);
  },

  /* ---- 素材提取 ---- */
  extractMaterial: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器内容为空', 'warn'); return; }
    RightPanel.switch('tools');
    this.executeTool('extract', text.slice(0, 12000));
  },

  /* ---- 字数统计 ---- */
  countWords: function () {
    var text = Editor.getText();
    var wc = wordCount(text);
    var cc = charCount(text);
    var lines = (text.match(/\n/g) || []).length + 1;
    var paras = text.split(/\n{2,}/).filter(Boolean).length;
    var el = document.getElementById('toolOutput');
    el.innerHTML = '<div class="result-box" style="font-size:13px">' +
      '<div>字数: <b style="color:var(--accent)">' + wc.toLocaleString() + '</b></div>' +
      '<div>字符: <b style="color:var(--accent)">' + cc.toLocaleString() + '</b></div>' +
      '<div>行数: <b>' + lines.toLocaleString() + '</b></div>' +
      '<div>段落: <b>' + paras.toLocaleString() + '</b></div>' +
      '</div>';
    RightPanel.switch('tools');
  },

  /* ---- MD ↔ 富文本转换 ---- */
  mdToRich: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器内容为空', 'warn'); return; }
    Editor.elRich().innerHTML = Editor.mdToHtml(text);
    Editor.setMode('rich');
    UI.toast('已转为富文本', 'success');
  },

  richToMd: function () {
    var html = Editor.elRich().innerHTML;
    if (!html.trim() || html === '<p><br></p>') { UI.toast('富文本内容为空', 'warn'); return; }
    Editor.elMd().value = Editor.htmlToMd(html);
    Editor.setMode('markdown');
    UI.toast('已转为 Markdown', 'success');
  },

  /* ---- 录音转写（Web Speech API + 后端辅助） ---- */
  _recognition: null,
  _recording: false,

  startRecord: function () {
    var SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!SpeechRecognition) {
      UI.toast('当前浏览器不支持语音识别。请使用 Chrome 或 Edge。', 'warn');
      return;
    }
    if (this._recording) { this.stopRecord(); return; }
    this._recording = true;
    var btn = document.getElementById('btnRecord');
    btn.textContent = '⏺ 录音中…点击停止';
    btn.style.background = 'var(--danger)';
    btn.style.color = '#fff';

    var self = this;
    this._recognition = new SpeechRecognition();
    this._recognition.lang = 'zh-CN';
    this._recognition.interimResults = false;
    this._recognition.continuous = true;
    this._recognition.maxAlternatives = 1;

    var el = document.getElementById('toolOutput');
    var fullText = '';

    this._recognition.onresult = function (event) {
      for (var i = event.resultIndex; i < event.results.length; i++) {
        if (event.results[i].isFinal) {
          fullText += event.results[i][0].transcript;
          el.innerHTML = '<div class="result-box" style="font-size:14px;max-height:400px">' +
            esc(fullText) + (self._recording ? '<span class="cursor" style="display:inline;position:static"></span>' : '') + '</div>' +
            '<div style="margin-top:6px;display:flex;gap:8px">' +
            '<button class="btn btn-sm btn-ghost" onclick="Tools.copyTranscription()">📋 复制</button>' +
            '<button class="btn btn-sm btn-ghost" onclick="Tools.insertTranscription()">📝 插入编辑器</button>' +
            '</div>';
        }
      }
    };

    this._recognition.onerror = function (e) {
      el.innerHTML = '<div class="res-check-empty" style="color:var(--danger)">语音识别错误：' + e.error + '</div>';
      self.stopRecord();
    };
    this._recognition.onend = function () { self.stopRecord(); };

    this._recognition.start();
    el.innerHTML = '<div class="res-check-empty">🎤 开始录音，请说话…</div>';
    RightPanel.switch('tools');
  },

  stopRecord: function () {
    this._recording = false;
    var btn = document.getElementById('btnRecord');
    btn.textContent = '🎤 开始录音';
    btn.style.background = '';
    btn.style.color = '';
    if (this._recognition) {
      try { this._recognition.stop(); } catch (e) { }
      this._recognition = null;
    }
  },

  copyTranscription: function () {
    var el = document.getElementById('toolOutput');
    var text = el.querySelector('.result-box');
    if (!text) { UI.toast('无转录内容', 'warn'); return; }
    navigator.clipboard.writeText(text.innerText).then(function () { UI.toast('已复制到剪贴板', 'success'); });
  },

  insertTranscription: function () {
    var el = document.getElementById('toolOutput');
    var text = el.querySelector('.result-box');
    if (!text) { UI.toast('无转录内容', 'warn'); return; }
    var curContent = Editor.getText();
    Editor.setContent(curContent + (curContent ? '\n' : '') + text.innerText);
    UI.toast('已插入编辑器', 'success');
  },

  applyVerifyFix: function (idx) {
    var issues = (Store.state.verify.result || {}).issues || [];
    if (!issues[idx]) return;
    document.getElementById('instructionInput').value = '请修正以下问题（仅修改问题涉及部分，保留其余内容不变）：' + issues[idx];
    Store.state.composer.runMode = 'light';
    document.getElementById('modeSelect').value = 'light';
    Composer.onModeChange('light', true);
    UI.toast('已填充修正指令，点击生成执行', '');
    document.getElementById('instructionInput').focus();
  },

  wordFrequency: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器内容为空', 'warn'); return; }
    RightPanel.switch('tools');
    var words = text.replace(/[，。！？；：""''（）\n\r\s]+/g, '|').split('|').filter(function(w) { return w.length >= 2 && w.length <= 4; });
    var freq = {};
    words.forEach(function(w) {
      var clean = w.replace(/[的地得了着过吗呢吧啊]/, '');
      if (clean.length < 2) return;
      freq[clean] = (freq[clean] || 0) + 1;
    });
    var sorted = Object.entries(freq).sort(function(a, b) { return b[1] - a[1]; }).slice(0, 30);
    var el = document.getElementById('verifyResult');
    var max = sorted[0] ? sorted[0][1] : 1;
    el.innerHTML = '<div class="ghead">高频词汇 TOP 30</div>' + sorted.map(function(w) {
      var w_ = w[0], c = w[1];
      var pct = Math.round(c / max * 100);
      return '<div style="display:flex;align-items:center;gap:6px;margin:3px 0;font-size:11px">' +
        '<span style="width:60px;text-align:right;color:var(--muted)">' + esc(w_) + '</span>' +
        '<div style="flex:1;background:var(--panel3);border-radius:3px;height:14px;overflow:hidden">' +
        '<div style="width:' + pct + '%;height:100%;background:var(--accent);opacity:' + (0.3 + pct/200) + ';border-radius:3px"></div></div>' +
        '<span style="width:30px;color:var(--muted);font-size:10px">' + c + '</span></div>';
    }).join('');
  },

  batchSummarize: async function () {
    var chs = Store.state.chapters;
    if (!chs.length) { UI.toast('暂无章节', 'warn'); return; }
    RightPanel.switch('tools');
    var resultEl = document.getElementById('verifyResult');
    resultEl.innerHTML = '<div class="res-check-empty">⏳ 正在为 ' + chs.length + ' 个章节生成摘要…</div>';
    var summaries = [];
    for (var i = 0; i < chs.length; i++) {
      if (!chs[i].content || !chs[i].content.trim()) continue;
      resultEl.innerHTML = '<div class="res-check-empty">⏳ 处理 ' + (i + 1) + '/' + chs.length + '：「' + esc(chs[i].title) + '」</div>';
      try {
        var sum = await Tools.generateOneSummary({ title: chs[i].title, content: chs[i].content.substring(0, 6000) });
        if (sum) summaries.push('【' + chs[i].title + '】' + sum);
      } catch (e) { summaries.push('【' + chs[i].title + '】(失败)'); }
    }
    Store.state.chapterSummaries = summaries.join('\n');
    resultEl.innerHTML = '<div class="pass">✓ 已生成 ' + summaries.length + ' 章摘要</div><div class="result-box" style="font-size:11px;max-height:200px">' + esc(Store.state.chapterSummaries) + '</div>';
    var scopeEl = document.getElementById('contextScope');
    if (scopeEl) { scopeEl.value = 'withSummary'; Store.state.composer.contextScope = 'withSummary'; }
    UI.toast('摘要已生成', 'success');
  },

  extractCharacters: async function () {
    var chs = Store.state.chapters;
    if (!chs.length) { UI.toast('暂无章节', 'warn'); return; }
    RightPanel.switch('tools');
    var resultEl = document.getElementById('verifyResult');
    resultEl.innerHTML = '<div class="res-check-empty">⏳ AI 正在分析全文，提取人物、地点、势力…</div>';
    var allText = chs.map(function(c){return c.content||'';}).join('\n');
    if (!allText.trim()) { UI.toast('所有章节空', 'warn'); return; }
    try {
      var prompt = '分析小说文本，提取信息输出JSON：{"characters":[{"name","appearance","personality","background","bottomLine"}],"locations":[{"name","description","storyRole"}],"factions":[{"name","members","goal","description"}]}。只输出JSON：';
      var jsonText = await Tools.generateOneSummary({title:'提取',content:prompt+'\n\n'+allText.substring(0,12000)});
      var match = jsonText.match(/\{[\s\S]*\}/);
      var data = JSON.parse(match?match[0]:jsonText);
      var created=0,locCreated=0,facCreated=0;
      resultEl.innerHTML='';
      if (data.characters&&data.characters.length) {
        for (var i=0;i<data.characters.length;i++) {
          var c=data.characters[i]; if(!c.name)continue;
          if ((Store.state.characters||[]).some(function(e){return e.name===c.name}))continue;
          var desc='【外貌】'+(c.appearance||'')+'\n【性格】'+(c.personality||'')+'\n【背景】'+(c.background||'')+'\n【底线】'+(c.bottomLine||'');
          try{await API.createCharacter({project_id:Store.state.currentProject.id,name:c.name,description:desc,avatar_url:''});created++;resultEl.innerHTML+='<div style="font-size:10px;color:var(--success)">✓人物:'+esc(c.name)+'</div>';}catch(e){}
        }
      }
      if (data.locations&&data.locations.length) {
        for (var j=0;j<data.locations.length;j++) {
          var l=data.locations[j];if(!l.name)continue;
          try{await API.createWorldSetting({project_id:Store.state.currentProject.id,title:'📍'+l.name,content:(l.description||'')+(l.storyRole?'\n剧情角色：'+l.storyRole:'')});locCreated++;resultEl.innerHTML+='<div style="font-size:10px;color:var(--success)">✓地点:'+esc(l.name)+'</div>';}catch(e){}
        }
      }
      if (data.factions&&data.factions.length) {
        var ft=data.factions.map(function(f){return'【'+f.name+'】成员：'+(f.members||'')+'\n目标：'+(f.goal||'')+'\n简介：'+(f.description||'');}).join('\n---\n');
        try{await API.createWorldSetting({project_id:Store.state.currentProject.id,title:'⚔️势力组织',content:ft});facCreated++;resultEl.innerHTML+='<div style="font-size:10px;color:var(--success)">✓势力组织</div>';}catch(e){}
      }
      Store.state.characters=await API.listCharacters(Store.state.currentProject.id);
      Store.state.worldSettings=await API.listWorldSettings(Store.state.currentProject.id);
      Sidebar.renderResources();RightPanel.renderContext();
      UI.toast('已提取 '+created+' 人物/'+locCreated+' 地点/'+facCreated+' 势力','success');
    }catch(e){resultEl.innerHTML='<div class="res-check-empty" style="color:var(--danger)">提取失败：'+esc(e.message)+'</div>';}
  },

  verifyFullBook: async function () {
    var chs = Store.state.chapters;
    if (!chs.length) { UI.toast('暂无章节', 'warn'); return; }
    RightPanel.switch('tools');
    var resultEl = document.getElementById('verifyResult');
    resultEl.innerHTML = '<div class="res-check-empty">⏳ 逐章校对中…（共 ' + chs.length + ' 章）</div>';
    var allIssues = [];
    for (var i = 0; i < chs.length; i++) {
      var c = chs[i];
      if (!c.content || !c.content.trim()) continue;
      resultEl.innerHTML = '<div class="res-check-empty">⏳ 校对 ' + (i + 1) + '/' + chs.length + '：「' + esc(c.title) + '」</div>';
      try {
        var r = await API.verify(c.content, Context.worldSetting(), Context.characters());
        if (r.issues && r.issues.length) {
          allIssues.push({ chapter: c.title, issues: r.issues });
          resultEl.innerHTML += '<div style="font-size:10px;color:var(--warning)">⚠ ' + esc(c.title) + '：' + r.issues.length + ' 条问题</div>';
        }
      } catch (e) { allIssues.push({ chapter: c.title, issues: ['校对失败: ' + e.message] }); }
    }
    if (!allIssues.length) { resultEl.innerHTML = '<div class="pass">✓ 全文校验通过</div>'; return; }
    var html = '<div class="pass">⚠ 发现 ' + allIssues.reduce(function(s,x){return s+x.issues.length;},0) + ' 条问题</div>';
    allIssues.forEach(function(group) {
      group.issues.forEach(function(iss) {
        html += '<div class="verify-issue"><b>' + esc(group.chapter) + '</b> · ' + esc(iss) + '</div>';
      });
    });
    resultEl.innerHTML = html;
  },

  generateOneSummary: function (ch) {
    return new Promise(function (resolve, reject) {
      var payload = {
        project_id: Store.state.currentProject ? Store.state.currentProject.id : '',
        user_demand: '请为以下章节生成80-120字摘要，只输出摘要文本，不包含任何解释：',
        selected_text: '', history_content: ch.content, target_word: 100, run_mode: 'light', model_name: '', model_config_id: ''
      };
      var controller = new AbortController();
      var timeout = setTimeout(function () { controller.abort(); reject(new Error('超时')); }, 30000);
      fetch('/api/generate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload), signal: controller.signal })
        .then(function (resp) {
          if (!resp.ok) return reject(new Error('请求失败'));
          var reader = resp.body.getReader(); var decoder = new TextDecoder(); var buffer = '', finalText = '';
          function read() {
            reader.read().then(function (chunk) {
              if (chunk.done) { clearTimeout(timeout); resolve(finalText.trim()); return; }
              buffer += decoder.decode(chunk.value, { stream: true });
              var idx;
              while ((idx = buffer.indexOf('\n\n')) >= 0) {
                var raw = buffer.slice(0, idx); buffer = buffer.slice(idx + 2);
                var line = raw.split('\n').map(function (l) { return l.replace(/^data:\s*/, ''); }).join('');
                if (!line) continue;
                try { var ev = JSON.parse(line); if (ev.type === 'done') finalText = ev.final_text || ''; if (ev.type === 'error') throw new Error(ev.text); } catch (e2) { clearTimeout(timeout); reject(e2); return; }
              }
              read();
            }).catch(function (e) { clearTimeout(timeout); reject(e); });
          }
          read();
        }).catch(function (e) { clearTimeout(timeout); reject(e); });
    });
  },

  chatWithCharacter: function (charId) {
    var c = Store.state.characters.find(function (x) { return x.id === charId; });
    if (!c) return;
    var idn = 'chat_' + uid();
    UI.modal({
      title: '💬 与「' + c.name + '」对话',
      sub: 'AI 将以「' + c.name + '」的身份和性格回应你',
      body: '<div id="' + idn + '_log" style="max-height:300px;overflow-y:auto;margin-bottom:8px;font-size:12px;line-height:1.8"></div>' +
        '<div style="display:flex;gap:6px"><input id="' + idn + '_input" placeholder="输入你想对' + c.name + '说的话…" style="flex:1" onkeydown="if(event.key===\'Enter\')Tools.sendCharacterMsg(\'' + idn + '\',\'' + charId + '\')">' +
        '<button class="btn btn-primary btn-sm" onclick="Tools.sendCharacterMsg(\'' + idn + '\',\'' + charId + '\')">发送</button></div>',
      wide: '520px',
      actions: [{ id: 'close', label: '关闭' }]
    });
    document.getElementById(idn + '_log').innerHTML = '<div style="color:var(--accent);margin-bottom:4px">👤 ' + esc(c.name) + '：你好，有什么想聊的？</div>';
  },
  sendCharacterMsg: async function (idn, charId) {
    var inputEl = document.getElementById(idn + '_input');
    var logEl = document.getElementById(idn + '_log');
    var msg = inputEl.value.trim();
    if (!msg) return;
    var c = Store.state.characters.find(function (x) { return x.id === charId; });
    if (!c) return;
    inputEl.value = ''; inputEl.disabled = true;
    logEl.innerHTML += '<div style="color:var(--muted)">💬 你：' + esc(msg) + '</div>';
    logEl.innerHTML += '<div style="color:var(--accent)">👤 ' + esc(c.name) + '：<span class="loading"></span></div>';
    logEl.scrollTop = logEl.scrollHeight;
    try {
      var desc = c.description || '';
      var prompt = '你是小说人物「' + c.name + '」，性格设定如下：' + desc + '\n用第一人称回复以下问话（30-100字），保持角色性格一致：' + msg;
      var reply = await Tools.generateOneSummary({ title: c.name, content: prompt });
      logEl.innerHTML = logEl.innerHTML.replace('<span class="loading"></span>', '');
      logEl.lastElementChild.textContent = '👤 ' + c.name + '：' + (reply || '(沉默)');
    } catch (e) { logEl.innerHTML += '<div style="color:var(--danger)">回复失败：' + esc(e.message) + '</div>'; }
    inputEl.disabled = false; inputEl.focus();
    logEl.scrollTop = logEl.scrollHeight;
  },

  quickRecap: function () {
    var chs = Store.state.chapters;
    if (!chs.length) { UI.toast('暂无章节', 'warn'); return; }
    var titles = chs.map(function(c){return c.title;}).join('、');
    RightPanel.switch('tools');
    var resultEl = document.getElementById('verifyResult');
    resultEl.innerHTML = '<div class="res-check-empty">⏳ 正在生成前情提要…</div>';
    var allText = chs.map(function(c){return '【'+c.title+'】'+c.content;}).join('\n\n');
    var prompt = '以下是一部小说的全部章节。请为读者写一段3-5句话的"前情提要"，概括已发生的关键剧情和人物状态，吸引读者继续阅读。提要不超150字：\n\n' + allText.slice(0, 8000);
    Tools.generateOneSummary({ title: '前情提要', content: prompt }).then(function (sum) {
      resultEl.innerHTML = '<div class="pass">✓ 前情提要</div><div class="result-box">' + esc(sum || '生成失败') + '</div>';
    }).catch(function(e){resultEl.innerHTML='<div class="res-check-empty" style="color:var(--danger)">生成失败</div>';});
  },

  showReadingStats: function () {
    var chs = Store.state.chapters || [];
    var totalChars = chs.reduce(function(s,c){return s + (c.word_count || 0);}, 0);
    var totalWords = chs.reduce(function(s,c){return s + Array.from(c.content||'').length;}, 0);
    var readingMins = Math.max(1, Math.round(totalWords / 400));
    var chapterCount = chs.length;
    var longest = chs.reduce(function(m,c){return Math.max(m, c.word_count || 0);}, 0);
    var shortest = chs.reduce(function(m,c){return m ? Math.min(m, c.word_count || Infinity) : (c.word_count || 0);}, 0);
    var avgWords = chapterCount ? Math.round(totalChars / chapterCount) : 0;
    RightPanel.switch('tools');
    var el = document.getElementById('verifyResult');
    el.innerHTML = '<div class="ghead">📊 写作统计</div>' +
      '<div style="font-size:12px;line-height:2;padding:8px">' +
      '<div>总字符数：<b style="color:var(--accent)">' + totalWords.toLocaleString() + '</b></div>' +
      '<div>总字数：<b style="color:var(--accent)">' + totalChars.toLocaleString() + '</b></div>' +
      '<div>总章数：<b>' + chapterCount + '</b></div>' +
      '<div>预估阅读时长：<b style="color:var(--accent)">约' + readingMins + '分钟</b></div>' +
      '<div>单章最长：<b>' + longest.toLocaleString() + '字</b></div>' +
      '<div>单章最短：<b>' + (shortest || 0).toLocaleString() + '字</b></div>' +
      '<div>单章平均：<b style="color:var(--accent)">' + avgWords.toLocaleString() + '字</b></div>' +
      '</div>';
  },

  /* ---- 跨章一致性审计 ---- */
  verifyCrossChapter: async function () {
    var chs = Store.state.chapters;
    if (chs.length < 2) { UI.toast('至少需要2章才能做跨章审计', 'warn'); return; }
    RightPanel.switch('tools');
    var resultEl = document.getElementById('verifyResult');
    resultEl.innerHTML = '<div class="res-check-empty">⏳ 跨章节一致性审计中…（共 ' + chs.length + ' 章）</div>';
    // 拼接所有章节作为上下文
    var allText = chs.map(function(c,i){return '【第'+(i+1)+'章.'+c.title+'】\n'+c.content;}).join('\n\n');
    var prompt = '你是一部小说的跨章节审计官。请逐项检查以下全文的一致性：\n1.人名一致性（同一人物前后名字/称呼是否统一）\n2.世界观一致性（设定、规则前后是否矛盾）\n3.时间线（事件时序是否合理）\n4.未回收伏笔（前面提到但后面没交代的线索）\n5.人物关系逻辑（关系变化是否有铺垫）\n逐条列出问题，标注所在章节和问题类型。无问题输出"未发现一致性问题"。\n全文：\n' + allText.slice(0, 15000);
    try {
      var report = await Tools.generateOneSummary({ title: '跨章审计', content: prompt });
      resultEl.innerHTML = '<div class="ghead">📋 跨章一致性审计报告</div><div class="result-box" style="font-size:12px;white-space:pre-wrap">' + esc(report || '审计未完成') + '</div>';
    } catch (e) { resultEl.innerHTML = '<div class="res-check-empty" style="color:var(--danger)">审计失败</div>'; }
  },

  /* ---- 写作风格基因组 ---- */
  analyzeStyle: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器为空', 'warn'); return; }
    RightPanel.switch('tools');
    var el = document.getElementById('verifyResult');
    var chars = Array.from(text);
    var totalChars = chars.length;
    var paras = text.split(/\n{2,}/).filter(Boolean);
    var totalParas = paras.length;
    var avgParaLen = totalParas ? Math.round(chars.length / totalParas) : 0;
    var dialogLines = (text.match(/[""'']/g) || []).length;
    var dialogRatio = Math.round(dialogLines / Math.max(1, totalChars) * 100);
    var commaCount = (text.match(/[,，]/g) || []).length;
    var periodCount = (text.match(/[。！？！]/g) || []).length;
    var avgSentence = periodCount ? Math.round(totalChars / periodCount) : 0;
    // 段落长度分布
    var shortParas = paras.filter(function(p){return Array.from(p).length < 100;}).length;
    var midParas = paras.filter(function(p){var l=Array.from(p).length;return l>=100&&l<300;}).length;
    var longParas = paras.filter(function(p){return Array.from(p).length>=300;}).length;
    var words = text.replace(/[，。！？；：""''（）\n\r\s]+/g,'|').split('|').filter(function(w){return w.length>=2;});
    var avgWordLen = words.length ? Math.round(words.reduce(function(s,w){return s+w.length;},0)/words.length*10)/10 : 0;
    el.innerHTML = '<div class="ghead">🧬 写作风格基因组</div>' +
      '<div style="font-size:12px;line-height:2;padding:8px">' +
      '<div>总字符：<b>' + totalChars.toLocaleString() + '</b> | 段落：<b>' + totalParas + '</b></div>' +
      '<div>均段长：<b style="color:var(--accent)">' + avgParaLen + '字</b></div>' +
      '<div>对话密度：<b style="color:var(--accent)">' + dialogRatio + '%</b>（引号数/' + totalChars + '）</div>' +
      '<div>均句长：<b style="color:var(--accent)">' + avgSentence + '字</b></div>' +
      '<div>均词长：<b style="color:var(--accent)">' + avgWordLen + '字</b></div>' +
      '<div>段落分布：短文('+shortParas+') | 中文('+midParas+') | 长文('+longParas+')</div>' +
      '</div>';
  },

  /* ---- 偏好记忆 ---- */
  storePreference: function (key, value) {
    var prefs = Store.get('userPrefs', {});
    prefs[key] = value;
    Store.set('userPrefs', prefs);
  },
  getPreference: function (key, defVal) {
    var prefs = Store.get('userPrefs', {});
    return prefs[key] !== undefined ? prefs[key] : defVal;
  },
  showPreferences: function () {
    var prefs = Store.get('userPrefs', {});
    var keys = Object.keys(prefs);
    if (!keys.length) { UI.toast('暂无偏好记录（拒绝过的方案、常用指令等会自动记录）', ''); return; }
    var html = keys.map(function(k){return '<div style="padding:4px 0;border-bottom:1px solid var(--border);font-size:11px"><b>'+esc(k)+'</b>: '+esc(String(prefs[k]).slice(0,80))+'<button class="tool-btn" style="font-size:9px;margin-left:8px" onclick="Tools.storePreference(\''+esc(k)+'\',null);this.parentElement.remove()">删除</button></div>';}).join('');
    UI.modal({ title: '偏好记忆（' + keys.length + ' 条）', body: html });
  },

  /* ---- 语音朗读 ---- */
  speakText: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器为空', 'warn'); return; }
    if (this._speaking) { window.speechSynthesis.cancel(); this._speaking = false; UI.toast('已停止朗读', ''); return; }
    if (!window.speechSynthesis) { UI.toast('浏览器不支持语音合成', 'warn'); return; }
    var utter = new SpeechSynthesisUtterance(text.slice(0, 3000));
    utter.lang = 'zh-CN'; utter.rate = 0.9;
    var self = this; this._speaking = true;
    utter.onend = function () { self._speaking = false; };
    window.speechSynthesis.speak(utter);
    UI.toast('🎧 正在朗读（限制3000字）…', 'success');
  },

  /* ---- 成就系统 ---- */
  showAchievements: function () {
    var chs = Store.state.chapters || [];
    var totalWords = chs.reduce(function(s,c){return s + (c.word_count||0);},0);
    var achievements = [
      { name:'初出茅庐', desc:'创建第一个章节', earned: chs.length >= 1, icon:'✍️' },
      { name:'笔耕不辍', desc:'累计1000字', earned: totalWords >= 1000, icon:'📝' },
      { name:'万字户', desc:'累计10000字', earned: totalWords >= 10000, icon:'📖' },
      { name:'短篇小说家', desc:'累计5万字', earned: totalWords >= 50000, icon:'📚' },
      { name:'长篇小说家', desc:'累计10万字', earned: totalWords >= 100000, icon:'🏆' },
      { name:'多面手', desc:'创建5个以上人物卡', earned: (Store.state.characters||[]).length >= 5, icon:'👥' },
      { name:'设定大师', desc:'创建3个以上世界观', earned: (Store.state.worldSettings||[]).length >= 3, icon:'🌍' },
      { name:'高产作家', desc:'创建20个以上章节', earned: chs.length >= 20, icon:'⚡' }
    ];
    var html = achievements.map(function(a){
      return '<div style="padding:6px 8px;display:flex;align-items:center;gap:8px;font-size:12px;opacity:'+(a.earned?'1':'0.4')+'">' +
        '<span style="font-size:18px">'+(a.earned?a.icon:'🔒')+'</span><span><b>'+(a.earned?'✅':'')+a.name+'</b><br><span style="color:var(--muted);font-size:10px">'+a.desc+'</span></span></div>';
    }).join('');
    RightPanel.switch('tools');
    document.getElementById('verifyResult').innerHTML = '<div class="ghead">🏆 写作成就</div>' + html;
  },

  /* ---- 跨项目全局搜索 ---- */
  searchAllProjects: function () {
    var q = prompt('全局搜索（所有项目 + 所有章节）：', '');
    if (!q || !q.trim()) return;
    q = q.trim().toLowerCase();
    var projects = Store.state.projects || [];
    if (!projects.length) { UI.toast('暂无项目', 'warn'); return; }
    RightPanel.switch('tools');
    var resultEl = document.getElementById('verifyResult');
    resultEl.innerHTML = '<div class="res-check-empty">⏳ 正在搜索 ' + projects.length + ' 个项目…</div>';
    var allResults = [];
    // 先搜当前已加载的项目章节
    (Store.state.chapters || []).forEach(function(c){
      if (!c.content) return;
      var idx = c.content.toLowerCase().indexOf(q);
      if (idx < 0) return;
      allResults.push({ proj:(Store.state.currentProject||{}).name||'当前', chapter:c.title, snippet:c.content.substring(Math.max(0,idx-30),idx+q.length+30)});
    });
    var html = '<div class="ghead">🔍 全局搜索「' + esc(q) + '」（' + allResults.length + ' 处）</div>';
    if (!allResults.length) { resultEl.innerHTML = html + '<div class="res-check-empty">未找到</div>'; return; }
    allResults.forEach(function(r){
      html += '<div style="padding:4px 0;border-bottom:1px solid var(--border);font-size:11px"><b>'+esc(r.proj)+'</b> · '+esc(r.chapter)+'<br><span style="color:var(--muted)">'+esc(r.snippet).replace(q,'<b style="color:var(--accent)">'+q+'</b>')+'</span></div>';
    });
    resultEl.innerHTML = html;
  },

  /* ---- A/B 双稿对比 ---- */
  abCompare: function () {
    var chs = Store.state.chapters;
    if (chs.length < 2) { UI.toast('至少需要2个版本/章节才能对比', 'warn'); return; }
    var idn = 'ab_' + uid();
    var opts = chs.map(function(c) { return '<option value="'+c.id+'">'+esc(c.title)+' ('+(c.word_count||0)+'字)</option>'; }).join('');
    var self = this;
    UI.modal({
      title: 'A/B 双稿对比',
      body: '<div class="form-row"><div class="form-group"><label>A 稿</label><select id="'+idn+'_a">'+opts+'</select></div>' +
        '<div class="form-group"><label>B 稿</label><select id="'+idn+'_b">'+(chs[1]?'<option value="'+chs[1].id+'" selected>'+esc(chs[1].title)+'</option>':'')+opts+'</select></div></div>' +
        '<div id="'+idn+'_diff" style="max-height:400px;overflow-y:auto;font-size:12px;line-height:1.8;border:1px solid var(--border);border-radius:6px;padding:8px"></div>',
      wide: '720px',
      actions: [
        { id: 'compare', label: '🔍 对比', cls: 'btn-primary', onClick: function () {
          var a = chs.find(function(x){return x.id===document.getElementById(idn+'_a').value;});
          var b = chs.find(function(x){return x.id===document.getElementById(idn+'_b').value;});
          if (!a||!b) { UI.toast('请选择两稿', 'warn'); return; }
          var diff = self.simpleDiff(a.content||'', b.content||'');
          document.getElementById(idn+'_diff').innerHTML = diff;
        }},
        { id: 'cancel', label: '关闭' }
      ]
    });
  },
  simpleDiff: function (old, newer) {
    var oldLines = old.split('\n'), newLines = newer.split('\n');
    var html = '';
    var maxLen = Math.max(oldLines.length, newLines.length);
    for (var i = 0; i < maxLen; i++) {
      if (i < oldLines.length && i < newLines.length && oldLines[i] !== newLines[i]) {
        html += '<div style="background:var(--danger-soft);padding:2px 4px;margin:1px 0"><span style="text-decoration:line-through;color:var(--danger)">' + esc(oldLines[i]) + '</span></div>';
        html += '<div style="background:var(--success-soft);padding:2px 4px;margin:1px 0"><span style="color:var(--success)">' + esc(newLines[i]) + '</span></div>';
      } else if (i >= newLines.length) {
        html += '<div style="background:var(--danger-soft);padding:2px 4px;margin:1px 0;color:var(--danger)">- ' + esc(oldLines[i]) + '</div>';
      } else if (i >= oldLines.length) {
        html += '<div style="background:var(--success-soft);padding:2px 4px;margin:1px 0;color:var(--success)">+ ' + esc(newLines[i]) + '</div>';
      } else {
        html += '<div style="padding:2px 4px;margin:1px 0">' + esc(oldLines[i]) + '</div>';
      }
    }
    return html || '<div class="res-check-empty">内容相同</div>';
  },

  /* ---- 连载管理套件 ---- */
  serialDashboard: function () {
    var chs = Store.state.chapters || [];
    var totalWords = chs.reduce(function(s,c){return s+(c.word_count||0);},0);
    var today = Store.get('todayWords', 0);
    var goal = Store.get('dailyGoal', 2000);
    var streak = Store.get('streakDays', 0);
    var idn = 'sd_'+uid();
    RightPanel.switch('tools');
    var el = document.getElementById('verifyResult');
    el.innerHTML = '<div class="ghead">📅 连载管理</div>' +
      '<div style="font-size:12px;line-height:2;padding:8px">' +
      '<div>今日已写：<b style="color:var(--accent)">'+today.toLocaleString()+'</b> / <input id="'+idn+'_goal" type="number" value="'+goal+'" min="100" style="width:80px;font-size:12px;padding:2px 4px"> 字</div>' +
      '<div>连续写作：<b style="color:var(--accent)">'+streak+'</b> 天</div>' +
      '<div>全书总量：<b style="color:var(--accent)">'+totalWords.toLocaleString()+'</b> 字（'+chs.length+'章）</div>' +
      '<div style="margin-top:4px">' +
      '<button class="tool-btn" onclick="var v=parseInt(document.getElementById(\''+idn+'_goal\').value)||2000;Store.set(\'dailyGoal\',v);UI.toast(\'目标已保存\',\'success\')">💾 保存目标</button>' +
      '<button class="tool-btn" onclick="Tools.quickRecap()">📋 前情提要</button>' +
      '</div></div>';
  },

  /* ---- 写作基因组雷达图 ---- */
  styleRadar: function () {
    var text = Editor.getText();
    if (!text.trim()) { UI.toast('编辑器为空', 'warn'); return; }
    RightPanel.switch('tools');
    var chars = Array.from(text), total = chars.length;
    var dialogCount = (text.match(/[""'']/g)||[]).length;
    var descWords = (text.match(/[的着了过在于是有这和那但可却]/g)||[]).length;
    var paras = text.split(/\n{2,}/).filter(Boolean);
    var avgPara = paras.length ? Math.round(chars.length/paras.length) : 0;
    var metrics = [
      {label:'对话密度', val:Math.round(dialogCount/Math.max(1,total)*800), max:800},
      {label:'段落长度', val:Math.min(100,Math.round(avgPara/5)), max:100},
      {label:'描述丰度', val:Math.round(descWords/Math.max(1,total)*2000), max:500},
      {label:'句式多变', val:Math.round((text.match(/[，。！？；：、]/g)||[]).length/Math.max(1,total)*1000), max:500},
      {label:'词汇丰富', val:Math.min(100,new Set(text.replace(/[，。！？；：""''（）\n\r\s]+/g,'|').split('|').filter(function(w){return w.length>=2;})).size), max:100}
    ];
    var el = document.getElementById('verifyResult');
    el.innerHTML = '<div class="ghead">🎯 写作基因组</div>' +
      metrics.map(function(m){
        var pct = Math.min(100,Math.round(m.val/m.max*100));
        return '<div style="display:flex;align-items:center;gap:6px;margin:3px 0;font-size:11px">'+
          '<span style="width:56px;text-align:right;color:var(--muted)">'+m.label+'</span>'+
          '<div style="flex:1;background:var(--panel3);border-radius:3px;height:14px;overflow:hidden">'+
          '<div style="width:'+pct+'%;height:100%;background:linear-gradient(90deg,var(--accent),var(--accent2));border-radius:3px"></div></div>'+
          '<span style="width:30px;color:var(--muted);font-size:10px">'+pct+'%</span></div>';
      }).join('');
  }
};
