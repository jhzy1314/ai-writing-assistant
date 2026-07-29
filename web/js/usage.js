/* ============ usage.js：调用额度展示与校验 ============ */
var Usage = {
  canGenerate: function () {
    var u = Store.state.usage;
    if (!u) return true;
    var calls = u.today ? u.today.calls : 0;
    var tokens = u.today ? u.today.tokens : 0;
    var cl = (u.limits && u.limits.daily_call_limit) || 0;
    var tl = (u.limits && u.limits.daily_token_limit) || 0;
    if (cl && calls >= cl) return false;
    if (tl && tokens >= tl) return false;
    return true;
  },
  refresh: async function () {
    // 预填默认上限，避免 API 加载中显示 0
    if (!Store.state.usage) { document.getElementById('qCallsLimit').textContent = '500'; document.getElementById('qTokensLimit').textContent = '2,000,000'; }
    try {
      var u = await API.getUsage();
      Store.state.usage = u;
      this.render();
    } catch (e) { /* 静默失败，不打扰用户 */ }
  },
  render: function () {
    var u = Store.state.usage;
    if (!u) return;
    var calls = u.today ? u.today.calls : 0;
    var tokens = u.today ? u.today.tokens : 0;
    var cl = (u.limits && u.limits.daily_call_limit) || 0;
    var tl = (u.limits && u.limits.daily_token_limit) || 0;
    document.getElementById('qCalls').textContent = calls.toLocaleString();
    document.getElementById('qCallsLimit').textContent = cl ? cl.toLocaleString() : '∞';
    document.getElementById('qTokens').textContent = tokens.toLocaleString();
    document.getElementById('qTokensLimit').textContent = tl ? tl.toLocaleString() : '∞';
    var cm = document.getElementById('qCallsMeter');
    var tm = document.getElementById('qTokensMeter');
    var cp = cl ? Math.min(100, calls / cl * 100) : 0;
    var tp = tl ? Math.min(100, tokens / tl * 100) : 0;
    cm.querySelector('i').style.width = cp + '%';
    tm.querySelector('i').style.width = tp + '%';
    cm.classList.toggle('warn', cp >= 80 && cp < 100);
    cm.classList.toggle('danger', cp >= 100);
    tm.classList.toggle('warn', tp >= 80 && tp < 100);
    tm.classList.toggle('danger', tp >= 100);
    // 分角色 Token 消耗明细
    var roleBD = document.getElementById('quotaRoleBreakdown');
    var roles = u.today_by_role || [];
    if (roleBD && roles.length) {
      var ROLE_ICONS = { thinker: '🖊️', worker: '📝', verifier: '🔍', helper: '⚡' };
      roleBD.innerHTML = '<div style="font-weight:600;margin-bottom:3px">各角色 Token 消耗</div>' +
        roles.map(function (r) {
          var pct = tokens > 0 ? Math.round(r.tokens / tokens * 100) : 0;
          var barStyle = 'width:' + pct + '%;background:var(--accent);height:3px;border-radius:2px';
          return '<div style="margin:2px 0">' + (ROLE_ICONS[r.role] || '') + ' ' + (r.role || '?') +
            '：' + r.tokens.toLocaleString() + '（' + pct + '%）<div style="margin-top:1px;background:var(--border);border-radius:2px"><div style="' + barStyle + '"></div></div></div>';
        }).join('');
      roleBD.style.display = '';
    } else if (roleBD) {
      roleBD.style.display = 'none';
    }
    var hint = document.getElementById('usageHint');
    if (!this.canGenerate()) {
      hint.textContent = '⚠ 今日额度已用完';
      hint.style.color = 'var(--danger)';
      document.getElementById('btnGenerate').disabled = true;
    } else if ((cl && cp >= 80) || (tl && tp >= 80)) {
      hint.textContent = '⚠ 额度即将用尽';
      hint.style.color = 'var(--warning)';
      document.getElementById('btnGenerate').disabled = SSE.active;
    } else {
      hint.textContent = '';
      document.getElementById('btnGenerate').disabled = SSE.active;
    }
  }
};
