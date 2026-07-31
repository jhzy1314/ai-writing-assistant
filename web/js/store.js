/* ============ store.js：全局状态管理 + 本地持久化 ============ */
var Store = {
  state: {
    projects: [],
    currentProject: null,
    versions: [],
    latestVersion: null,
    characters: [],
    worldSettings: [],
    materials: [],
    templates: [],
    models: [],
    volumes: [],
    chapters: [],
    currentChapter: null,
    usage: null,
    selection: { characters: new Set(), worldSettings: new Set(), materials: new Set() },
    editor: { mode: 'rich', preview: false, selectedText: '' },
    composer: { runMode: 'auto', targetWord: 1000, modelName: '', modelConfigId: '', orchThinker: '', orchWorker: '', orchVerifier: '', cursorPosition: 0, contextScope: 'smart', noRewrite: false, autoAppend: true, styleChapterId: '', styleIntensity: 'medium', skipWordCheck: false },
    chapterSummaries: '',
    pipeline: { active: false, stage: '等待生成', role: '', progress: 0, roleKey: 'idle', log: [], warn: '',
      steps: [], outline: '', issues: [], models: {}, degraded: null },
    verify: { result: null }
  },
  lsPrefix: 'aiNovel.',
  get: function (key, def) {
    try { var v = localStorage.getItem(this.lsPrefix + key); return v === null ? def : JSON.parse(v); }
    catch (e) { return def; }
  },
  set: function (key, val) { try { localStorage.setItem(this.lsPrefix + key, JSON.stringify(val)); } catch (e) { console.warn('[store] localStorage quota exceeded for key:', key, ' Consider clearing old drafts or trash.'); } },
  remove: function (key) { try { localStorage.removeItem(this.lsPrefix + key); } catch (e) { } },
  loadPrefs: function () {
    this.state.composer.runMode = this.get('runMode', 'auto');
    this.state.composer.targetWord = this.get('targetWord', 1000);
    this.state.composer.modelName = this.get('modelName', '');
    this.state.composer.contextScope = this.get('contextScope', 'smart');
    this.state.composer.skipWordCheck = this.get('skipWordCheck', false);
    this.state.composer.outline = this.get('outline', '');
    this.state.composer.webSearch = this.get('webSearch', false);
    this.state.editor.mode = this.get('editorMode', 'rich');
    // 主题：多主题系统（themes.js），经典主题 id 保持 dark/light 兼容旧值
    if (typeof Themes !== 'undefined' && Themes.loadSaved) {
      Themes.loadSaved();
      document.documentElement.setAttribute('data-theme', Themes.current);
      var tg = document.getElementById('themeToggle');
      if (tg) tg.textContent = Themes.get(Themes.current).icon + ' ' + Themes.get(Themes.current).name;
    } else {
      var theme = this.get('theme', 'dark');
      document.documentElement.setAttribute('data-theme', theme);
      var tg2 = document.getElementById('themeToggle');
      if (tg2) tg2.textContent = theme === 'dark' ? '☀ 浅色' : '🌙 深色';
    }
  },
  savePrefs: function () {
    this.set('runMode', this.state.composer.runMode);
    this.set('targetWord', this.state.composer.targetWord);
    this.set('modelName', this.state.composer.modelName);
    this.set('editorMode', this.state.editor.mode);
    this.set('contextScope', this.state.composer.contextScope);
    this.set('skipWordCheck', this.state.composer.skipWordCheck);
    this.set('webSearch', !!this.state.composer.webSearch);
    this.set('outline', this.state.composer.outline || '');
  },
  saveSelection: function (projectId) {
    if (!projectId) return;
    var s = this.state.selection;
    this.set('sel.' + projectId, {
      characters: Array.from(s.characters),
      worldSettings: Array.from(s.worldSettings),
      materials: Array.from(s.materials)
    });
  },
  loadSelection: function (projectId) {
    var s = this.get('sel.' + projectId, null);
    this.state.selection = {
      characters: new Set(s ? s.characters : []),
      worldSettings: new Set(s ? s.worldSettings : []),
      materials: new Set(s ? s.materials : [])
    };
  },
  draftKey: function (projectId) { return 'draft.' + projectId; },
  saveDraft: function (projectId, text) {
    if (!projectId) return;
    this.set(this.draftKey(projectId), { text: text, ts: Date.now() });
  },
  getDraft: function (projectId) { return this.get(this.draftKey(projectId), null); },
  clearDraft: function (projectId) { this.remove(this.draftKey(projectId)); }
};
