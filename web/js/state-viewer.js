/* ============ state-viewer.js：运行时状态仪表盘 ============ */
;(function () {
  var StateViewer = {
    loading: false,

    render: function () {
      var el = document.getElementById('page-state');
      if (!el) return;
      el.innerHTML = '<div class="ghead">状态仪表盘 <span style="font-size:10px;color:var(--muted)">· facts / hooks / summaries</span></div>' +
        '<button class="btn btn-primary btn-block btn-sm" onclick="StateViewer.load()">🔄 刷新状态</button>' +
        '<div id="stDashboard" style="margin-top:8px"><div class="res-check-empty">点击刷新加载</div></div>';
      this.load();
    },

    load: function () {
      var self = this;
      var pid = Store.state.currentProject ? String(Store.state.currentProject.id) : '';
      if (!pid) {
        var d = document.getElementById('stDashboard');
        if (d) d.innerHTML = '<div class="res-check-empty">请先选择作品</div>';
        return;
      }
      var d = document.getElementById('stDashboard');
      if (d) d.innerHTML = '<div style="text-align:center;padding:20px;color:var(--muted)">⏳ 加载中...</div>';

      EinoAPI.getStateJSON(pid).then(function (data) {
        self.renderDashboard(data);
      }).catch(function () {
        EinoAPI.getRuntimeState(pid).then(function (text) {
          var d2 = document.getElementById('stDashboard');
          if (d2) d2.innerHTML = '<pre style="font-size:10px;line-height:1.6;max-height:400px;overflow:auto;white-space:pre-wrap">' + esc(text || '无状态数据') + '</pre>';
        }).catch(function () {
          var d3 = document.getElementById('stDashboard');
          if (d3) d3.innerHTML = '<div class="res-check-empty">加载失败，确认后端 8082 端口已启动</div>';
        });
      });
    },

    renderDashboard: function (data) {
      var d = document.getElementById('stDashboard');
      if (!d) return;
      if (!data) { d.innerHTML = '<div class="res-check-empty">暂无状态数据</div>'; return; }

      var html = '';

      var facts = (data.currentState && data.currentState.facts) || [];
      var hooks = (data.hooks && data.hooks.hooks) || [];
      var summaries = (data.chapterSummaries && data.chapterSummaries.rows) || (data.summaries && data.summaries.rows) || [];

      html += '<div class="fc-section" style="border-top:1px solid var(--border);padding-top:8px;margin-top:8px">' +
        '<div class="ghead" style="margin-bottom:4px">📊 当前事实 <span style="font-weight:400;font-size:10px;color:var(--muted)">(' + facts.length + '条)</span></div>';
      if (facts.length === 0) {
        html += '<div class="res-check-empty">暂无记录</div>';
      } else {
        html += '<table style="width:100%;font-size:10px;border-collapse:collapse"><tr style="color:var(--muted);border-bottom:1px solid var(--border)"><th style="text-align:left;padding:3px 4px">主语</th><th style="text-align:left;padding:3px 4px">谓词</th><th style="text-align:left;padding:3px 4px">宾语</th><th style="text-align:right;padding:3px 4px;white-space:nowrap">始→终章</th></tr>';
        facts.slice(-15).forEach(function (f) {
          var until = f.validUntilChapter ? f.validUntilChapter : '至今';
          html += '<tr style="border-bottom:1px solid var(--panel3)">' +
            '<td style="padding:3px 4px">' + esc(f.subject || '') + '</td>' +
            '<td style="padding:3px 4px;color:var(--accent)">' + esc(f.predicate || '') + '</td>' +
            '<td style="padding:3px 4px">' + esc((f.object || '').substring(0, 40)) + '</td>' +
            '<td style="padding:3px 4px;text-align:right;color:var(--muted);white-space:nowrap">' + esc(f.validFromChapter || '') + '→' + esc(until) + '</td>' +
            '</tr>';
        });
        html += '</table>';
      }
      html += '</div>';

      html += '<div class="fc-section" style="border-top:1px solid var(--border);padding-top:8px;margin-top:8px">' +
        '<div class="ghead" style="margin-bottom:4px">🎣 伏笔追踪 <span style="font-weight:400;font-size:10px;color:var(--muted)">(' + hooks.length + '条)</span></div>';
      if (hooks.length === 0) {
        html += '<div class="res-check-empty">暂无伏笔</div>';
      } else {
        hooks.forEach(function (h) {
          var statusColor = '#6ee7b7';
          var statusLabel = h.status || 'open';
          if (statusLabel === 'open') { statusColor = '#6ee7b7'; }
          else if (statusLabel === 'progressing') { statusColor = '#facc15'; }
          else if (statusLabel === 'deferred') { statusColor = '#fb923c'; }
          else if (statusLabel === 'resolved') { statusColor = '#9ca3af'; }

          html += '<div style="padding:8px 10px;border-left:3px solid ' + statusColor + ';margin-bottom:6px;background:var(--panel2);border-radius:0 6px 6px 0">' +
            '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:2px">' +
            '<strong style="font-size:11px">' + esc(h.hookId || '') + '</strong>' +
            '<span style="font-size:9px;padding:1px 6px;border-radius:3px;background:' + statusColor + '20;color:' + statusColor + '">' + esc(statusLabel) + '</span>' +
            '</div>' +
            '<div style="font-size:10px;color:var(--text2)">' + esc((h.expectedPayoff || h.notes || '').substring(0, 80)) + '</div>' +
            '<div style="font-size:9px;color:var(--muted);margin-top:2px">始于第' + esc(h.startChapter || '?') + '章 · 推进 ' + esc(h.advancedCount || 0) + '次' + (h.coreHook ? ' · 核心钩子' : '') + '</div>' +
            '</div>';
        });
      }
      html += '</div>';

      if (summaries.length > 0) {
        html += '<div class="fc-section" style="border-top:1px solid var(--border);padding-top:8px;margin-top:8px">' +
          '<div class="ghead" style="margin-bottom:4px">📝 章节摘要 <span style="font-weight:400;font-size:10px;color:var(--muted)">(' + summaries.length + '章)</span></div>';
        summaries.slice(-5).forEach(function (s) {
          html += '<div style="padding:6px 8px;margin-bottom:4px;background:var(--panel2);border-radius:6px;font-size:10px">' +
            '<strong>第' + esc(s.chapter || '?') + '章</strong> ' + esc((s.title || '').substring(0, 30)) +
            '<div style="color:var(--muted);margin-top:2px">' +
            (s.characters && s.characters.length ? '👤 ' + esc(s.characters.join('、').substring(0, 60)) : '') +
            (s.events && s.events.length ? '<br>📖 ' + esc(s.events.join('；').substring(0, 80)) : '') +
            (s.mood ? '<br>🎭 ' + esc(s.mood) : '') +
            '</div></div>';
        });
        html += '</div>';
      }

      d.innerHTML = html;
    }
  };

  window.StateViewer = StateViewer;
})();
