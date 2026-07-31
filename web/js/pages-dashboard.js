/* ============ pages-dashboard.js：创作数据仪表盘 ============ */
var DashboardPage = {
  init: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    this.refresh();
  },
  refresh: async function () {
    var p = Store.state.currentProject;
    if (!p) {
      document.getElementById('dashCards').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">📊</div><div>请先在左侧选中一个项目</div></div>';
      return;
    }
    try {
      var stats = await API.getProjectStats(p.id);
      var usage = null;
      try { usage = await API.getUsage(); } catch (e) {}
      this.renderStats(stats, usage);
      this.renderChart(stats);
      this.renderLog(p);
      var now = new Date();
      document.getElementById('dashLastUpdate').textContent = '更新于 ' + now.toLocaleTimeString();
    } catch (e) {
      UI.toast('加载仪表盘失败: ' + e.message, 'error');
    }
  },
  renderStats: function (stats, usage) {
    var totalWords = 0, chCount = 0, volCount = 0;
    if (stats) {
      totalWords = stats.total_words || 0;
      // 后端字段是 total_chapters / volumes（历史上前端误读 chapter_count/volume_count，导致恒为 0）
      // volumes 里的"未分类"是伪卷（volume_id 为空），不计入真实卷数
      chCount = stats.total_chapters || 0;
      volCount = (stats.volumes || []).filter(function (v) { return v.volume_id; }).length;
    }
    var chars = Store.state.characters || [];
    var worlds = Store.state.worldSettings || [];
    document.getElementById('dashTotalWords').textContent = totalWords.toLocaleString();
    document.getElementById('dashChapters').textContent = volCount + '卷 / ' + chCount + '章';
    document.getElementById('dashCharacters').textContent = chars.length;
    document.getElementById('dashWorldSettings').textContent = worlds.length;
    if (usage) {
      // 后端 /api/usage 返回 today:{calls,tokens}（历史上前端误读 call_count/token_count，导致恒为 0）
      var today = usage.today || {};
      document.getElementById('dashCalls').textContent = (today.calls || 0).toLocaleString();
      document.getElementById('dashTokens').textContent = (today.tokens || 0).toLocaleString();
    }
  },
  renderChart: function (stats) {
    var container = document.getElementById('dashBarChart');
    // 后端 stats 无 chapter_word_counts 字段时，回退用本地已加载的章节数据
    var counts = (stats && stats.chapter_word_counts) || (Store.state.chapters || []).map(function (c) {
      return { title: c.title, words: c.word_count || 0 };
    });
    if (!counts || !counts.length) {
      container.innerHTML = '<div class="page-empty-state" style="padding:20px"><div style="font-size:14px">暂无写作数据</div></div>';
      return;
    }
    var max = Math.max.apply(null, counts.map(function (c) { return c.words || 0; }));
    if (max === 0) max = 1;
    var html = '<div class="bar-chart-bars">';
    counts.forEach(function (c, i) {
      var pct = Math.round(((c.words || 0) / max) * 100);
      var h = Math.max(4, pct);
      html += '<div class="bar-col">';
      html += '<div class="bar-fill" style="height:' + h + '%" title="' + escAttr(c.title || '第' + (i + 1) + '章') + ': ' + (c.words || 0) + ' 字"></div>';
      html += '<div class="bar-label">' + (i + 1) + '</div>';
      html += '</div>';
    });
    html += '</div>';
    html += '<div class="bar-chart-legend">';
    html += '<span>最少: ' + Math.min.apply(null, counts.map(function (c) { return c.words || 0; })) + ' 字</span>';
    html += '<span>最多: ' + max.toLocaleString() + ' 字</span>';
    html += '<span>平均: ' + Math.round(counts.reduce(function (s, c) { return s + (c.words || 0); }, 0) / counts.length).toLocaleString() + ' 字</span>';
    html += '</div>';
    container.innerHTML = html;
  },
  renderLog: function (p) {
    var container = document.getElementById('dashLogList');
    var chs = Store.state.chapters || [];
    if (!chs.length) {
      container.innerHTML = '<div class="page-empty-state" style="padding:30px"><div class="page-empty-icon" style="font-size:24px">📋</div><div>暂无最近活动记录</div></div>';
      return;
    }
    var recent = chs.slice(-10).reverse();
    var html = '';
    recent.forEach(function (ch) {
      html += '<div class="log-item">';
      html += '<span class="log-icon">📄</span>';
      html += '<span class="log-title">' + esc(ch.title || '未命名章节') + '</span>';
      html += '<span class="log-meta">' + (ch.word_count || 0) + ' 字</span>';
      html += '<span class="log-time">' + (ch.updated_at ? new Date(ch.updated_at).toLocaleDateString() : '') + '</span>';
      html += '</div>';
    });
    container.innerHTML = html;
  }
};
