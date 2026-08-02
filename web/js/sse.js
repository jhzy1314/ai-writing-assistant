/* ============ sse.js：SSE 流式生成与终止 ============ */
var SSE = {
  controller: null,
  active: false,
  streamText: '',
  // 主操作区生成进度条（通俗文案 + 阶段权重 + 已用/预计时间）
  _setProgress: function (label, weight, cur, target) {
    var el = document.getElementById('genProgress');
    if (!el) return;
    el.style.display = '';
    var stageEl = document.getElementById('genProgStage');
    var textEl = document.getElementById('genProgText');
    var barEl = document.getElementById('genProgBar');
    if (stageEl) stageEl.textContent = label || '';
    // 权重与字数双维度进度：阶段为主，字数在阶段区间内微调
    var pct = weight;
    if (target > 0 && cur > 0 && weight >= 20 && weight <= 75) {
      var span = 75 - 20;
      pct = 20 + Math.min(1, cur / target) * span;
    }
    pct = Math.max(2, Math.min(99, pct));
    if (barEl) barEl.style.width = pct + '%';
    if (textEl) {
      if (cur > 0 && target > 0) {
        textEl.textContent = cur.toLocaleString() + ' / ' + target.toLocaleString() + ' 字';
      } else {
        textEl.textContent = '';
      }
    }
    // 启动计时器：每秒刷新“已用/预计还需”
    this._stageWeight = pct;
    var self = this;
    if (this._timerInt) clearInterval(this._timerInt);
    this._startTs = this._startTs || Date.now();
    var upd = function () {
      var el2 = document.getElementById('genProgText');
      if (!el2) return;
      var used = Math.round((Date.now() - (self._startTs || Date.now())) / 1000);
      var usedTxt = self._fmtTime(used);
      var w = self._stageWeight || 20;
      var eta = w >= 98 ? 0 : Math.round(used / w * (100 - w));
      var base = (cur > 0 && target > 0) ? (cur.toLocaleString() + ' / ' + target.toLocaleString() + ' 字 · ') : '';
      var etaTxt = eta > 0 ? ('预计还需约 ' + self._fmtTime(eta)) : '即将完成…';
      el2.textContent = base + '已用 ' + usedTxt + ' · ' + etaTxt;
    };
    upd();
    this._timerInt = setInterval(upd, 1000);
  },
  _fmtTime: function (sec) {
    if (sec < 60) return sec + ' 秒';
    var m = Math.floor(sec / 60);
    var s = sec % 60;
    return m + ' 分 ' + (s < 10 ? '0' : '') + s + ' 秒';
  },
  _hideProgress: function () {
    var el = document.getElementById('genProgress');
    if (el) el.style.display = 'none';
    if (this._timerInt) { clearInterval(this._timerInt); this._timerInt = null; }
    this._startTs = null;
  },
  // 把后端阶段文案映射为通俗进度文案与权重
  _stageMeta: function (stage) {
    var s = stage || '';
    var m = s.match(/第\s*(\d+)\s*\/\s*(\d+)\s*段/);
    if (m) {
      var i = parseInt(m[1]), n = parseInt(m[2]);
      return { label: '✍️ 正在写作…（第 ' + i + '/' + n + ' 段）', weight: Math.round(20 + (i - 1) / n * 50 + 15 / n) };
    }
    if (s.indexOf('校验官') >= 0) {
      var vm = s.match(/第\s*(\d+)\s*轮/);
      return { label: '🔍 审稿中' + (vm ? '（第 ' + vm[1] + ' 轮）' : '…'), weight: 78 };
    }
    if (s.indexOf('微调') >= 0) { return { label: '🔄 根据审稿意见修改中…', weight: 88 }; }
    if (s.indexOf('规划') >= 0 || s.indexOf('框架') >= 0) { return { label: '📋 规划故事框架中…', weight: 12 }; }
    if (s.indexOf('撰写') >= 0) { return { label: '✍️ 正在写作…', weight: 40 }; }
    if (s.indexOf('校验通过') >= 0) { return { label: '✅ 审稿通过', weight: 92 }; }
    if (s.indexOf('补写') >= 0) { return { label: '➕ 补写中…', weight: 90 }; }
    return { label: '⏳ ' + (s || '处理中…'), weight: 30 };
  },
  // 超长文本自动拆分多章：优先按后端分章标记（模型情节段）拆，无标记则按句子边界切分（仅自动追加模式）
  splitIntoChapters: function (fullText, boundCh, tw) {
    var parts;
    var marker = '[=====AI-NOVEL-CHAPTER-BREAK=====]';
    if (fullText.indexOf(marker) >= 0) {
      // 后端分段生成时已在段间插入标记：直接按标记拆，清理标记与首尾空行
      parts = fullText.split(marker).map(function (s) { return s.replace(/^\n+|\n+$/g, ''); }).filter(function (s) { return s.trim(); });
    } else {
      // 无标记：按目标字数的 1.1 倍为一段，在句子边界处切分
      var chars = Array.from(fullText);
      parts = [];
      var pos = 0;
      var minLen = Math.max(50, Math.floor(tw * 0.6));
      var maxLen = Math.max(200, Math.ceil(tw * 1.1));
      while (chars.length - pos > maxLen) {
        var end = Math.min(pos + maxLen, chars.length);
        var cut = end;
        for (var i = end - 1; i > pos + minLen && i >= 0; i--) {
          var c = chars[i];
          if (c === '。' || c === '！' || c === '？' || c === '…' || c === '\n' || c === '；' || c === '」' || c === '"') {
            cut = i + 1;
            break;
          }
        }
        parts.push(chars.slice(pos, cut).join(''));
        pos = cut;
      }
      parts.push(chars.slice(pos).join(''));
    }
    // 当前章节 = 第一部分（同步写入，保证与后续自动追加一致）
    if (boundCh) {
      boundCh.content = parts[0];
      API.updateChapter(boundCh.id, { content: parts[0] }).catch(function () {});
    }
    Editor.setContent(parts[0]);
    // 其余部分异步建新章
    var extra = parts.slice(1);
    if (extra.length) {
      var p = Store.state.currentProject;
      var num = (Store.state.chapters || []).length + 1;
      var chain = Promise.resolve();
      extra.forEach(function (content) {
        chain = chain.then(function () {
          return API.createChapter({ project_id: p.id, volume_id: '', title: '第' + num + '章', content: content }).then(function (ch) {
            num++;
            if (ch) { Store.state.chapters = Store.state.chapters || []; Store.state.chapters.push(ch); }
          });
        });
      });
      chain.then(function () {
        ChapterUI.renderTree();
        ProjectUI.updateMeta();
        UI.toast('已自动拆分为 ' + parts.length + ' 章（当前章 ' + Array.from(parts[0]).length.toLocaleString() + ' 字）', 'success');
      }).catch(function (e) {
        UI.toast('自动分章失败：' + (e && e.message || '未知错误'), 'error');
      });
    } else {
      UI.toast('字数超出目标，已按句子边界整理', 'warn');
    }
  },
  start: async function (payload) {
    if (this.active) { UI.toast('已有生成任务进行中', 'warn'); return; }
    this.active = true;
    this.streamText = '';
    this._tokenCount = 0;
    this._snapshotCount = 0;
    // 生成前保存编辑器当前内容作为撤销快照
    if (Editor) Editor.undoContent = Editor.getText();
    // 快照绑定：防止生成中切换项目/章节导致串文
    this._bindChapterId = (Store.state.currentChapter || {}).id || '';
    this._bindProjectId = (Store.state.currentProject || {}).id || '';
    // 保存 payload 用于断连重试
    this._lastPayload = payload;
    this._retryCount = 0;
    this.controller = new AbortController();
    Composer.setGenerating(true);
    PipelineUI.reset();
    this._setProgress('⏳ 准备中…', 3, 0, payload.target_word || 0);
    // 自动快照：每 90 秒保存一次当前进度（新章模式下跳过，避免污染当前章节）
    this._snapshotTimer = setInterval(function () {
      if (!SSE.active || !SSE._bindChapterId || !SSE.streamText) return;
      if (Store.state.composer && Store.state.composer.newChapterWrite) return;
      var ch = Store.state.chapters.find(function (c) { return c.id === SSE._bindChapterId; });
      if (!ch) return;
      var snapTitle = (ch.title || '未命名') + ' · 自动快照 ' + new Date().toLocaleTimeString('zh-CN', { hour12: false });
      API.saveChapterVersion(ch.id, { title: snapTitle, content: SSE.streamText }).then(function () {
        SSE._snapshotCount++;
        PipelineUI.log('📸 快照 #' + SSE._snapshotCount + ' 已保存（' + Array.from(SSE.streamText || '').length.toLocaleString() + ' 字）');
        document.getElementById('pipeSnapshotCount').textContent = SSE._snapshotCount;
        document.getElementById('pipeSnapshotBadge').style.display = '';
      }).catch(function (e) { console.warn('[sse] snapshot save failed:', e && e.message); });
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
      // 新章模式下断连不覆盖当前章节（重试会继续累积）
      if (boundCh && this.streamText && !(Store.state.composer && Store.state.composer.newChapterWrite)) {
        API.updateChapter(boundCh.id, { content: this.streamText }).catch(function(e){ console.warn('[sse] disconnect save failed:', e && e.message); });
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
  handle: async function (ev) {
    switch (ev.type) {
      case 'estimate':
        PipelineUI.setStage('预估 Token…', 'idle', 8);
        PipelineUI.log('预估 Token：' + (ev.tokens || 0));
        this._setProgress('⏳ 准备中…', 4, 0, Store.state.composer.targetWord || 0);
        break;
      case 'plan':
        PipelineUI.setStage(ev.stage || '流水线已规划', 'idle', 15);
        PipelineUI.log('执行计划：' + (ev.stage || ''));
        this._setProgress('📋 规划中…', 8, 0, Store.state.composer.targetWord || 0);
        if (Store.state.composer.runMode === 'auto') {
          UI.toast('智能协同 → 已选择「' + (ev.stage || ev.pipeline || '') + '」', '');
        }
        break;
      case 'stage':
        PipelineUI.applyStage(ev);
        var meta = this._stageMeta(ev.stage || '');
        this._setProgress(meta.label, meta.weight, 0, Store.state.composer.targetWord || 0);
        break;
      case 'token':
        if (ev.reset) { this.streamText = ''; this._tokenCount = 0; Editor.resetStream(); }
        if (ev.text) {
          this.streamText += ev.text;
          // 「新章」模式：流式内容不写入当前章节编辑器，仅缓存到 streamText，done 时统一建新章
          if (!(Store.state.composer && Store.state.composer.newChapterWrite)) {
            Editor.appendStream(ev.text);
          }
          this._tokenCount = (this._tokenCount || 0) + Math.ceil(Array.from(ev.text).length / 1.5);
          // 实时更新配额条
          document.getElementById('qTokens').textContent = ((Store.state.usage && Store.state.usage.today ? Store.state.usage.today.tokens : 0) + this._tokenCount).toLocaleString();
        }
        // 更新字数进度条（带颜色分级）
                // 更新字数进度条（节流 300ms，避免每次 token 全量 Array.from）
        var wcProg = document.getElementById('pipeWCProgress');
        var tw2 = Store.state.composer.targetWord || 0;
        if (wcProg && tw2 > 0 && !this._wcProgTimer) {
          var self3 = this;
          this._wcProgTimer = setTimeout(function () {
            self3._wcProgTimer = null;
            var cur = self3.streamText ? self3.streamText.length : 0;
            wcProg.style.display = '';
            var target = tw2;
            var pct = Math.min(100, Math.round(cur / target * 100));
            var cnt = document.getElementById('pipeWCCount');
            if (cnt) cnt.textContent = cur.toLocaleString() + ' / ' + target.toLocaleString();
            var bar = document.getElementById('pipeWCBar');
            if (bar) {
              bar.style.width = pct + '%';
              bar.classList.remove('wc-low', 'wc-mid', 'wc-full', 'wc-over');
              bar.classList.add(pct < 40 ? 'wc-low' : pct < 80 ? 'wc-mid' : pct <= 100 ? 'wc-full' : 'wc-over');
            }
          }, 300);
        }
        // 主操作区进度条同步（写作阶段，字数区间 20%~75% 内微调）
        if (tw2 > 0) {
          var meta2 = this._stageMeta(Store.state.pipeline.stage || '');
          if (meta2.label.indexOf('写作') >= 0) {
            this._setProgress(meta2.label, 40, this.streamText ? this.streamText.length : 0, tw2);
          }
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
          UI.toast('⚠️ 模型降级：' + ev.text, 'warn');
        }
        // 第二层校验：大纲字数 mismatch（Thinker 产出后触发）
        if (ev.outline_words && ev.outline_words.mismatch) {
          var ow = ev.outline_words;
          PipelineUI.log('📏 大纲字数校验：建议 ' + ow.suggested_total + ' 字 vs 目标 ' + ow.target_word + ' 字（' + Math.round(ow.ratio * 100) + '%）');
          UI.toast('📏 ' + (ow.advice || '').slice(0, 80), 'warn');
        }
        break;
      case 'error':
        this._hideProgress();
        var roleLabel = (ROLE_META[ev.role] || {}).label || '';
        var errMsg = (roleLabel ? '[' + roleLabel + '] ' : '') + (ev.text || '生成失败');
        UI.toast(errMsg, 'error');
        // 修复：即使链路报错，已流式生成的正文也不能丢——立即保存到绑定章节并同步编辑器
        var boundCh2 = Store.state.chapters.find(function(c){return c.id===SSE._bindChapterId;});
        if (boundCh2 && SSE.streamText && SSE.streamText.trim() && !(Store.state.composer && Store.state.composer.newChapterWrite)) {
          var savedLen = Array.from(SSE.streamText).length;
          boundCh2.content = SSE.streamText;
          API.updateChapter(boundCh2.id, { content: SSE.streamText }).catch(function(){});
          Editor.syncChapterContent && Editor.syncChapterContent();
          ProjectUI.updateMeta && ProjectUI.updateMeta();
          ChapterUI.renderTree && ChapterUI.renderTree();
          UI.toast('已生成 ' + savedLen.toLocaleString() + ' 字并自动保存（校验环节异常）', 'warn');
        }
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
        this._hideProgress();
        if (ev.final_text) { this.streamText = ev.final_text; }
    // 用快照ID写入，防止生成中切换章节导致串文
    var boundCh = Store.state.chapters.find(function(c){return c.id===SSE._bindChapterId;});
    // 「新章」模式：生成内容写入新建的下一章节，当前章节保持不动
    if (Store.state.composer && Store.state.composer.newChapterWrite && ev.final_text) {
      var p = Store.state.currentProject;
      if (p) {
        var lastCh = Store.state.chapters && Store.state.chapters.length ? Store.state.chapters[Store.state.chapters.length - 1] : null;
        var newVol = boundCh ? boundCh.volume_id : '';
        var num = (Store.state.chapters || []).length + 1;
        var newTitle = '第' + num + '章';
        try {
          var newCh = await API.createChapter({ project_id: p.id, volume_id: newVol, title: newTitle, content: ev.final_text });
          if (newCh) {
            Store.state.chapters = Store.state.chapters || [];
            Store.state.chapters.push(newCh);
            Store.state.currentChapter = newCh;
            Editor.setContent(ev.final_text);
            ChapterUI.renderTree();
            ProjectUI.updateMeta();
            SSE._bindChapterId = newCh.id;
            UI.toast('已写入新章节「' + newTitle + '」（' + Array.from(ev.final_text).length.toLocaleString() + '字）', 'success');
            // 新章模式下跳过下面的覆盖/追加逻辑
            document.getElementById('instructionInput').value = '';
            Store.state.pipeline.steps.forEach(function (s) { s.status = 'done'; });
            PipelineUI.setStage('生成完成', 'idle', 100);
            PipelineUI.log('✓ 完成（新章节）');
            PipelineUI.setActive(false, true);
            PipelineUI.render();
            var rb2 = document.getElementById('pipeRetry'); if (rb2) rb2.remove();
            ProjectUI.updateMeta();
            ChapterUI.renderTree();
            return;
          }
        } catch (e) {
          UI.toast('创建新章节失败：' + e.message + '，已退回写入当前章节', 'warn');
        }
      }
    }
    // 修复：final_text 为空时不覆盖章节已有内容（防止后端异常把正文清空）
    if (boundCh && ev.final_text) {
      boundCh.content = ev.final_text;
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
    // 字数校验/自动分章：有分章标记，或超出目标 30%（默认自动追加模式），自动拆分多章；否则裁剪
    if (ev.final_text && Store.state.composer.targetWord > 0) {
      var finalLen = Array.from(ev.final_text).length;
      var tw = Store.state.composer.targetWord;
      var hasBreak = ev.final_text.indexOf('[=====AI-NOVEL-CHAPTER-BREAK=====]') >= 0;
      if (hasBreak || finalLen > tw * 1.3) {
        if (Store.state.composer.autoAppend !== false) {
          this.splitIntoChapters(ev.final_text, boundCh, tw);
        } else {
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
        }
      } else if (finalLen < tw * 0.7) {
        UI.toast('字数不足目标 70%，可点下方「补写」按钮', 'warn');
      }
    }
    // 自动追加：流式已追加到编辑器，仅同步状态
    if (Store.state.composer.autoAppend !== false) {
      Editor.syncChapterContent();
      // 自动追加模式：流式内容无感写入，仅显示撤销+丢弃，清除 streamText 防重复追加
      this.streamText = '';
      var acts = document.getElementById('genActions');
      acts.innerHTML = '<span class="gen-act-hint">已自动追加（' + finalLen.toLocaleString() + '字）</span>' +
        '<button class="tool-btn undo-btn" onclick="Editor.undoGenerated()">↩ 撤销</button>' +
        '<button class="tool-btn" onclick="Editor.discardGenerated()">✕ 丢弃</button>';
      acts.style.display = '';
      setTimeout(function () { acts.style.display = 'none'; }, 3500);
    } else {
      document.getElementById('genActions').style.display = '';
    }
    // Refresh sidebar and chapter tree
    ProjectUI.updateMeta();
    ChapterUI.renderTree();

    // Trigger Eino: auto-forecast after chapter completion (async, fire-and-forget)
    if (typeof ForecastPanel !== 'undefined' && Store.state.composer.runMode !== 'draft') {
      setTimeout(function () { ForecastPanel.autoAfterChapter(); }, 800);
    }
    // Auto AIGC detection after chapter completion (silent, just toast if issues)
    if (typeof EinoAPI !== 'undefined' && ev.final_text && ev.final_text.trim()) {
      setTimeout(function () {
        EinoAPI.detectAIGC(ev.final_text).then(function (r) {
          if (r && (r.aiProbability || r.aiProbability === 0) && r.aiProbability >= 0.4) {
            UI.toast('⚠ AI 检测: ' + Math.round(r.aiProbability * 100) + '% — 右侧🔎查看详情', 'warn');
          }
        }).catch(function () {});
      }, 1500);
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
    this._hideProgress();
    if (this._snapshotTimer) { clearInterval(this._snapshotTimer); this._snapshotTimer = null; }
    Composer.setGenerating(false);
    Editor.endStream();
    // 兜底：异常终止（用户点终止/网络中断）时保存已流式生成的内容，防止刷新后丢失
    // 正常 done 路径：autoAppend 已清空 streamText；非 autoAppend 时 done 已把 final_text 写入章节，这里幂等跳过
    var ch = Store.state.chapters.find(function (c) { return c.id === SSE._bindChapterId; });
    if (ch && this.streamText && this.streamText.trim() && (ch.content || '') !== this.streamText) {
      var savedLen = Array.from(this.streamText).length;
      ch.content = this.streamText;
      API.updateChapter(ch.id, { content: this.streamText }).catch(function () {});
      ChapterUI.renderTree && ChapterUI.renderTree();
      UI.toast('已保存已生成的 ' + savedLen.toLocaleString() + ' 字（生成被终止）', 'warn');
    }
    Usage.refresh();
    var sidebar = document.getElementById('sidebar');
    if (sidebar) sidebar.style.pointerEvents = '';
  }
};
