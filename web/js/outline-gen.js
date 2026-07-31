/* ============ outline-gen.js：AI 智能大纲生成器 ============ */
;(function () {
  var OutlineGen = {
    onGenerate: function (idn) {
      var idea = document.getElementById(idn + '_idea');
      var chaptersEl = document.getElementById(idn + '_chapters');
      var outlineEl = document.getElementById(idn + '_outline');
      var btn = document.getElementById(idn + '_genBtn');
      if (!idea || !outlineEl) return;

      var ideaVal = idea.value.trim();
      if (!ideaVal) { UI.toast('请先输入创意描述', 'warn'); return; }

      var chapters = parseInt(chaptersEl && chaptersEl.value) || 30;
      if (btn) { btn.disabled = true; btn.textContent = '⏳ 生成中...'; }
      outlineEl.value = '正在生成大纲...';
      outlineEl.style.background = 'var(--panel3)';

      EinoAPI.generateOutline(ideaVal, chapters).then(function (data) {
        if (data && data.outline) {
          outlineEl.value = data.outline;
          outlineEl.style.background = 'var(--panel2)';
          UI.toast('大纲已生成 (' + chapters + '章)', 'success');
        } else {
          outlineEl.value = '';
          outlineEl.style.background = 'var(--panel2)';
          UI.toast('大纲生成失败，请重试', 'error');
        }
      }).catch(function (e) {
        outlineEl.value = '';
        outlineEl.style.background = 'var(--panel2)';
        UI.toast('大纲生成失败: ' + (e && e.message || '网络错误'), 'error');
      }).finally(function () {
        if (btn) { btn.disabled = false; btn.textContent = '🤖 AI 生成大纲'; }
      });
    },

    addToModal: function (idn) {
      return '<div class="form-group" style="border-top:1px solid var(--border);padding-top:10px;margin-top:4px">' +
        '<label>AI 智能大纲 <span style="font-size:10px;color:var(--muted)">（输入创意，自动生成章节目录）</span></label>' +
        '<textarea id="' + idn + '_idea" placeholder="例如：一个少年在山洞捡到神秘戒指，踏上修仙之路..." style="width:100%;height:60px;font-size:12px;resize:vertical"></textarea>' +
        '<div style="display:flex;align-items:center;gap:6px;margin-top:4px">' +
        '<input id="' + idn + '_chapters" type="number" value="30" min="5" max="200" style="width:55px;font-size:11px" title="大纲章节数">' +
        '<span style="font-size:11px;color:var(--muted)">章</span>' +
        '<button id="' + idn + '_genBtn" class="btn btn-ghost btn-sm" onclick="OutlineGen.onGenerate(\'' + idn + '\')" style="margin-left:auto">🤖 AI 生成大纲</button>' +
        '</div>' +
        '<textarea id="' + idn + '_outline" placeholder="大纲将显示在此...也可手动编辑" style="width:100%;height:80px;font-size:11px;margin-top:6px;resize:vertical;background:var(--panel2);border:1px solid var(--border);border-radius:5px;padding:6px"></textarea>' +
        '</div>';
    }
  };

  window.OutlineGen = OutlineGen;
})();
