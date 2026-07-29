/* ============ pipeline.js：多Agent流水线可视化（步骤列表+中间产出展示） ============ */
var ROLE_META = {
  thinker:  { ico: '🖊️', label: '构思大纲', cls: 'thinker', order: 0 },
  worker:   { ico: '📝', label: '动笔写作', cls: 'worker', order: 1 },
  verifier: { ico: '🔍', label: '品质审稿', cls: 'verifier', order: 2 },
  helper:   { ico: '⚡', label: '辅助润色', cls: 'helper', order: 0 },
  manual:   { ico: '🎯', label: '手动直调', cls: 'manual', order: 0 },
  idle:     { ico: '·', label: '等待生成', cls: 'idle', order: -1 }
};
var PIPELINE_STEPS = [
  { key: 'thinker',  label: '🖊️ 构思大纲', role: 'thinker' },
  { key: 'worker',   label: '📝 动笔写作', role: 'worker' },
  { key: 'verifier', label: '🔍 品质审稿', role: 'verifier' }
];

var PipelineUI = {
  reset: function () {
    Store.state.pipeline = {
      active: true, stage: '准备中…', role: '', progress: 0, roleKey: 'idle',
      log: [], warn: '', steps: [], outline: '', issues: [], models: {}, degraded: null
    };
    var badge = document.getElementById('pipeSnapshotBadge');
    if (badge) badge.style.display = 'none';
    document.getElementById('pipeSnapshotCount').textContent = '0';
    this.render();
  },
  setActive: function (active, done) {
    Store.state.pipeline.active = active;
    if (done) {
      var self = this;
      setTimeout(function () { if (!Store.state.pipeline.active) document.getElementById('pipeCard').style.opacity = '.55'; }, 3000);
    }
    this.render();
  },
  applyStage: function (ev) {
    var roleKey = ev.role || 'idle';
    var meta = ROLE_META[roleKey] || ROLE_META.idle;
    var p = Store.state.pipeline;

    // 记录步骤完成状态
    var steps = p.steps;
    if (roleKey !== 'idle' && roleKey !== 'manual' && roleKey !== 'helper') {
      // 标记之前步骤为已完成
      steps.forEach(function (s) {
        if (ROLE_META[s.role] && ROLE_META[s.role].order < (meta.order || 0)) { s.status = 'done'; }
      });
      var existing = steps.find(function (s) { return s.role === roleKey; });
      if (existing) {
        existing.status = existing.status === 'done' ? 'done' : 'active';
        existing.model = ev.model || existing.model || '';
        existing.stage = ev.stage || existing.stage || '';
      } else {
        steps.push({ role: roleKey, label: meta.label, status: 'active', model: ev.model || '', stage: ev.stage || meta.label, iteration: ev.iteration || 0, text: ev.text || '' });
      }
      // 存储中间文本产出
      if (ev.text && roleKey === 'thinker') { p.outline = ev.text; }
    }
    // 记录模型使用信息
    if (ev.model && roleKey !== 'idle') {
      if (!p.models[roleKey]) p.models[roleKey] = ev.model;
    }
    p.stage = ev.stage || meta.label;
    p.role = meta.label;
    p.roleKey = roleKey;
    p.log.push({ t: new Date().toLocaleTimeString('zh-CN', { hour12: false }), msg: (ev.model ? '[' + ev.model + '] ' : '') + (ev.stage || meta.label) });
    this.render();
  },
  setStage: function (stage, roleKey, progress) {
    var meta = ROLE_META[roleKey] || ROLE_META.idle;
    Store.state.pipeline.stage = stage;
    Store.state.pipeline.role = meta.label;
    Store.state.pipeline.roleKey = roleKey;
    if (progress != null) Store.state.pipeline.progress = progress;
    this.render();
  },
  setWarn: function (msg) { Store.state.pipeline.warn = msg; this.render(); },
  log: function (msg) {
    Store.state.pipeline.log.push({ t: new Date().toLocaleTimeString('zh-CN', { hour12: false }), msg: msg });
    this.render();
  },
  render: function () {
    var p = Store.state.pipeline;
    var card = document.getElementById('pipeCard');
    card.classList.toggle('idle', !p.active && !p.steps.length);
    // 静态引导：有步骤时隐藏，无步骤时显示
    var intro = document.getElementById('pipeIntro');
    if (intro) intro.style.display = p.steps.length || p.active ? 'none' : '';
    var meta = ROLE_META[p.roleKey] || ROLE_META.idle;
    document.getElementById('pipeRoleIco').textContent = meta.ico;
    document.getElementById('pipeRoleIco').className = 'role-ico ' + meta.cls;
    document.getElementById('pipeStage').textContent = p.stage || '等待生成';
    document.getElementById('pipeRole').textContent = p.role || meta.label;

    // 步骤列表渲染
    var stepsEl = document.getElementById('pipeSteps');
    if (stepsEl) {
      var totalSteps = PIPELINE_STEPS.length;
      var completed = p.steps.filter(function (s) { return s.status === 'done'; }).length;
      if (p.steps.length === 0) {
        stepsEl.innerHTML = PIPELINE_STEPS.map(function (s) {
          return '<div class="pipe-step pending"><span class="step-icon">⬜</span><span>' + s.label + '</span></div>';
        }).join('');
      } else {
        stepsEl.innerHTML = PIPELINE_STEPS.map(function (st) {
          var ss = p.steps.find(function (s) { return s.role === st.role; });
          if (!ss) return '<div class="pipe-step pending"><span class="step-icon">⬜</span><span>' + st.label + '</span></div>';
          var icon = ss.status === 'done' ? '✅' : '⏳';
          var cls = ss.status === 'done' ? 'done' : 'active';
          var modelTag = ss.model ? ' <span class="step-model">(' + esc(ss.model) + ')</span>' : '';
          return '<div class="pipe-step ' + cls + '"><span class="step-icon">' + icon + '</span><span>' + ss.label + modelTag + '</span></div>';
        }).join('');
      }
    }

    // Thinker 大纲面板
    var outlineEl = document.getElementById('pipeOutline');
    if (outlineEl && p.outline) {
      outlineEl.style.display = '';
      document.getElementById('pipeOutlineContent').textContent = p.outline;
    } else if (outlineEl) {
      outlineEl.style.display = 'none';
    }

    // Verifier issues 面板
    var issuesEl = document.getElementById('pipeIssues');
    if (issuesEl && p.issues && p.issues.length) {
      issuesEl.style.display = '';
      document.getElementById('pipeIssuesList').innerHTML = p.issues.map(function (iss, i) {
        return '<div class="pipe-issue-item">#' + (i + 1) + ' ' + esc(iss) + '</div>';
      }).join('');
    } else if (issuesEl) {
      issuesEl.style.display = 'none';
    }

    // 模型绑定展示
    var modelsEl = document.getElementById('pipeModels');
    if (modelsEl && Object.keys(p.models).length) {
      modelsEl.style.display = '';
      var modelLines = [];
      Object.keys(p.models).forEach(function (r) {
        var rm = ROLE_META[r] || {};
        modelLines.push('<span class="pipe-model-tag"><b>' + (rm.ico || '') + ' ' + (rm.label || r) + ':</b> ' + esc(p.models[r]) + '</span>');
      });
      modelsEl.innerHTML = modelLines.join(' ');
    } else if (modelsEl) {
      modelsEl.style.display = 'none';
    }

    // 降级详情
    var degradedEl = document.getElementById('pipeDegraded');
    if (degradedEl && p.degraded) {
      degradedEl.style.display = '';
      degradedEl.textContent = '⚠ 降级: ' + esc(p.degraded);
    } else if (degradedEl) {
      degradedEl.style.display = 'none';
    }

    // 进度条（基于步骤完成数）
    var pct = p.steps.length ? Math.min(100, Math.round((p.steps.filter(function (s) { return s.status === 'done'; }).length / Math.max(1, PIPELINE_STEPS.length)) * 100)) : (p.active ? 5 : 0);
    if (p.roleKey === 'thinker') pct = Math.min(pct, 30);
    else if (p.roleKey === 'worker') pct = Math.max(pct, 60);
    else if (p.roleKey === 'verifier') pct = Math.max(pct, 85);
    document.getElementById('pipePct').textContent = pct + '%';
    document.getElementById('pipeBar').style.width = pct + '%';

    // 警告
    var warnEl = document.getElementById('pipeWarn');
    if (p.warn) { warnEl.textContent = '⚠ ' + p.warn; warnEl.classList.add('show'); }
    else { warnEl.classList.remove('show'); }

    // 日志
    var logEl = document.getElementById('pipeLog');
    logEl.innerHTML = p.log.slice(-20).map(function (l) {
      return '<div class="line"><span class="t">' + esc(l.t) + '</span><span>' + esc(l.msg) + '</span></div>';
    }).join('');
    logEl.scrollTop = logEl.scrollHeight;
  }
};
