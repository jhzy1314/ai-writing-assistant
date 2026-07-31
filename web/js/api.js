/* ============ api.js：请求封装（REST + 上传） ============ */
var API = {
  base: '',
  req: async function (method, path, body, opts) {
    opts = opts || {};
    var init = { method: method, headers: { 'Accept': 'application/json' } };
    if (body !== undefined && !(body instanceof FormData)) {
      init.headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(body);
    } else if (body instanceof FormData) {
      init.body = body;
    }
    if (opts.signal) init.signal = opts.signal;
    var resp;
    try { resp = await fetch(this.base + path, init); }
    catch (e) { throw new Error('网络连接失败，请确认服务已启动'); }
    if (resp.status === 204) return null;
    var data = null;
    try { data = await resp.json(); } catch (e) { }
    if (!resp.ok) {
      var msg = (data && data.error) || ('请求失败(' + resp.status + ')');
      // 认证过期
      if (resp.status === 401 && data && data.code === 'auth_required') {
        if (typeof Auth !== 'undefined') Auth.showLogin();
        throw new Error('登录已过期，请重新登录');
      }
      // 人性化错误映射
      if (msg.indexOf('429') >= 0 || msg.indexOf('rate limit') >= 0 || msg.indexOf('TPD') >= 0) {
        msg = 'API 请求过于频繁，请稍后重试（约 30 秒）';
      } else if (msg.indexOf('401') >= 0 || msg.indexOf('unauthorized') >= 0) {
        msg = 'API 密钥无效，请在模型管理中更新密钥';
      } else if (msg.indexOf('503') >= 0 || msg.indexOf('unavailable') >= 0) {
        msg = '模型服务暂时不可用，请稍后重试';
      } else if (msg.indexOf('403') >= 0) {
        msg = 'API 访问被拒绝，请检查密钥权限';
      }
      throw new Error(msg);
    }
    return data;
  },
  get: function (p, o) { return this.req('GET', p, undefined, o); },
  post: function (p, b, o) { return this.req('POST', p, b, o); },
  put: function (p, b, o) { return this.req('PUT', p, b, o); },
  del: function (p, o) { return this.req('DELETE', p, undefined, o); },

  // 项目
  listProjects: function () { return this.get('/api/projects').then(function (d) { return d.items || []; }); },
  createProject: function (name, type) { return this.post('/api/projects', { name: name, type: type }).then(function (d) { return d.item; }); },
  getProject: function (id) { return this.get('/api/projects/' + id); },
  updateProject: function (id, nameOrObj, type) {
    // 兼容两种调用方式: (id, name, type) 或 (id, { name, type, outline })
    var body;
    if (typeof nameOrObj === 'object') {
      body = nameOrObj;
    } else {
      body = { name: nameOrObj, type: type };
    }
    return this.put('/api/projects/' + id, body).then(function (d) { return d.item; });
  },
  deleteProject: function (id) { return this.del('/api/projects/' + id); },
  duplicateProject: function (id) { return this.post('/api/projects/' + id + '/duplicate'); },
  // 版本
  listVersions: function (pid) { return this.get('/api/projects/' + pid + '/versions').then(function (d) { return d.items || []; }); },
  saveVersion: function (pid, title, content) { return this.post('/api/versions', { project_id: pid, title: title, content: content }).then(function (d) { return d.item; }); },
  getVersion: function (id) { return this.get('/api/versions/' + id).then(function (d) { return d.item; }); },
  // 人物卡
  listCharacters: function (pid) { return this.get('/api/characters?project_id=' + pid).then(function (d) { return d.items || []; }); },
  createCharacter: function (c) { return this.post('/api/characters', c).then(function (d) { return d.item; }); },
  updateCharacter: function (id, c) { return this.put('/api/characters/' + id, c).then(function (d) { return d.item; }); },
  deleteCharacter: function (id) { return this.del('/api/characters/' + id); },
  // 世界观
  listWorldSettings: function (pid) { return this.get('/api/worldsettings?project_id=' + pid).then(function (d) { return d.items || []; }); },
  createWorldSetting: function (w) { return this.post('/api/worldsettings', w).then(function (d) { return d.item; }); },
  updateWorldSetting: function (id, w) { return this.put('/api/worldsettings/' + id, w).then(function (d) { return d.item; }); },
  deleteWorldSetting: function (id) { return this.del('/api/worldsettings/' + id); },
  // 素材
  listMaterials: function (pid) { return this.get('/api/materials?project_id=' + pid).then(function (d) { return d.items || []; }); },
  uploadMaterial: function (fd) { return this.post('/api/materials/upload', fd).then(function (d) { return d.item; }); },
  deleteMaterial: function (id) { return this.del('/api/materials/' + id); },
  // 模板
  listTemplates: function () { return this.get('/api/templates').then(function (d) { return d.items || []; }); },
  createTemplate: function (t) { return this.post('/api/templates', t).then(function (d) { return d.item; }); },
  updateTemplate: function (id, t) { return this.put('/api/templates/' + id, t).then(function (d) { return d.item; }); },
  deleteTemplate: function (id) { return this.del('/api/templates/' + id); },
  // 模型
  listModels: function () { return this.get('/api/models').then(function (d) { return d.items || []; }); },
  createModel: function (m) { return this.post('/api/models', m).then(function (d) { return d.item; }); },
  updateModel: function (id, m) { return this.put('/api/models/' + id, m).then(function (d) { return d.item; }); },
  deleteModel: function (id) { return this.del('/api/models/' + id); },
  testModel: function (id) { return this.post('/api/models/' + id + '/test'); },
  setDefaultModel: function (id) { return this.put('/api/models/' + id + '/default').then(function (d) { return d.item; }); },
  // 网页AI模型
  createWebAIModel: function (m) { return this.post('/api/webai/models', m).then(function (d) { return d.item; }); },
  updateWebAIModel: function (id, m) { return this.put('/api/webai/models/' + id, m).then(function (d) { return d.item; }); },
  testWebAIModel: function (cfg) { return this.post('/api/webai/test', cfg); },
  listWebAIProviders: function () { return this.get('/api/webai/providers').then(function (d) { return d.providers || {}; }); },
  autoCookieStart: function (provider) { return this.post('/api/webai/auto-cookie', { provider: provider }); },
  autoCookiePoll: function (sessionId) { return this.get('/api/webai/auto-cookie/' + sessionId); },
  autoCookieCancel: function (sessionId) { return this.del('/api/webai/auto-cookie/' + sessionId); },
  // 卷
  listVolumes: function (pid) { return this.get('/api/projects/' + pid + '/volumes').then(function (d) { return d.items || []; }); },
  createVolume: function (v) { return this.post('/api/volumes', v).then(function (d) { return d.item; }); },
  updateVolume: function (id, v) { return this.put('/api/volumes/' + id, v).then(function (d) { return d.item; }); },
  deleteVolume: function (id) { return this.del('/api/volumes/' + id); },
  reorderVolumes: function (items) { return this.post('/api/volumes/reorder', { items: items }); },
  // 章节
  listChapters: function (pid, vid) { return this.get('/api/chapters?project_id=' + pid + (vid ? '&volume_id=' + vid : '')).then(function (d) { return d.items || []; }); },
  createChapter: function (c) { return this.post('/api/chapters', c).then(function (d) { return d.item; }); },
  getChapter: function (id) { return this.get('/api/chapters/' + id).then(function (d) { return d.item; }); },
  updateChapter: function (id, c) { return this.put('/api/chapters/' + id, c).then(function (d) { return d.item; }); },
  deleteChapter: function (id) { return this.del('/api/chapters/' + id); },
  copyChapter: function (id) { return this.post('/api/chapters/' + id + '/copy').then(function (d) { return d.item; }); },
  reorderChapters: function (items) { return this.post('/api/chapters/reorder', { items: items }); },
  // 章节版本
  listChapterVersions: function (cid) { return this.get('/api/chapters/' + cid + '/versions').then(function (d) { return d.items || []; }); },
  saveChapterVersion: function (cid, title, content) { return this.post('/api/chapters/' + cid + '/versions', { title: title, content: content }).then(function (d) { return d.item; }); },
  getChapterVersion: function (id) { return this.get('/api/chapters/versions/' + id).then(function (d) { return d.item; }); },
  // 章节导入导出
  exportChapters: function (pid) { return this.get('/api/chapters/export?project_id=' + pid).then(function (d) { return d.item; }); },
  importChapters: function (data) { return this.post('/api/chapters/import', data); },
  splitChapters: function (pid, content, splitBy) { return this.post('/api/chapters/split', { project_id: pid, content: content, split_by: splitBy || 'auto' }); },
  mergeChapters: function (ids, title) { return this.post('/api/chapters/merge', { chapter_ids: ids, title: title }).then(function (d) { return d.item; }); },
  splitChapterAtCursor: function (id, cursorPos) { return this.post('/api/chapters/' + id + '/split', { cursor_pos: cursorPos }).then(function (d) { return d.items; }); },
  getProjectStats: function (pid) { return this.get('/api/projects/' + pid + '/stats').then(function (d) { return d.item; }); },
  // 回收站
  listTrashChapters: function (pid) { return this.get('/api/chapters/trash?project_id=' + (pid || '')).then(function (d) { return d.items || []; }); },
  restoreChapter: function (id) { return this.post('/api/chapters/' + id + '/restore'); },
  permanentDeleteChapter: function (id, confirm) { return this.post('/api/chapters/' + id + '/permanent-delete', { confirm: confirm }); },
  // 用量
  getUsage: function () { return this.get('/api/usage'); },
  // 逻辑自检
  verify: function (content, world, character) {
    return this.post('/api/verify', { content: content, world_setting: world || '', character_setting: character || '' });
  },
  aiTells: function (payload) { return this.post('/api/ai-tells', payload); },
  aiPolish: function (payload) { return this.post('/api/ai-polish', payload); },
  // 角色-模型映射
  getRoleModels: function (role) { return this.get('/api/roles/' + role + '/models'); },
  setRoleModels: function (role, modelIds) { return this.put('/api/roles/' + role + '/models', { model_ids: modelIds }); },
  // 背景外观
  getBackgrounds: function () { return this.get('/api/appearance/backgrounds'); },
  setBackground: function (type, theme, file) { return this.post('/api/appearance/backgrounds/set', { type: type, theme: theme, file: file }); },
  uploadBackground: function (fd) { return this.post('/api/appearance/backgrounds/upload', fd); },
  generateBackground: function (type, theme, prompt) { return this.post('/api/appearance/backgrounds/generate', { type: type, theme: theme, prompt: prompt }); },
  randomBackground: function (type, theme) { return this.post('/api/appearance/backgrounds/random?type=' + type + '&theme=' + theme); },
  resetBackgrounds: function () { return this.post('/api/appearance/backgrounds/reset', {}); }
};
