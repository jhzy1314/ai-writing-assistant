/* ============ sse.js：SSE 流式生成与终止 ============ */
var SSE = {
  controller: null,
  active: false,
  streamText: '',
  start: async function (payload) {
    if (this.active) { UI.toast('已有生成任务进行中', 'warn'); return; }
    this.active = true;
    this.streamText = '';
    this._tokenCount = 0;
    this._snapshotCount = 0;
    // 快照绑定：防止生成中切换项目/章节导致串文
    this._bindChapterId = (Store.state.currentChapter || {}).id || '';
    this._bindProjectId = (Store.state.currentProject || {}).id || '';
    // 保存 payload 用于断连重试
    this._lastPayload = payload;
    this._retryCount = 0;
    this.controller = new AbortController();
    Composer.setGenerating(true);
    PipelineUI.reset();
    // 自动快照：每 90 秒保存一次当前进度
    this._snapshotTimer = setInterval(function () {
      if (!SSE.active || !SSE._bindChapterId || !SSE.streamText) return;
      var ch = Store.state.chapters.find(function (c) { return c.id === SSE._bindChapterId; });
      if (!ch) return;
      var snapTitle = (ch.title || '未命名') + ' · 自动快照 ' + new Date().toLocaleTimeString('zh-CN', { hour12: false });
      API.saveChapterVersion(ch.id, { title: snapTitle, content: SSE.streamText }).then(function () {
        SSE._snapshotCount++;
        PipelineUI.log('📸 快照 #' + SSE._snapshotCount + ' 已保存（' + Array.from(SSE.streamText || '').length.toLocaleString() + ' 字）');
        document.getElementById('pipeSnapshotCount').textContent = SSE._snapshotCount;
        document.getElementById('pipeSnapshotBadge').style.display = '';
      }).catch(function () { /* 静默失败 */ });
    }, 90000);
    var resp;
    try {
      resp = await fetch('/api/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal: this.controller.signal
      });
    } catch (e) {
      if (e.name === 'AbortError') { this.finish(); UI.toast('已终止生成', 'warn'); PipelineUI.setActive(false); return; }
      // 断连时缓存已有内容，并尝试重试一次
      var boundCh = Store.state.chapters.find(function(c){return c.id===SSE._bindChapterId;});
      if (boundCh && this.streamText) {
        API.updateChapter(boundCh.id, { content: this.streamText }).catch(function(){});
      }
      if (this._retryCount < 1 && this._lastPayload) {
        this._retryCount++;
        this.active = false; this.controller = null;
        UI.toast('网络波动，正在自动重试…（已缓存 ' + Array.from(this.streamText || '').length.toLocaleString() + ' 字）', 'warn');
        var savedPayload = this._lastPayload;
        setTimeout(function () { SSE.start(savedPayload); }, 1500);
        return;
      }
      this.finish();
      UI.toast('连接失败（已重试）—— 已缓存已生成内容，请检查网络后重新生成', 'error');
      return;
    }
    if (!resp.ok) {
      var msg = '生成请求失败';
      try { var d = await resp.json(); msg = d.error || msg; } catch (e) { }
      this.finish();
      UI.toast(msg, 'error');
      PipelineUI.setActive(false);
      return;
    }
    var reader = resp.body.getReader();
    var decoder = new TextDecoder();
    var buffer = '';
    try {
      while (true) {
        var chunk = await reader.read();
        if (chunk.done) break;
        buffer += decoder.decode(chunk.value, { stream: true });
        var idx;
        while ((idx = buffer.indexOf('\n\n')) >= 0) {
          var raw = buffer.slice(0, idx);
          buffer = buffer.slice(idx + 2);
          var line = raw.split('\n').map(function (l) { return l.replace(/^data:\s*/, ''); }).join('');
          if (!line) continue;
          var ev;
          try { ev = JSON.parse(line); } catch (e) { continue; }
          this.handle(ev);
        }
      }
    } catch (e) {
      if (e.name !== 'AbortError') UI.toast('流式读取异常：' + e.message, 'error');
    }
    this.finish();
  },
  handle: function (ev) {
    switch (ev.type) {
      case 'estimate':
        PipelineUI.setStage('预估 Token…', 'idle', 8);
        PipelineUI.log('预估 Token：' + (ev.tokens || 0));
        break;
      case 'plan':
        PipelineUI.setStage(ev.stage || '流水线已规划', 'idle', 15);
        PipelineUI.log('执行计划：' + (ev.stage || ''));
        if (Store.state.composer.runMode === 'auto') {
          UI.toast('智能协同 → 已选择「' + (ev.stage || ev.pipeline || '') + '」', '');
        }
        break;
      case 'stage':
        PipelineUI.applyStage(ev);
        break;
      case 'token':
        if (ev.reset) { this.streamText = ''; this._tokenCount = 0; Editor.resetStream(); }
        if (ev.text) {
          this.streamText += ev.text; Editor.appendStream(ev.text);
          this._tokenCount = (this._tokenCount || 0) + Math.ceil(Array.from(ev.text).length / 1.5);
          // 实时更新配额条
          document.getElementById('qTokens').textContent = ((Store.state.usage && Store.state.usage.today ? Store.state.usage.today.tokens : 0) + this._tokenCount).toLocaleString();
        }
        // 更新字数进度条（带颜色分级）
        var wcProg = document.getElementById('pipeWCProgress');
        if (wcProg && Store.state.composer.targetWord > 0) {
          wcProg.style.display = '';
          var cur = Array.from(this.streamText || '').length;
          var target = Store.state.composer.targetWord;
          var pct = Math.min(100, Math.round(cur / target * 100));
          document.getElementById('pipeWCCount').textContent = cur.toLocaleString() + ' / ' + target.toLocaleString();
          var bar = document.getElementById('pipeWCBar');
          bar.style.width = pct + '%';
          bar.classList.remove('wc-low', 'wc-mid', 'wc-full', 'wc-over');
          bar.classList.add(pct < 40 ? 'wc-low' : pct < 80 ? 'wc-mid' : pct <= 100 ? 'wc-full' : 'wc-over');
        }
        break;
      case 'warning':
        var warnMsg = (ev.stage || ev.text || '') + ((ev.issues && ev.issues.length) ? (' · ' + ev.issues.length + ' 项问题') : '');
        PipelineUI.setWarn(warnMsg);
        if (ev.stage) PipelineUI.log('⚠ ' + (ev.stage || ev.text || ''));
        if (ev.issues && ev.issues.length) {
          Store.state.pipeline.issues = (Store.state.pipeline.issues || []).concat(ev.issues);
          PipelineUI.render();
        }
        if (ev.degraded && ev.text) {
          Store.state.pipeline.degraded = ev.text;
          PipelineUI.render();
        }
        // 第二层校验：大纲字数 mismatch（Thinker 产出后触发）
        if (ev.outline_words && ev.outline_words.mismatch) {
          var ow = ev.outline_words;
          PipelineUI.log('📏 大纲字数校验：建议 ' + ow.suggested_total + ' 字 vs 目标 ' + ow.target_word + ' 字（' + Math.round(ow.ratio * 100) + '%）');
          UI.toast('📏 ' + (ow.advice || '').slice(0, 80), 'warn');
        }
        break;
      case 'error':
        var roleLabel = (ROLE_META[ev.role] || {}).label || '';
        var errMsg = (roleLabel ? '[' + roleLabel + '] ' : '') + (ev.text || '生成失败');
        UI.toast(errMsg, 'error');
        // 展示重试提示
        var pipeCard = document.getElementById('pipeCard');
        if (pipeCard) {
          var retryBtn = document.getElementById('pipeRetry');
          if (!retryBtn) {
            var btn = document.createElement('button');
            btn.id = 'pipeRetry'; btn.className = 'btn btn-primary btn-sm';
            btn.textContent = '🔄 重试'; btn.style.cssText = 'margin-top:8px;width:100%';
            btn.onclick = function () { btn.remove(); Composer.generate(); };
            document.getElementById('page-pipeline').appendChild(btn);
          }
        }
        PipelineUI.setStage('生成失败', 'idle', 0);
        PipelineUI.setActive(false);
        break;
      case 'done':
        if (ev.final_text) { this.streamText = ev.final_text; }
    // 用快照ID写入，防止生成中切换章节导致串文
    var boundCh = Store.state.chapters.find(function(c){return c.id===SSE._bindChapterId;});
    if (boundCh) {
      boundCh.content = ev.final_text || '';
      API.updateChapter(boundCh.id, { content: ev.final_text }).catch(function(){});
    }
    document.getElementById('instructionInput').value = '';
    // 标记所有步骤完成
    Store.state.pipeline.steps.forEach(function (s) { s.status = 'done'; });
    PipelineUI.setStage('生成完成', 'idle', 100);
    PipelineUI.log('✓ 完成');
    PipelineUI.setActive(false, true);
    PipelineUI.render();
    var rb = document.getElementById('pipeRetry'); if (rb) rb.remove();
    // 字数校验
    if (ev.final_text && Store.state.composer.targetWord > 0) {
      var finalLen = Array.from(ev.final_text).length;
      var tw = Store.state.composer.targetWord;
      if (finalLen > tw * 1.3) {
        UI.toast('字数超出目标 ' + tw + ' 字 30%，已自动裁剪至最近句子边界', 'warn');
        // 按句子边界截断：找 targetWord 范围内最后一个句末符号
        var chars = Array.from(ev.final_text);
        var cutPos = tw;
        for (var i = cutPos - 1; i > tw * 0.6 && i >= 0; i--) {
          var c = chars[i];
          if (c === '。' || c === '！' || c === '？' || c === '…' || c === '\n' || c === '」' || c === '"') {
            cutPos = i + 1;
            break;
          }
        }
        var trimmed = chars.slice(0, cutPos).join('');
        boundCh.content = trimmed;
        Editor.setContent(trimmed);
      } else if (finalLen < tw * 0.7) {
        UI.toast('字数不足目标 70%，可点下方「补写」按钮', 'warn');
      }
    }
    // 自动追加：流式已追加到编辑器，仅同步状态
    if (Store.state.composer.autoAppend !== false) {
      Editor.undoContent = '';
      Editor.syncChapterContent();
      ProjectUI.updateMeta();
      ChapterUI.renderTree();
      // 自动追加模式：仅显示撤销+丢弃，2秒后自动消失
      var acts = document.getElementById('genActions');
      acts.innerHTML = '<span class="gen-act-hint">已自动追加（' + finalLen.toLocaleString() + '字）</span>' +
        '<button class="tool-btn undo-btn" onclick="Editor.undoGenerated()">↩ 撤销</button>' +
        '<button class="tool-btn" onclick="Editor.discardGenerated()">✕ 丢弃</button>';
      acts.style.display = '';
      setTimeout(function () { acts.style.display = 'none'; }, 3500);
    } else {
      document.getElementById('genActions').style.display = '';
    }
    // 补写按钮
    var actsEl = document.getElementById('genActions');
    var oldFill = actsEl.querySelector('.fillup-btn'); if (oldFill) oldFill.remove();
    var oldRetry = actsEl.querySelector('.retry-strategy-btn'); if (oldRetry) oldRetry.remove();
    if (Store.state.composer.targetWord > 0 && finalLen < Store.state.composer.targetWord * 0.7) {
      var fillUpBtn = document.createElement('button');
      fillUpBtn.className = 'tool-btn fillup-btn';
      fillUpBtn.textContent = '✚ 补写至 ' + Store.state.composer.targetWord + ' 字';
      fillUpBtn.onclick = function () { ChapterUI.fillUpToTarget(); };
      actsEl.appendChild(fillUpBtn);
    }
    // 换策略重试按钮
    var retryBtn = document.createElement('button');
    retryBtn.className = 'tool-btn retry-strategy-btn';
    retryBtn.textContent = '🔄 换策略重试';
    retryBtn.title = '使用不同创作模式重新生成';
    retryBtn.onclick = function () {
      var savedPayload = SSE._lastPayload;
      if (!savedPayload) return;
      // 弹出模式选择
      UI.modal({
        title: '换策略重试',
        body: '<div class="form-group"><label>选择新创作模式</label>' +
          '<select id="retryMode"><option value="collab">协同闭环（更高质量）</option><option value="strict"' + (savedPayload.run_mode === 'strict' ? ' selected' : '') + '>严谨模式</option><option value="art">文艺创作</option>' +
          '<option value="orchestrated">指派Agent模型</option><option value="draft">快速草稿</option></select></div>' +
          '<div class="form-group"><label>可修改创作需求</label><textarea id="retryDemand" rows="3">' + esc(savedPayload.user_demand || '') + '</textarea></div>',
        actions: [
          { id: 'cancel', label: '取消' },
          { id: 'ok', label: '开始重试', cls: 'btn-primary', onClick: function (m, ov) {
            var newMode = document.getElementById('retryMode').value;
            var newDemand = document.getElementById('retryDemand').value.trim();
            ov.remove();
            Store.state.composer.runMode = newMode;
            if (newDemand) savedPayload.user_demand = newDemand;
            savedPayload.run_mode = newMode;
            Composer.onModeChange(newMode);
            document.getElementById('instructionInput').value = savedPayload.user_demand;
            setTimeout(function () { Composer.generate(); }, 300);
          }}
        ]
      });
    };
    actsEl.appendChild(retryBtn);
    document.getElementById('genStatus').classList.remove('show');
    // 自动滚动 + 标题模式
    Editor.scrollToEnd();
    // 恢复编辑器焦点
    setTimeout(function () {
      if (Editor.tiptap && Editor.mode === 'rich') Editor.tiptap.commands.focus('end');
    }, 200);
    if (Store.state.composer._titleMode) {
      Store.state.composer._titleMode = false;
      if (ev.final_text) {
        var title = ev.final_text.replace(/[""''《》「」『』\n\r]/g, '').trim().substring(0, 20);
        if (boundCh && title) {
          boundCh.title = title;
          API.updateChapter(boundCh.id, { title: title }).then(function () {
            document.getElementById('docTitle').textContent = title;
            ChapterUI.renderTree();
            UI.toast('标题已更新：' + title, 'success');
          }).catch(function () {});
        }
      }
    }
    break;
    }
  },
  stop: function () {
    if (this.controller) {
      this.controller.abort();
      UI.toast('正在终止…', 'warn');
    }
  },
  finish: function () {
    this.active = false;
    this.controller = null;
    if (this._snapshotTimer) { clearInterval(this._snapshotTimer); this._snapshotTimer = null; }
    Composer.setGenerating(false);
    Editor.endStream();
    Usage.refresh();
    var sidebar = document.getElementById('sidebar');
    if (sidebar) sidebar.style.pointerEvents = '';
  }
};
