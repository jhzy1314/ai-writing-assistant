/* ============ modelsettings.js：自定义 API 模型管理面板 ============ */
/* ===== 顶层常量：厂商预设列表，后续新增模型仅修改此处 ===== */
var MODEL_PRESETS = [
  { label:"DeepSeek",           baseUrl:"https://api.deepseek.com/v1",                     models:["deepseek-v4-pro","deepseek-v4-flash"] },
  { label:"Kimi(Moonshot)",     baseUrl:"https://api.moonshot.cn/v1",                      models:["kimi-k3","kimi-k2.7","kimi-k2.6","kimi-k2.5","kimi-latest"] },
  { label:"智谱GLM",            baseUrl:"https://open.bigmodel.cn/api/paas/v4",            models:["glm-5.2","glm-5-flash"] },
  { label:"通义千问Qwen",       baseUrl:"https://dashscope.aliyuncs.com/compatible-mode/v1", models:["qwen3.8-max","qwen3.8-turbo"] },
  { label:"文心一言ERNIE",      baseUrl:"https://qianfan.baidubce.com/v2",                  models:["ernie-6.0","ernie-5.1-turbo"] },
  { label:"OpenAI",             baseUrl:"https://api.openai.com/v1",                       models:["gpt-5.6-sol","gpt-5.6-terra","gpt-5.6-luna"] },
  { label:"Claude(Anthropic)",  baseUrl:"https://api.anthropic.com/v1",                    models:["claude-opus-4-8","claude-sonnet-5","claude-haiku-4-5"] },
  { label:"Gemini(Google)",     baseUrl:"https://generativelanguage.googleapis.com/v1",     models:["gemini-3.5-flash","gemini-3.1-pro"] }
];
/* ===== 远期预留：联网同步厂商模型列表 ===== */
var ModelSettings = {
  /* 将 MODEL_PRESETS 数组转为内部 presets 对象 */
  presets: (function () {
    var obj = {};
    MODEL_PRESETS.forEach(function (p, i) {
      var key = 'p' + i; // 稳定键值
      obj[key] = { name: p.label, vendor: p.label, endpoint: p.baseUrl, ctx: 32768, stream: 1, models: p.models };
    });
    obj['custom'] = { name: '自定义', vendor: '', endpoint: '', ctx: 4096, stream: 1, models: [] };
    return obj;
  })(),
  syncOnlineModels: null,
  _tab: 'api',
  render: function () {
    var page = document.getElementById('page-models');
    if (!page) return;
    var self = this;
    var models = Store.state.models || [];
    page.innerHTML = '<div class="model-tabs" style="display:flex;gap:0;margin-bottom:12px;border-bottom:2px solid var(--border)">' +
      '<span class="model-tab" id="tabApi" style="padding:6px 16px;cursor:pointer;border-bottom:2px solid ' + (self._tab === 'api' ? 'var(--accent)' : 'transparent') + ';margin-bottom:-2px;font-weight:' + (self._tab === 'api' ? '600' : '400') + ';color:' + (self._tab === 'api' ? 'var(--accent)' : 'var(--text2)') + '" onclick="ModelSettings.switchTab(\'api\')">🔑 付费API</span>' +
      '<span class="model-tab" id="tabWebai" style="padding:6px 16px;cursor:pointer;border-bottom:2px solid ' + (self._tab === 'webai' ? 'var(--accent)' : 'transparent') + ';margin-bottom:-2px;font-weight:' + (self._tab === 'webai' ? '600' : '400') + ';color:' + (self._tab === 'webai' ? 'var(--accent)' : 'var(--text2)') + '" onclick="ModelSettings.switchTab(\'webai\')">🌐 免费网页AI</span>' +
      '</div>' +
      '<div id="modelTabContent"></div>' +
      '<div class="ghead" style="margin-top:20px">Agent 角色模型分配</div>' +
      '<div class="role-assign-list" id="roleAssignList">加载中…</div>';
    ModelSettings.loadRoleAssignments();
    self.switchTab(self._tab);
  },
  switchTab: function (tab) {
    this._tab = tab;
    var tabApi = document.getElementById('tabApi');
    var tabWebai = document.getElementById('tabWebai');
    if (tabApi) {
      tabApi.style.borderBottomColor = tab === 'api' ? 'var(--accent)' : 'transparent';
      tabApi.style.fontWeight = tab === 'api' ? '600' : '400';
      tabApi.style.color = tab === 'api' ? 'var(--accent)' : 'var(--text2)';
    }
    if (tabWebai) {
      tabWebai.style.borderBottomColor = tab === 'webai' ? 'var(--accent)' : 'transparent';
      tabWebai.style.fontWeight = tab === 'webai' ? '600' : '400';
      tabWebai.style.color = tab === 'webai' ? 'var(--accent)' : 'var(--text2)';
    }
    var ct = document.getElementById('modelTabContent');
    if (!ct) return;
    if (tab === 'api') { this.renderApiTab(ct); }
    else { this.renderWebaiTab(ct); }
  },
  renderApiTab: function (ct) {
    var models = Store.state.models || [];
    ct.innerHTML = '<div class="ghead" style="display:flex;align-items:center;gap:8px">' +
      '付费API模型' +
      '<span class="link-btn" onclick="ModelSettings.showCreate()" title="新增自定义API模型">＋ 新增</span>' +
      '</div>' +
      '<div class="model-list" id="modelList">' +
      (models.length ? models.map(function (m) { return ModelSettings.modelCard(m); }).join('') : '<div class="res-check-empty">暂无模型，点击"新增"添加 API 配置。或者切换到「免费网页AI」标签使用免费通道。</div>') +
      '</div>';
  },
  renderWebaiTab: function (ct) {
    var self = this;
    ct.innerHTML = '<div class="ghead" style="display:flex;align-items:center;gap:8px">' +
      '免费网页AI通道 <span style="font-size:10px;color:var(--muted)">（粘贴Cookie即可使用，零成本）</span>' +
      '<span class="link-btn" onclick="ModelSettings.showWebaiCreate()" title="新增网页AI">＋ 新增</span>' +
      '</div>' +
      '<div style="font-size:10.5px;color:var(--accent);background:var(--accent-soft);border-radius:6px;padding:6px 8px;margin-bottom:6px">🪄 点下方模板的「自动获取」按钮，会自动打开浏览器窗口 → 你在窗口里登录网站 → Cookie 自动抓取并保存，全程无需手动复制。</div>' +
      '<div id="webaiList" style="margin-top:8px"><div class="res-check-empty">加载中…</div></div>';
    this.loadWebaiList();
  },
  loadWebaiList: async function () {
    var el = document.getElementById('webaiList');
    if (!el) return;
    try {
      var providers = await API.listWebAIProviders();
      var models = Store.state.models || [];
      var webaiModels = models.filter(function (m) { return m.model_type === 'webai' || m.vendor === 'webai' || (m.provider && m.provider.length > 0); });
      // Also show models that are webai type
      var html = '';
      // Show built-in provider templates
      var provKeys = Object.keys(providers);
      if (provKeys.length > 0) {
        html += '<div style="font-size:11px;color:var(--muted);margin-bottom:6px">内置免费模板（选择一个粘贴Cookie即可使用）：</div>';
        provKeys.forEach(function (key) {
          var p = providers[key];
          html += '<div class="webai-provider-card" style="background:var(--panel2);border:1px solid var(--border);border-radius:7px;padding:10px;margin-bottom:8px">' +
            '<div style="display:flex;align-items:center;justify-content:space-between">' +
            '<div><b style="color:var(--accent)">' + esc(p.name || key) + '</b>' +
            '<div style="font-size:10px;color:var(--muted)">端点：' + esc(p.baseURL || '') + '</div></div>' +
            '<div style="display:flex;gap:6px">' +
            '<button class="btn btn-primary btn-sm" onclick="ModelSettings.autoCookie(\'' + key + '\',\'' + esc(p.name) + '\',\'' + esc(p.baseURL || '') + '\')" style="font-size:11px">🪄 自动获取</button>' +
            '<button class="btn btn-ghost btn-sm" onclick="ModelSettings.showWebaiQuickSetup(\'' + key + '\',\'' + esc(p.name) + '\',\'' + esc(p.baseURL || '') + '\')" style="font-size:11px">✍️ 手动填写</button>' +
            '</div></div></div>';
        });
      }
      // Show existing webai models
      var wmodels = Store.state.models.filter(function (m) { return m.vendor === 'kimi' || m.vendor === 'doubao' || m.provider; });
      if (wmodels.length > 0) {
        html += '<div style="font-size:11px;color:var(--muted);margin:10px 0 4px">已配置的网页AI：</div>';
        wmodels.forEach(function (m) {
          html += '<div class="model-card" style="margin-bottom:6px">' +
            '<div class="mc-head"><span class="mc-name">' + esc(m.name) + '</span><span class="mc-vendor">' + esc(m.vendor || '') + '</span></div>' +
            '<div class="mc-acts">' +
            '<button class="tool-btn" onclick="ModelSettings.testWebaiConnect(\'' + m.id + '\')">🔗 测试</button>' +
            '<button class="tool-btn danger" onclick="ModelSettings.delModel(\'' + m.id + '\')">🗑 删除</button>' +
            '</div></div>';
        });
      }
      if (!provKeys.length && !wmodels.length) {
        html += '<div class="res-check-empty">加载免费AI模板失败，请检查网络或服务状态。</div>';
      }
      el.innerHTML = html;
    } catch (e) {
      el.innerHTML = '<div class="res-check-empty">加载失败：' + esc(e.message) + '</div>';
    }
  },
  showWebaiQuickSetup: function (providerKey, name, baseURL) {
    var idn = 'wa_' + uid();
    UI.modal({
      title: '配置免费网页AI：' + name,
      body: '<div style="font-size:11px;color:var(--muted);margin-bottom:8px">在浏览器中打开 ' + esc(name) + ' 网站，按F12打开开发者工具→应用→Cookies→复制对应Cookie值</div>' +
        '<div class="form-group"><label>模型名称</label><input id="' + idn + '_name" value="' + esc(name) + '免费版" placeholder="模型名称"></div>' +
        '<div class="form-group"><label>Cookie值 *</label><input id="' + idn + '_cookie" type="password" placeholder="粘贴从浏览器复制的Cookie"></div>' +
        '<div class="form-group"><label>请求地址</label><input id="' + idn + '_url" value="' + esc(baseURL) + '" placeholder="网页AI接口地址"></div>' +
        '<div class="form-group"><label>模型标识</label><input id="' + idn + '_model" placeholder="网页AI内部模型名（可选）"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: async function (m, ov) {
          var cname = document.getElementById(idn + '_name').value.trim();
          var cookie = document.getElementById(idn + '_cookie').value.trim();
          var url = document.getElementById(idn + '_url').value.trim();
          var model = document.getElementById(idn + '_model').value.trim();
          if (!cname || !cookie) { UI.toast('模型名称和Cookie必填', 'warn'); return; }
          try {
            await API.createWebAIModel({ name: cname, provider: providerKey, cookie: cookie, request_url: url, model_type: 'webai', vendor: providerKey, model_name: model });
            ov.remove();
            await ModelSettings.loadAll();
            ModelSettings._tab = 'webai';
            ModelSettings.render();
            UI.toast('网页AI已配置', 'success');
          } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
        }}
      ]
    });
  },
  testWebaiConnect: async function (id) {
    var m = Store.state.models.find(function (x) { return x.id === id; });
    UI.toast('正在测试「' + (m ? m.name : id) + '」…', 'info');
    try {
      var r = await API.testWebAIModel({ provider: m ? m.vendor : '', cookie: m ? m.api_key : '', request_url: m ? m.api_endpoint : '' });
      UI.toast('连接成功', 'success');
    } catch (e) { UI.toast('连接失败：' + e.message, 'error'); }
  },
  // 自动抓取网页AI Cookie：启动浏览器 → 用户登录 → 轮询抓取 → 自动保存模型
  autoCookie: async function (providerKey, name, baseURL) {
    var self = this;
    if (this._cookieBusy) { UI.toast('已有抓取任务进行中', 'warn'); return; }
    this._cookieBusy = true;
    try {
      var s = await API.autoCookieStart(providerKey);
    } catch (e) { this._cookieBusy = false; UI.toast('启动浏览器失败：' + e.message, 'error'); return; }
    var sid = s.session_id;
    var t0 = Date.now();
    var m = UI.modal({
      title: '🪄 自动获取 Cookie：' + name,
      body: '<div style="font-size:12px;line-height:1.8">' +
        '<div>1️⃣ 系统已打开 <b>' + esc(name) + '</b> 的浏览器窗口</div>' +
        '<div>2️⃣ 请在窗口中<b>登录你的账号</b>（登录后会自动跳转回聊天页）</div>' +
        '<div>3️⃣ 登录成功后 Cookie 将<b>自动抓取并保存</b>，无需手动操作</div>' +
        '<div style="color:var(--muted);font-size:11px;margin-top:6px" id="acStatus">⏳ 等待登录…（10分钟内有效）</div>' +
        '<div style="color:#e6a23c;font-size:11px;margin-top:4px">⚠️ 若窗口意外消失：直接点下方「关闭窗口」再重新点「自动获取」即可</div></div>',
      actions: [{ id: 'close', label: '关闭窗口（取消）' }]
    });
    var iv = setInterval(async function () {
      try {
        var r = await API.autoCookiePoll(sid);
        if (r.status === 'completed' && r.cookie) {
          clearInterval(iv);
          self._cookieBusy = false;
          m.overlay.remove();
          try {
            await API.createWebAIModel({ name: name, provider: providerKey, cookie: r.cookie, request_url: baseURL, model_type: 'webai', vendor: providerKey });
            await ModelSettings.loadAll();
            UI.toast('✅ Cookie 抓取成功，已保存「' + name + '」', 'success');
          } catch (e2) { UI.toast('保存失败：' + e2.message, 'error'); }
        } else if (r.status === 'failed') {
          clearInterval(iv);
          self._cookieBusy = false;
          UI.toast('抓取失败：' + (r.error || '未知错误'), 'error');
        } else if (r.status === 'pending' || r.status === 'running') {
          // 实时更新等待状态：已用时间 + 检测到的 Cookie 数
          var el = document.getElementById('acStatus');
          if (el) {
            var sec = Math.floor((Date.now() - t0) / 1000);
            var mm = Math.floor(sec / 60), ss = sec % 60;
            var det = (r.detected_cookies != null && r.detected_cookies > 0)
              ? ' · 已检测到 ' + r.detected_cookies + ' 个 Cookie（等待登录态）' : '';
            el.textContent = '⏳ 等待登录… 已等待 ' + mm + '分' + (ss < 10 ? '0' : '') + ss + '秒' + det + '（10分钟内有效）';
          }
        }
      } catch (e) { /* 轮询中，忽略瞬时错误 */ }
    }, 2000);
    // 窗口关闭时取消
    m.overlay.querySelector('[data-act]').onclick = function () {
      clearInterval(iv);
      API.autoCookieCancel(sid).catch(function () {});
      self._cookieBusy = false;
      m.overlay.remove();
      UI.toast('已取消抓取', 'warn');
    };
  },
  showWebaiCreate: function () {
    this.showCreate();
  },
  modelCard: function (m) {
    var isCustom = m.is_custom === 1;
    var isDefault = m.is_default === 1;
    var isActive = m.status === 'active';
    return '<div class="model-card' + (isDefault ? ' default' : '') + (!isActive ? ' disabled' : '') + '">' +
      '<div class="mc-head">' +
        '<span class="mc-name">' + esc(m.name) + (isDefault ? ' <span class="tag builtin">默认</span>' : '') + '</span>' +
        '<span class="mc-vendor">' + esc(m.vendor || '未设置厂商') + '</span>' +
        '<span class="mc-status ' + (isActive ? 'on' : 'off') + '">' + (isActive ? '启用' : '停用') + '</span>' +
      '</div>' +
      '<div class="mc-body">' +
        '<div class="mc-row"><span>端点</span><span>' + esc(m.api_endpoint || '—') + '</span></div>' +
        '<div class="mc-row"><span>流式</span><span>' + (m.support_stream === 1 ? '支持' : '不支持') + '</span></div>' +
        '<div class="mc-row"><span>上下文</span><span>' + (m.context_limit || 4096) + ' tokens</span></div>' +
        '<div class="mc-row"><span>温度</span><span>' + (m.temperature != null ? m.temperature : 0.7) + '</span></div>' +
        (m.description ? '<div class="mc-row"><span>备注</span><span>' + esc(m.description) + '</span></div>' : '') +
      '</div>' +
      '<div class="mc-acts">' +
        '<span style="display:flex;align-items:center;gap:4px;font-size:10.5px;color:var(--muted)">' +
        '<label class="model-toggle" onclick="ModelSettings.toggleStatus(\'' + m.id + '\')" title="切换启用/停用">' +
        '<input type="checkbox"' + (isActive ? ' checked' : '') + '><span class="slider"></span></label>' +
        (isActive ? '启用' : '停用') + '</span>' +
        '<button class="tool-btn" onclick="ModelSettings.testConnect(\'' + m.id + '\')">🔗 测试</button>' +
        '<button class="tool-btn" onclick="ModelSettings.showEdit(\'' + m.id + '\')">✏️ 编辑</button>' +
        (!isDefault ? '<button class="tool-btn" onclick="ModelSettings.setDefault(\'' + m.id + '\')">⭐ 设为默认</button>' : '') +
        '<button class="tool-btn danger" onclick="ModelSettings.delModel(\'' + m.id + '\')">🗑 删除</button>' +
      '</div>' +
    '</div>';
  },
  loadAll: async function () {
    try {
      Store.state.models = await API.listModels();
      ModelSettings.render();
    } catch (e) { /* 静默 */ }
  },
  showCreate: function () {
    var idn = 'mdl_' + uid();
    var presetsHTML = Object.keys(this.presets).map(function (k) {
      var p = ModelSettings.presets[k];
      return '<option value="' + k + '" data-endpoint="' + esc(p.endpoint) + '" data-vendor="' + esc(p.vendor) + '" data-ctx="' + p.ctx + '" data-stream="' + p.stream + '">' + esc(p.name) + '</option>';
    }).join('');
    UI.modal({
      title: '新增自定义 API 模型',
      body: '<div class="form-group"><label>选择厂商模板</label><select id="' + idn + '_preset" onchange="ModelSettings.applyPreset(\'' + idn + '\')">' + presetsHTML + '</select></div>' +
        '<div class="form-group"><label>模型名称 *</label><div id="' + idn + '_nameWrap"><select id="' + idn + '_nameSel"></select><input id="' + idn + '_name" placeholder="请输入模型名称" style="display:none"></div>' +
          '<div id="' + idn + '_customNameRow" style="display:none;margin-top:4px"><label style="display:flex;align-items:center;gap:4px;cursor:pointer;font-size:11px;color:var(--muted)"><input type="checkbox" id="' + idn + '_customNameCb" onchange="ModelSettings.toggleCustomName(\'' + idn + '\')"><span>列表中无目标模型？勾选后手动输入模型标识（保留当前厂商参数）</span></label></div></div>' +
        '<div class="form-group"><label>API Key *</label><input id="' + idn + '_key" type="password" placeholder="sk-..."></div>' +
        '<div class="adv-wrap"><div class="adv-toggle" onclick="this.parentElement.classList.toggle(\'open\')">⚙ 高级设置 ▾</div>' +
        '<div class="adv-body">' +
          '<div class="form-group"><label>自定义 Base URL</label><input id="' + idn + '_endpoint" placeholder="https://api.openai.com/v1"></div>' +
          '<div class="form-group"><label>厂商</label><input id="' + idn + '_vendor" placeholder="例如：OpenAI"></div>' +
          '<div class="form-row">' +
            '<div class="form-group"><label>上下文限制</label><input id="' + idn + '_ctx" type="number" value="4096" min="512" max="1048576"></div>' +
            '<div class="form-group"><label>流式支持</label><select id="' + idn + '_stream"><option value="1">支持</option><option value="0">不支持</option></select></div>' +
          '</div>' +
          '<div class="form-row">' +
            '<div class="form-group"><label>Temperature</label><input id="' + idn + '_temp" type="number" value="0.7" min="0" max="2" step="0.1"></div>' +
            '<div class="form-group"><label>Top P</label><input id="' + idn + '_topp" type="number" value="0.9" min="0" max="1" step="0.1"></div>' +
          '</div>' +
          '<div class="form-group"><label>备注</label><input id="' + idn + '_desc" placeholder="可选描述"></div>' +
        '</div></div>' +
        '<div class="form-row" style="margin-top:8px">' +
          '<div class="form-group"><label>状态</label><select id="' + idn + '_status"><option value="active">启用</option><option value="disabled">停用</option></select></div>' +
          '<div class="form-group"><label>默认模型</label><select id="' + idn + '_def"><option value="0">否</option><option value="1">是</option></select></div>' +
        '</div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '创建', cls: 'btn-primary', onClick: async function (m, ov) {
          var nameSel = document.getElementById(idn + '_nameSel');
          var nameInput = document.getElementById(idn + '_name');
          // 可见控件即为有效输入源（互斥，仅一个可见）
          var name = (nameSel.style.display === 'none' ? nameInput.value : nameSel.value).trim();
          var endpoint = document.getElementById(idn + '_endpoint').value.trim();
          var key = document.getElementById(idn + '_key').value.trim();
          if (!name || !key) { UI.toast('模型名称和 API Key 必填', 'warn'); return; }
          try {
            await API.createModel({
              name: name,
              vendor: document.getElementById(idn + '_vendor').value.trim(),
              api_endpoint: endpoint,
              api_key: key,
              status: document.getElementById(idn + '_status').value,
              context_limit: parseInt(document.getElementById(idn + '_ctx').value) || 4096,
              support_stream: parseInt(document.getElementById(idn + '_stream').value),
              is_default: parseInt(document.getElementById(idn + '_def').value),
              is_custom: 1,
              description: document.getElementById(idn + '_desc').value.trim(),
              temperature: parseFloat(document.getElementById(idn + '_temp').value) || 0.7,
              top_p: parseFloat(document.getElementById(idn + '_topp').value) || 0.9
            });
            ov.remove();
            await ModelSettings.loadAll();
            UI.toast('模型已创建', 'success');
          } catch (e) { UI.toast('创建失败：' + e.message, 'error'); }
        }}
      ]
    });
    this.applyPreset(idn);
  },
  applyPreset: function (idn) {
    var sel = document.getElementById(idn + '_preset');
    if (!sel) return;
    var key = sel.value;
    var p = this.presets[key] || {};
    if (p.endpoint) document.getElementById(idn + '_endpoint').value = p.endpoint;
    if (p.vendor) document.getElementById(idn + '_vendor').value = p.vendor;
    document.getElementById(idn + '_ctx').value = p.ctx || 4096;
    document.getElementById(idn + '_stream').value = p.stream || 1;
    var nameSel = document.getElementById(idn + '_nameSel');
    var nameInput = document.getElementById(idn + '_name');
    var customRow = document.getElementById(idn + '_customNameRow');
    var cb = document.getElementById(idn + '_customNameCb');
    // 互斥渲染：预设厂商显示下拉（可勾选手动输入），自定义显示输入框
    if (key === 'custom' || !p.models || !p.models.length) {
      nameSel.style.display = 'none';
      nameInput.style.display = '';
      if (customRow) customRow.style.display = 'none';
      nameInput.value = '';
    } else {
      nameSel.style.display = '';
      nameInput.style.display = 'none';
      if (customRow) customRow.style.display = '';
      if (cb) cb.checked = false;
      nameSel.innerHTML = p.models.map(function (m) { return '<option value="' + esc(m) + '">' + esc(m) + '</option>'; }).join('');
    }
  },
  toggleCustomName: function (idn) {
    var cb = document.getElementById(idn + '_customNameCb');
    var nameSel = document.getElementById(idn + '_nameSel');
    var nameInput = document.getElementById(idn + '_name');
    if (cb && cb.checked) {
      nameSel.style.display = 'none';
      nameInput.style.display = '';
      nameInput.focus();
    } else {
      nameSel.style.display = '';
      nameInput.style.display = 'none';
    }
  },
  showEdit: function (id) {
    var m = Store.state.models.find(function (x) { return x.id === id; });
    if (!m) return;
    var idn = 'mdl_' + uid();
    UI.modal({
      title: '编辑模型：' + esc(m.name),
      body: '<div class="form-group"><label>模型名称 *</label><input id="' + idn + '_name" value="' + esc(m.name) + '"></div>' +
        '<div class="form-group"><label>API Key（留空不修改）</label><input id="' + idn + '_key" type="password" placeholder="留空不修改"></div>' +
        '<div class="adv-wrap open"><div class="adv-toggle" onclick="this.parentElement.classList.toggle(\'open\')">⚙ 高级设置 ▾</div>' +
        '<div class="adv-body">' +
          '<div class="form-group"><label>API 端点</label><input id="' + idn + '_endpoint" value="' + esc(m.api_endpoint || '') + '"></div>' +
          '<div class="form-group"><label>厂商</label><input id="' + idn + '_vendor" value="' + esc(m.vendor || '') + '"></div>' +
          '<div class="form-row">' +
            '<div class="form-group"><label>上下文限制</label><input id="' + idn + '_ctx" type="number" value="' + (m.context_limit || 4096) + '"></div>' +
            '<div class="form-group"><label>流式</label><select id="' + idn + '_stream"><option value="1"' + (m.support_stream === 1 ? ' selected' : '') + '>支持</option><option value="0"' + (m.support_stream === 0 ? ' selected' : '') + '>不支持</option></select></div>' +
          '</div>' +
          '<div class="form-row">' +
            '<div class="form-group"><label>Temperature</label><input id="' + idn + '_temp" type="number" value="' + (m.temperature != null ? m.temperature : 0.7) + '" min="0" max="2" step="0.1"></div>' +
            '<div class="form-group"><label>Top P</label><input id="' + idn + '_topp" type="number" value="' + (m.top_p != null ? m.top_p : 0.9) + '" min="0" max="1" step="0.1"></div>' +
          '</div>' +
        '</div></div>' +
        '<div class="form-row" style="margin-top:8px">' +
          '<div class="form-group"><label>状态</label><select id="' + idn + '_status"><option value="active"' + (m.status === 'active' ? ' selected' : '') + '>启用</option><option value="disabled"' + (m.status === 'disabled' ? ' selected' : '') + '>停用</option></select></div>' +
        '</div>' +
        '<div class="form-group"><label>备注</label><input id="' + idn + '_desc" value="' + esc(m.description || '') + '"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '保存', cls: 'btn-primary', onClick: async function (md, ov) {
          var update = {
            name: document.getElementById(idn + '_name').value.trim(),
            vendor: document.getElementById(idn + '_vendor').value.trim(),
            api_endpoint: document.getElementById(idn + '_endpoint').value.trim(),
            status: document.getElementById(idn + '_status').value,
            context_limit: parseInt(document.getElementById(idn + '_ctx').value) || 4096,
            support_stream: parseInt(document.getElementById(idn + '_stream').value),
            description: document.getElementById(idn + '_desc').value.trim(),
            temperature: parseFloat(document.getElementById(idn + '_temp').value) || 0.7,
            top_p: parseFloat(document.getElementById(idn + '_topp').value) || 0.9
          };
          var key = document.getElementById(idn + '_key').value.trim();
          if (key) update.api_key = key;
          try {
            await API.updateModel(id, update);
            ov.remove();
            await ModelSettings.loadAll();
            Composer.refreshModels();
            UI.toast('模型已更新', 'success');
          } catch (e) { UI.toast('更新失败：' + e.message, 'error'); }
        }}
      ]
    });
  },
  delModel: function (id) {
    var m = Store.state.models.find(function (x) { return x.id === id; });
    if (!m) return;
    UI.confirm('删除模型', '确认删除模型「' + m.name + '」？', async function () {
      try {
        await API.deleteModel(id);
        await ModelSettings.loadAll();
        Composer.refreshModels();
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  },
  testConnect: async function (id) {
    var m = Store.state.models.find(function (x) { return x.id === id; });
    UI.toast('正在测试「' + (m ? m.name : id) + '」连接…', 'info');
    try {
      var r = await API.testModel(id);
      UI.toast(r.message || '连通性测试通过', 'success');
    } catch (e) { UI.toast('连接失败：' + e.message, 'error'); }
  },
  toggleStatus: async function (id) {
    var m = Store.state.models.find(function (x) { return x.id === id; });
    if (!m) return;
    var newStatus = m.status === 'active' ? 'disabled' : 'active';
    try {
      await API.updateModel(id, { status: newStatus });
      await ModelSettings.loadAll();
      Composer.refreshModels();
      UI.toast('「' + m.name + '」已' + (newStatus === 'active' ? '启用' : '停用'), 'success');
    } catch (e) { UI.toast('操作失败：' + e.message, 'error'); }
  },
  setDefault: async function (id) {
    try {
      await API.setDefaultModel(id);
      await ModelSettings.loadAll();
      Composer.refreshModels();
      UI.toast('已设为默认模型', 'success');
    } catch (e) { UI.toast('设置失败：' + e.message, 'error'); }
  },

  // ====== 角色-模型映射管理 ======
  roleLabels: { thinker: '🧭 规划师 Thinker', worker: '✍️ 创作者 Worker', verifier: '🔍 校验官 Verifier', helper: '🤖 轻助手 Helper' },
  roleDesc: { thinker: '分析需求、搭建大纲框架', worker: '根据大纲撰写正文', verifier: '审查一致性、发现问题回传修正', helper: '轻量任务（摘要、标题等）' },

  loadRoleAssignments: async function () {
    var roles = ['thinker', 'worker', 'verifier', 'helper'];
    var models = Store.state.models || [];
    var el = document.getElementById('roleAssignList');
    if (!el) return;
    var html = '';
    for (var i = 0; i < roles.length; i++) {
      var role = roles[i];
      try {
        var assigned = await API.getRoleModels(role);
        var models = (assigned && assigned.item && assigned.item.models) || [];
        var names = models.map(function (a) { return a.name || a; }).join(', ') || '未分配（使用默认模型）';
        html += '<div class="role-row" style="display:flex;align-items:center;gap:10px;padding:8px 0;border-bottom:1px solid var(--border)">';
        html += '<div style="flex:1"><strong>' + ModelSettings.roleLabels[role] + '</strong>';
        html += '<div style="font-size:10px;color:var(--muted)">' + ModelSettings.roleDesc[role] + '</div>';
        html += '<div style="font-size:11px;color:var(--text2);margin-top:2px">当前：' + names + '</div></div>';
        html += '<button class="tool-btn" onclick="ModelSettings.assignRole(\'' + role + '\')" style="font-size:11px">⚙ 分配</button>';
        html += '</div>';
      } catch (e) { html += '<div class="role-row" style="padding:8px 0;color:var(--muted)">' + ModelSettings.roleLabels[role] + '：加载失败</div>'; }
    }
    el.innerHTML = html;
  },

  assignRole: function (role) {
    var models = Store.state.models || [];
    var opts = models.map(function (m) {
      return '<label style="display:block;padding:4px 0;cursor:pointer;font-size:12px">' +
        '<input type="checkbox" value="' + m.id + '" class="role-chk-' + role + '" style="margin-right:6px">' +
        esc(m.name) + ' <span style="color:var(--muted);font-size:10px">(' + esc(m.vendor || '') + ')</span></label>';
    }).join('');
    API.getRoleModels(role).then(function (data) {
      var assignedModels = (data && data.item && data.item.models) || [];
      var assignedIds = assignedModels.map(function (a) { return a.model_id || a.id || a; });
      UI.modal({
        title: '分配模型到 ' + ModelSettings.roleLabels[role],
        body: '<div style="max-height:300px;overflow-y:auto;margin:8px 0">' + opts + '</div>' +
          '<div style="font-size:10px;color:var(--muted)">勾选的模型将按顺序作为该角色的候选。取消全部勾选则使用默认模型。</div>',
        actions: [
          { id: 'cancel', label: '取消' },
          { id: 'save', label: '保存', cls: 'btn-primary', onClick: function (m) {
            var chks = document.querySelectorAll('.role-chk-' + role + ':checked');
            var ids = Array.from(chks).map(function (c) { return c.value; });
            API.setRoleModels(role, ids).then(function () {
              UI.toast('已更新 ' + ModelSettings.roleLabels[role] + ' 模型分配', 'success');
              m.remove();
              ModelSettings.loadRoleAssignments();
            }).catch(function (e) { UI.toast('保存失败：' + e.message, 'error'); });
          }}
        ]
      });
      setTimeout(function () {
        var chks = document.querySelectorAll('[class^="role-chk-' + role + '"]');
        chks.forEach(function (c) { if (assignedIds.indexOf(c.value) >= 0) c.checked = true; });
      }, 100);
    });
  },

  showQuickKey: function () {
    // P3-4：动态读取 thinker 角色主模型名（所有角色共用同一模型链时展示该名）
    try {
      var rm = Store.state.roleModels || {};
      var tm = (rm.thinker && rm.thinker.models && rm.thinker.models[0]) || {};
      ModelSettings._primaryModelName = tm.name || 'deepseek-chat';
    } catch (e) { ModelSettings._primaryModelName = 'deepseek-chat'; }
    var models = Store.state.models || [];
    var active = models.filter(function (m) { return m.status === 'active'; });
    // 查找任意已配置 key 的模型（不限于 deepseek-v4-pro），用于展示密钥状态
    var keyedModel = models.find(function (m) { return m.api_key && m.api_key.length >= 8; }) || {};
    var maskedKey = keyedModel.api_key ? (keyedModel.api_key.substring(0, 5) + '…' + keyedModel.api_key.slice(-4)) : '未设置（请在下方面板配置）';
    var keyModelName = keyedModel.name || '';

    var idn = 'qk_' + uid();
    UI.modal({
      title: '🔑 API 密钥与模型管理',
      wide: '520px',
      body: '<div style="margin-bottom:12px;font-size:12px;color:var(--muted)">当前 Agent 角色默认模型：<b style="color:var(--accent)">' + (ModelSettings._primaryModelName || 'deepseek-chat') + '</b>。可在下方按角色分别绑定模型。</div>' +
        '<div class="qp-section" style="background:var(--panel3);border-radius:8px;padding:12px;margin-bottom:10px">' +
        '<div style="font-weight:600;margin-bottom:8px">DeepSeek API Key</div>' +
        '<div style="display:flex;gap:8px;align-items:center">' +
        '<input id="' + idn + '_key" type="text" value="' + esc(keyedModel.api_key || '') + '" placeholder="sk-..." style="flex:1;font-size:13px;padding:8px 10px;border:1px solid var(--border);border-radius:6px;background:var(--bg);color:var(--text)">' +
        '<button class="btn btn-primary" onclick="ModelSettings.saveQuickKey(\'' + idn + '\')" style="white-space:nowrap">💾 保存</button>' +
        '</div>' +
        '<div style="font-size:10px;color:var(--muted);margin-top:4px">当前：' + maskedKey + ' &nbsp;|&nbsp; <a href="#" onclick="RightPanel.switch(\'models\');this.closest(\'.modal-overlay\').remove()" style="color:var(--accent)">管理全部模型 →</a></div>' +
        '</div>' +
        '<div style="font-size:11px;color:var(--muted)">' +
        '<b>提示：</b>如需使用其他模型（Kimi、智谱、通义千问等），点击上方链接进入完整模型管理面板添加，再在 Agent 角色分配中绑定即可。' +
        '</div>',
      actions: [{ id: 'close', label: '关闭' }]
    });
  },

  saveQuickKey: async function (idn) {
    var key = document.getElementById(idn + '_key').value.trim();
    if (!key) { UI.toast('请输入 API Key', 'warn'); return; }
    var models = Store.state.models || [];
    var dsModel = models.find(function (m) { return m.name === 'deepseek-v4-pro'; });
    if (!dsModel) { UI.toast('未找到 deepseek-v4-pro 模型', 'error'); return; }
    try {
      await API.updateModel(dsModel.id, { api_key: key });
      UI.toast('API Key 已保存', 'success');
      document.querySelectorAll('.modal-overlay').forEach(function (m) { m.remove(); });
    } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
  }
};
