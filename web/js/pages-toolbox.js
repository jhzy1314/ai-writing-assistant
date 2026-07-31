/* ============ pages-toolbox.js：AI工具箱页面 ============ */
var ToolboxPage = {
  currentCat: 'all',
  tools: [
    { id: 'generate', name: 'AI 智能生成', icon: '✨', cat: 'generate', desc: '根据创意需求AI写作续写', fn: 'Composer.generate' },
    { id: 'continue', name: '从光标续写', icon: '▶', cat: 'generate', desc: '从当前光标位置开始续写', fn: 'Composer.continueFromCursor' },
    { id: 'expand', name: '选中扩写', icon: '📝', cat: 'generate', desc: '扩写选中的文本段落', fn: 'SelectionActions.expand' },
    { id: 'condense', name: '选中缩写', icon: '📋', cat: 'generate', desc: '精简选中的文本段落', fn: 'SelectionActions.condense' },
    { id: 'rewrite', name: '改写文风', icon: '🎨', cat: 'generate', desc: '改变选中文本的文风', fn: 'SelectionActions.rewrite' },
    { id: 'polish', name: '润色优化', icon: '✨', cat: 'generate', desc: '润色选中的文本', fn: 'SelectionActions.polish' },
    { id: 'summary', name: '生成摘要', icon: '📄', cat: 'generate', desc: '对选中文本生成摘要', fn: 'SelectionActions.summary' },
    { id: 'atmosphere', name: '调整氛围', icon: '🎭', cat: 'generate', desc: '调整选中文段的氛围', fn: 'SelectionActions.atmosphere' },
    { id: 'autotitle', name: 'AI 拟标题', icon: '💡', cat: 'generate', desc: 'AI 自动生成标题', fn: 'Composer.autoTitle' },
    { id: 'proofread', name: '文字校对', icon: '🔍', cat: 'proofread', desc: '检测错别字/的地得/标点', fn: 'Tools.proofreadText' },
    { id: 'verify', name: '逻辑自检', icon: '✅', cat: 'proofread', desc: '一键逻辑一致性检查', fn: 'Tools.verify' },
    { id: 'verifyFull', name: '全书逐章校对', icon: '📖', cat: 'proofread', desc: '对全书所有章节进行校对', fn: 'Tools.verifyFullBook' },
    { id: 'crossAudit', name: '跨章一致性审计', icon: '📋', cat: 'proofread', desc: '检查跨章节设定一致性', fn: 'Tools.verifyCrossChapter' },
    { id: 'detectAIGC', name: 'AI 味检测', icon: '🔎', cat: 'proofread', desc: '检测章节的AI生成痕迹', fn: 'Tools.detectAIGC' },
    { id: 'cleanAIFiller', name: '净化AI话术', icon: '🧹', cat: 'proofread', desc: '清理AI生成的冗余话术', fn: 'Editor.cleanAIFiller' },
    { id: 'wordFreq', name: '高频词汇分析', icon: '📊', cat: 'analyze', desc: '统计高频词与用词习惯', fn: 'Tools.wordFrequency' },
    { id: 'wordCount', name: '字数统计', icon: '🔢', cat: 'analyze', desc: '详细统计全文字数分布', fn: 'Tools.countWords' },
    { id: 'styleGenome', name: '风格基因组', icon: '🧬', cat: 'analyze', desc: '分析写作风格特征', fn: 'Tools.analyzeStyle' },
    { id: 'styleRadar', name: '写作基因组', icon: '🎯', cat: 'analyze', desc: '多维度写作能力雷达图', fn: 'Tools.styleRadar' },
    { id: 'quickRecap', name: '前情提要', icon: '📋', cat: 'analyze', desc: '自动生成前情提要', fn: 'Tools.quickRecap' },
    { id: 'readingStats', name: '阅读统计', icon: '📖', cat: 'analyze', desc: '阅读时间与进度统计', fn: 'Tools.showReadingStats' },
    { id: 'batchSummary', name: '批量摘要', icon: '📑', cat: 'analyze', desc: '批量生成所有章节摘要', fn: 'Tools.batchSummarize' },
    { id: 'extractChars', name: '自动提取设定', icon: '👥', cat: 'analyze', desc: '从正文自动提取人物&世界观', fn: 'Tools.extractCharacters' },
    { id: 'serialDashboard', name: '连载管理', icon: '📅', cat: 'analyze', desc: '连载计划与进度追踪', fn: 'Tools.serialDashboard' },
    { id: 'cleanText', name: 'AI文本清洗', icon: '🧹', cat: 'convert', desc: '清理格式混乱的文本', fn: 'Tools.cleanText' },
    { id: 'convertFormat', name: '格式转换', icon: '🔄', cat: 'convert', desc: '转换文本格式', fn: 'Tools.convertFormat' },
    { id: 'mdToRich', name: 'MD→富文本', icon: '📝', cat: 'convert', desc: 'Markdown转富文本', fn: 'Tools.mdToRich' },
    { id: 'richToMd', name: '富文本→MD', icon: '⇥', cat: 'convert', desc: '富文本转Markdown', fn: 'Tools.richToMd' },
    { id: 'sortChapters', name: '章节排序', icon: '📑', cat: 'convert', desc: '智能排序章节', fn: 'Tools.sortChapters' },
    { id: 'extractMaterial', name: '素材提取', icon: '🔎', cat: 'convert', desc: '从正文提取素材', fn: 'Tools.extractMaterial' },
    { id: 'startRecord', name: '语音录音', icon: '🎤', cat: 'voice', desc: '开始录音并转文字', fn: 'Tools.startRecord' },
    { id: 'speakText', name: '语音朗读', icon: '🎧', cat: 'voice', desc: '朗读当前章节内容', fn: 'Tools.speakText' },
    { id: 'versionHistory', name: '版本历史', icon: '📜', cat: 'proofread', desc: '查看文档版本变更', fn: 'VersionUI.open' },
    { id: 'trash', name: '回收站', icon: '🗑', cat: 'proofread', desc: '恢复已删除的章节', fn: 'Tools.showTrash' },
    { id: 'ragMemory', name: 'RAG记忆检索', icon: '🧠', cat: 'analyze', desc: '全书记忆索引与检索', fn: 'Tools.ragMemory' },
    { id: 'abCompare', name: 'A/B双稿对比', icon: '⚖', cat: 'analyze', desc: '两个版本对比差异', fn: 'Tools.abCompare' },
    { id: 'globalSearch', name: '全局搜索', icon: '🌐', cat: 'analyze', desc: '跨项目搜索所有内容', fn: 'Tools.searchAllProjects' },
    { id: 'achievements', name: '写作成就', icon: '🏆', cat: 'analyze', desc: '查看写作里程碑成就', fn: 'Tools.showAchievements' },
    { id: 'preferences', name: '偏好记忆', icon: '⚙', cat: 'analyze', desc: 'AI偏好与历史习惯', fn: 'Tools.showPreferences' }
  ],
  init: function () {
    this.render();
  },
  render: function () {
    var grid = document.getElementById('toolboxGrid');
    var cat = this.currentCat;
    var tools = cat === 'all' ? this.tools : this.tools.filter(function (t) { return t.cat === cat; });
    var html = '';
    tools.forEach(function (t) {
      html += '<div class="tool-card" data-cat="' + t.cat + '" onclick="ToolboxPage.execute(\'' + t.id + '\')">';
      html += '<div class="tool-card-icon">' + t.icon + '</div>';
      html += '<div class="tool-card-body">';
      html += '<div class="tool-card-name">' + t.name + '</div>';
      html += '<div class="tool-card-desc">' + t.desc + '</div>';
      html += '</div>';
      html += '</div>';
    });
    grid.innerHTML = html;
  },
  switchCat: function (cat) {
    this.currentCat = cat;
    document.querySelectorAll('.tb-tab').forEach(function (el) {
      el.classList.toggle('active', el.getAttribute('data-cat') === cat);
    });
    this.render();
  },
  filter: function (query) {
    var q = query.toLowerCase();
    var grid = document.getElementById('toolboxGrid');
    var tools = this.currentCat === 'all' ? this.tools : this.tools.filter(function (t) { return t.cat === ToolboxPage.currentCat; });
    if (q) {
      tools = tools.filter(function (t) {
        return t.name.toLowerCase().indexOf(q) >= 0 || t.desc.toLowerCase().indexOf(q) >= 0;
      });
    }
    var html = '';
    tools.forEach(function (t) {
      html += '<div class="tool-card" data-cat="' + t.cat + '" onclick="ToolboxPage.execute(\'' + t.id + '\')">';
      html += '<div class="tool-card-icon">' + t.icon + '</div>';
      html += '<div class="tool-card-body"><div class="tool-card-name">' + t.name + '</div><div class="tool-card-desc">' + t.desc + '</div></div>';
      html += '</div>';
    });
    grid.innerHTML = html;
  },
  execute: function (id) {
    var tool = null;
    for (var i = 0; i < this.tools.length; i++) {
      if (this.tools[i].id === id) { tool = this.tools[i]; break; }
    }
    if (!tool) return;
    if (!Store.state.currentProject) {
      UI.toast('请先选中一个项目', 'warn'); return;
    }
    // Try calling the global function
    var fnPath = tool.fn.split('.');
    var ctx = window;
    for (var i = 0; i < fnPath.length; i++) ctx = ctx[fnPath[i]];
    if (typeof ctx === 'function') {
      try { ctx(); } catch (e) { UI.toast('工具执行失败: ' + e.message, 'error'); }
    } else {
      UI.toast('工具尚未就绪，请先在编辑器中操作', 'info');
    }
  },
  clearOutput: function () {
    document.getElementById('toolboxOutputBody').innerHTML = '<div class="page-empty-state" style="padding:20px"><div class="page-empty-icon" style="font-size:24px">🔧</div><div>点击工具卡片开始使用</div></div>';
  }
};
