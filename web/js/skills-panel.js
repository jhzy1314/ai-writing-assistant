/* ============ skills-panel.js：写作技能管理面板 ============ */
;(function () {
  var SkillsPanel = {
    skillList: [],
    loaded: false,

    renderTo: function (containerId) {
      var el = document.getElementById(containerId);
      if (!el) return;
      var self = this;
      el.innerHTML = '<div class="ghead" style="display:flex;align-items:center;gap:8px">写作技能<span style="font-size:10px;color:var(--muted)">· 激活后将融入生成管线</span></div>' +
        '<div id="skillsPanelList" class="res-check-empty">加载中...</div>';
      this.load(function () { self.renderList(); });
    },

    load: function (cb) {
      var self = this;
      if (this.loaded) { cb(); return; }
      if (typeof EinoAPI === 'undefined') { this.skillList = []; this.loaded = true; cb(); return; }
      EinoAPI.get('/api/v1/skills').then(function (data) {
        self.skillList = Array.isArray(data) ? data : [];
        self.loaded = true;
        cb();
      }).catch(function () { self.skillList = []; self.loaded = true; cb(); });
    },

    renderList: function () {
      var el = document.getElementById('skillsPanelList');
      if (!el) return;
      var self = this;
      if (!this.skillList.length) {
        el.innerHTML = '<div class="res-check-empty">暂无写作技能<br><small style="color:var(--muted);font-size:10px">在 skills/ 目录创建 SKILL.md 后自动加载</small></div>';
        return;
      }
      var html = '';
      this.skillList.forEach(function (s) {
        var active = s.active;
        html += '<div style="padding:10px 12px;border:1px solid ' + (active ? 'var(--accent)' : 'var(--border)') + ';border-radius:8px;margin-bottom:8px;background:' + (active ? 'var(--panel3)' : 'var(--panel2)') + '">' +
          '<div style="display:flex;justify-content:space-between;align-items:center">' +
          '<div><strong>' + esc(s.name) + '</strong> <span style="font-size:10px;color:var(--muted)">v' + esc(s.version || '1.0') + '</span></div>' +
          '<button class="btn btn-sm ' + (active ? 'btn-ghost' : 'btn-primary') + '" onclick="SkillsPanel.toggle(\'' + esc(s.id) + '\',' + !active + ')" style="font-size:11px;padding:3px 8px">' + (active ? '停用' : '激活') + '</button></div>' +
          '<div style="font-size:11px;color:var(--muted);margin-top:2px">' + esc(s.description || '') + '</div>' +
          '<div style="font-size:10px;color:var(--faint);margin-top:2px">适用: <b>' + esc(s.target || 'all') + '</b></div>';
        if (s.rules && s.rules.length) {
          html += '<div style="margin-top:4px;font-size:10px;color:var(--muted)">';
          s.rules.forEach(function (r) { html += '<span style="background:var(--panel1);padding:1px 4px;border-radius:3px;margin-right:3px">' + esc(r.substring(0, 35)) + '</span>'; });
          html += '</div>';
        }
        html += '</div>';
      });
      el.innerHTML = html;
    },

    toggle: function (id, activate) {
      var endpoint = activate ? '/api/v1/skills/activate' : '/api/v1/skills/deactivate';
      EinoAPI.post(endpoint, { id: id }).then(function () {
        UI.toast((activate ? '已激活' : '已停用') + ': ' + id, 'success');
        SkillsPanel.loaded = false;
        SkillsPanel.load(function () { SkillsPanel.renderList(); });
      }).catch(function (e) { UI.toast('操作失败', 'error'); });
    }
  };

  window.SkillsPanel = SkillsPanel;
})();
