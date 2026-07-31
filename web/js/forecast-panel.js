/* ============ forecast-panel.js：叙事预测面板 ============ */
;(function () {
  var ForecastPanel = {
    branches: [],
    loading: false,
    lastNovelId: '',
    lastChapter: 0,

    render: function () {
      var el = document.getElementById('page-forecast');
      if (!el) return;
      var self = this;
      var pid = Store.state.currentProject ? String(Store.state.currentProject.id) : '';
      var chapter = Store.state.currentChapter ? Store.state.currentChapter.order || 1 : 1;

      el.innerHTML = '<div class="ghead">叙事预测 <span style="font-size:10px;color:var(--muted)">· 写前预览分支走向</span></div>';

      if (!pid) {
        el.innerHTML += '<div class="res-check-empty">请先选择作品</div>';
        return;
      }

      el.innerHTML +=
        '<div class="form-group"><label>当前进度</label><span style="font-size:11px;color:var(--accent)">第 ' + chapter + ' 章</span></div>' +
        '<div class="form-group"><label>分支数量</label><select id="fcBranchCount" style="width:100%;padding:4px">' +
        '<option value="2">2 条</option><option value="3" selected>3 条</option><option value="4">4 条</option><option value="5">5 条</option>' +
        '</select></div>' +
        '<div class="form-group"><label>分歧点描述（选填）</label>' +
        '<textarea id="fcDivergence" rows="2" style="width:100%;box-sizing:border-box;padding:6px;background:var(--panel3);border:1px solid var(--border);border-radius:6px;color:var(--text);font-size:11px;resize:vertical" placeholder="如：主角在岔路口选择..."></textarea></div>' +
        '<button class="btn btn-primary btn-block btn-sm" id="fcGenerateBtn" onclick="ForecastPanel.generate()">🔮 生成预测</button>' +
        '<div id="fcResult" style="margin-top:8px"></div>';
    },

    generate: function () {
      var self = this;
      var pid = Store.state.currentProject ? String(Store.state.currentProject.id) : '';
      var chapter = Store.state.currentChapter ? Store.state.currentChapter.order || 1 : 1;
      var bc = parseInt(document.getElementById('fcBranchCount').value) || 3;
      var divergence = (document.getElementById('fcDivergence') || {}).value || '';
      var memo = Store.state.pipeline.outline || '';
      var btn = document.getElementById('fcGenerateBtn');
      var resultEl = document.getElementById('fcResult');

      if (btn) { btn.disabled = true; btn.textContent = '⏳ 推演中...'; }
      resultEl.innerHTML = '<div style="text-align:center;padding:20px;color:var(--muted)">⏳ 正在推演分支...</div>';

      var currentState = '';
      try {
        var ch = Store.state.currentChapter;
        if (ch && ch.content) { currentState = ch.content.substring(0, 1500); }
      } catch (e) {}

      EinoAPI.getForecast(pid, chapter, bc).then(function (data) {
        if (!data || !data.branches || !data.branches.length) {
          EinoAPI.generateOutline('', 0).then(function () {}).catch(function () {});
          resultEl.innerHTML = '<div class="res-check-empty">推演失败，请确认后端服务已启用</div>';
          if (btn) { btn.disabled = false; btn.textContent = '🔮 生成预测'; }
          return;
        }
        self.branches = data.branches;
        self.lastNovelId = pid;
        self.lastChapter = chapter;
        self.renderBranches(data.branches, resultEl, data.forecastId);
        if (btn) { btn.disabled = false; btn.textContent = '🔮 重新推演'; }
      }).catch(function (e) {
        resultEl.innerHTML = '<div class="res-check-empty">推演失败: ' + (e && e.message || '网络错误') + '</div>';
        if (btn) { btn.disabled = false; btn.textContent = '🔮 生成预测'; }
      });
    },

    renderBranches: function (branches, containerEl, forecastId) {
      var html = '';
      var icons = ['🔴', '🟠', '🟡', '🟢', '🔵'];
      branches.forEach(function (b, i) {
        var label = b.label || '分支 ' + (i + 1);
        var direction = b.direction || b.premise || '';
        var beats = b.beats || '';
        var chars = b.characters || b.characterDecisions ? (b.characterDecisions || []).map(function (d) { return d.character + '：' + d.decision; }).join('；') : '';
        var risk = b.risk || '';
        var icon = icons[i] || '⚪';

        html +=
          '<div class="fc-branch" style="padding:10px 12px;border:1px solid var(--border);border-radius:8px;margin-bottom:8px;background:var(--panel2);cursor:pointer" onclick="ForecastPanel.selectBranch(\'' + (forecastId || '') + '\', ' + i + ')">' +
          '<div style="display:flex;align-items:center;gap:6px;margin-bottom:4px">' +
          '<span style="font-size:16px">' + icon + '</span>' +
          '<strong>' + esc(label) + '</strong>' +
          '<span style="font-size:10px;color:var(--muted);margin-left:auto">#' + (i + 1) + '</span>' +
          '</div>' +
          '<div style="font-size:10.5px;color:var(--text2);line-height:1.6;margin-bottom:4px">' + esc(direction) + '</div>';
        if (beats) {
          html += '<div style="font-size:10px;color:var(--muted);margin-bottom:2px">📖 ' + esc(beats) + '</div>';
        }
        if (chars) {
          html += '<div style="font-size:10px;color:var(--muted);margin-bottom:2px">👤 ' + esc(chars) + '</div>';
        }
        if (risk) {
          html += '<div style="font-size:10px;color:var(--danger);margin-bottom:2px">⚠ ' + esc(risk) + '</div>';
        }
        html += '</div>';
      });

      if (containerEl) { containerEl.innerHTML = html; }
    },

    selectBranch: function (forecastId, index) {
      var b = this.branches[index];
      if (!b) return;
      var label = b.label || '分支 ' + (index + 1);
      var direction = b.direction || b.premise || '';
      var beats = b.beats || '';

      UI.modal({
        title: '选中分支：' + label,
        body: '<div style="line-height:1.8">' +
          '<p><strong>走向</strong>：' + esc(direction) + '</p>' +
          (beats ? '<p><strong>节拍</strong>：' + esc(beats) + '</p>' : '') +
          '<p style="color:var(--muted);font-size:10px">点击"应用"将此分支添加到创作需求中</p>' +
          '</div>',
        actions: [
          { id: 'cancel', label: '取消' },
          { id: 'ok', label: '应用到大纲', cls: 'btn-primary', onClick: function (m, ov) {
            ov.remove();
            var outline = document.getElementById('genOutline');
            var inst = document.getElementById('instructionInput');
            var insertText = '【叙事预测分支：' + label + '】\n走向：' + direction + (beats ? '\n节拍：' + beats : '') + '\n';
            if (outline) { outline.value = (outline.value ? outline.value + '\n' : '') + insertText; }
            if (inst) { inst.value = insertText + (inst.value || ''); }
            UI.toast('已应用分支', 'success');
          }}
        ]
      });
    },

    autoAfterChapter: function () {
      var self = this;
      var pid = Store.state.currentProject ? String(Store.state.currentProject.id) : '';
      var chapter = Store.state.currentChapter ? Store.state.currentChapter.order || 1 : 1;
      if (!pid) return;

      var memo = Store.state.pipeline.outline || '';
      var currentState = '';
      try {
        var ch = Store.state.currentChapter;
        if (ch && ch.content) { currentState = ch.content.substring(0, 1500); }
      } catch (e) {}

      EinoAPI.createForecast(pid, chapter, currentState, memo, 3, '写完全章后的发展预测').then(function (data) {
        if (!data || !data.branches || !data.branches.length) return;
        self.branches = data.branches;
        self.lastNovelId = pid;
        self.lastChapter = chapter;

        var el = document.getElementById('page-forecast');
        if (el) {
          self.renderBranches(data.branches, document.getElementById('fcResult'), data.forecastId);
        }

        UI.toast('叙事预测已就绪（3条分支）→ 右侧「🔮」面板查看', 'success');
      }).catch(function () {});
    }
  };

  window.ForecastPanel = ForecastPanel;
})();
