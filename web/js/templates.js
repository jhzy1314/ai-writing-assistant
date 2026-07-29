/* ============ templates.js：模板库（内置硬编码 + 后端自定义） ============ */
/* 内置模板（规格第七章）— 前端硬编码，点击直接填充 */
var BUILTIN_TEMPLATES = [
  {
    name: '小说章节续写', category: '小说创作', builtin: true,
    content: '根据下面需求创作小说章节。\n故事世界观：\n{world_setting}\n人物设定：\n{character_setting}\n前文剧情：\n{history_content}\n写作需求：\n{user_demand}\n要求：注重画面描写、人物心理、氛围感，人设不 OOC，剧情流畅自然。\n目标字数：{target_word}'
  },
  {
    name: '大纲搭建', category: '大纲创作', builtin: true,
    content: '基于需求搭建完整故事大纲，输出章节节点、核心冲突、人物行动逻辑。\n世界观设定：\n{world_setting}\n人物设定：\n{character_setting}\n创作需求：\n{user_demand}\n只输出结构框架，不要撰写大量正文。'
  },
  {
    name: '正式文案撰写', category: '文案撰写', builtin: true,
    content: '根据需求撰写正式文案，语言规范、结构清晰、信息准确、逻辑严谨。\n创作需求：\n{user_demand}\n背景资料：\n{material_text}\n目标字数：{target_word}'
  },
  {
    name: '文风模仿创作', category: '文风模仿', builtin: true,
    content: '参考范例文本的行文语感、叙事风格，按照该风格完成创作。\n范例文风样本：\n{style_sample}\n创作需求：\n{user_demand}'
  },
  {
    name: '全文逻辑自检', category: '工具类', builtin: true,
    content: '通读全文，排查所有问题：人设 OOC、世界观冲突、剧情逻辑漏洞、时间线矛盾、语句不通顺。逐条列出问题位置以及可行修改方案。文稿内容：{article_content}'
  },
  {
    name: '人物设定生成', category: '人物设定', builtin: true,
    content: '根据故事背景和需求，生成 1-3 个核心人物设定。每人包含：姓名、年龄、外貌、性格特征、背景故事、核心动机、行为底线。\n故事背景：{world_setting}\n需求：{user_demand}'
  },
  {
    name: '世界观构建', category: '世界观', builtin: true,
    content: '根据需求搭建完整的世界观设定体系。包括：时代背景、地理环境、力量体系/科技水平、社会制度、文化习俗、关键组织势力。\n创作需求：{user_demand}\n已有设定：{world_setting}'
  },
  {
    name: '场景描写扩写', category: '小说创作', builtin: true,
    content: '对以下场景进行深度扩写，强化画面感、感官描写（视觉/听觉/嗅觉/触觉）、氛围渲染。保留原剧情核心，不新增剧情转折。\n场景内容：{selected_text}\n目标字数：{target_word}'
  },
  {
    name: '对话写作', category: '小说创作', builtin: true,
    content: '根据人物设定和场景需求，创作自然流畅的人物对话。对话需符合角色性格，推动剧情发展，避免信息倾倒式对白。\n人物设定：{character_setting}\n场景需求：{user_demand}\n前文语境：{history_content}'
  },
  {
    name: '情感冲突设计', category: '剧情设计', builtin: true,
    content: '基于已有故事设定，设计情感冲突场景。明确冲突根源、双方立场、情绪曲线、转折点和化解方案。\n人物设定：{character_setting}\n当前剧情状态：{history_content}\n需求：{user_demand}'
  },
  {
    name: '篇章摘要提取', category: '工具类', builtin: true,
    content: '通读以下文本，提取核心要点，生成简洁摘要（100-150 字），保留关键信息和剧情节点。\n原文本：{article_content}'
  },
  {
    name: '高潮场景设计', category: '剧情设计', builtin: true,
    content: '基于当前故事进度，设计一个高张力剧情场景。包含：场景前因、冲突升级过程、关键转折、情绪爆发点、后续影响。\n人物设定：{character_setting}\n前文进度：{history_content}\n创作需求：{user_demand}'
  },
  {
    name: '日常过渡章节', category: '小说创作', builtin: true,
    content: '撰写日常过渡性章节，用于调节叙事节奏。展现角色日常生活、关系互动、为下一阶段冲突埋下伏笔。保持文风轻松但不失深度。\n人物设定：{character_setting}\n前文：{history_content}\n目标字数：{target_word}'
  },
  {
    name: '悬疑铺垫场景', category: '剧情设计', builtin: true,
    content: '设计悬疑/伏笔场景。使用细节暗示、信息差和时序技巧制造悬念。暗示但不点破关键信息，为后续反转铺垫。\n故事背景：{world_setting}\n前文铺垫：{history_content}\n需求：{user_demand}'
  },
  {
    name: '校园青春场景', category: '小说创作', builtin: true,
    content: '创作校园青春题材场景，注重少男少女的心理描写、朦胧情感、校园生活的真实细节。文风清新自然，避免过度煽情。\n人物设定：{character_setting}\n前文：{history_content}\n需求：{user_demand}\n目标字数：{target_word}'
  }
];

var TemplateUI = {
  cats: {},
  activeCat: '全部',
  init: function () {
    this.cats = {};
    var all = BUILTIN_TEMPLATES.concat(Store.state.templates);
    all.forEach(function (t) {
      var c = t.category || '未分类';
      if (!this.cats[c]) this.cats[c] = [];
      this.cats[c].push(t);
    }, this);
    this.renderCats();
    this.render();
  },
  renderCats: function () {
    var cats = ['全部'].concat(Object.keys(this.cats).sort());
    var self = this;
    document.getElementById('tplCats').innerHTML = cats.map(function (c) {
      return '<div class="tpl-cat' + (c === self.activeCat ? ' active' : '') + '" onclick="TemplateUI.setCat(\'' + esc(c) + '\')">' + esc(c) + '</div>';
    }).join('');
  },
  setCat: function (c) { this.activeCat = c; this.renderCats(); this.render(); },
  quickOpen: function () { RightPanel.switch('templates'); document.getElementById('tplSearch').focus(); },
  render: function (q) {
    q = (q || '').toLowerCase();
    var items = [];
    if (this.activeCat === '全部') {
      Object.values(this.cats).forEach(function (arr) { items = items.concat(arr); });
    } else items = this.cats[this.activeCat] || [];
    if (q) items = items.filter(function (t) {
      return (t.name || '').toLowerCase().includes(q) || (t.content || '').toLowerCase().includes(q);
    });
    var el = document.getElementById('tplList');
    if (!items.length) { el.innerHTML = '<div class="res-check-empty">未找到模板</div>'; return; }
    el.innerHTML = items.map(function (t, i) {
      var id = t.id || ('b' + i);
      var builtin = !!t.builtin || t.is_system;
      return '<div class="tpl-item" onclick="TemplateUI.fill(\'' + esc(id) + '\')">' +
        '<div class="n">' + esc(t.name) + '<span class="tag ' + (builtin ? 'builtin' : '') + '">' + (builtin ? '内置' : '自定义') + '</span></div>' +
        '<div class="c">' + esc((t.content || '').slice(0, 60)) + '</div>' +
        '<div class="acts">' +
        '<button class="tool-btn" onclick="event.stopPropagation();TemplateUI.fill(\'' + esc(id) + '\')">填充</button>' +
        (!builtin ? '<button class="tool-btn" onclick="event.stopPropagation();TemplateUI.edit(\'' + esc(id) + '\')">编辑</button><button class="tool-btn danger" onclick="event.stopPropagation();TemplateUI.del(\'' + esc(id) + '\')">删除</button>' : '') +
        '</div></div>';
    }).join('');
  },
  findTpl: function (id) {
    var t = BUILTIN_TEMPLATES.find(function (x) { return (x.id || x.name) === id; });
    if (t) return t;
    return Store.state.templates.find(function (x) { return x.id === id; });
  },
  fill: function (id) {
    var t = this.findTpl(id);
    if (!t) return;
    var filled = this.fillVars(t.content);
    var ta = document.getElementById('instructionInput');
    ta.value = filled;
    Editor.syncInstructionHeight(ta);
    UI.toast('已填充模板「' + t.name + '」，可调整后点击生成', 'success');
    ta.focus();
  },
  fillVars: function (content) {
    var world = Context.worldSetting();
    var char = Context.characters();
    var mat = Context.materials();
    var hist = Editor.getText();
    var tw = Store.state.composer.targetWord;
    var sel = Editor.getSelectedText();
    var ud = document.getElementById('instructionInput').value;
    return content
      .replace(/\{world_setting\}/g, world || '（未选择世界观）')
      .replace(/\{character_setting\}/g, char || '（未选择人物卡）')
      .replace(/\{history_content\}/g, hist ? hist.slice(0, 6000) : '（无前文）')
      .replace(/\{material_text\}/g, mat || '（未选择素材）')
      .replace(/\{target_word\}/g, tw)
      .replace(/\{user_demand\}/g, ud || '（请填写创作需求）')
      .replace(/\{selected_text\}/g, sel || '（无选中文本）')
      .replace(/\{article_content\}/g, hist ? hist.slice(0, 8000) : '（无文稿内容）')
      .replace(/\{style_sample\}/g, sel || '（请在此替换为范例文风样本）')
      .replace(/\{style_input\}/g, '（请填写目标文风）')
      .replace(/\{atmosphere_input\}/g, '（请填写目标氛围）');
  },
  editNew: function () { this.edit(null); },
  edit: function (id) {
    var t = id ? this.findTpl(id) : null;
    var ids = 'tp_' + uid();
    UI.modal({
      title: t ? '编辑模板' : '新建模板', wide: '560px',
      body: '<div class="form-group"><label>名称 *</label><input id="' + ids + '_name" value="' + esc(t ? t.name : '') + '"></div>' +
        '<div class="form-group"><label>分类</label><input id="' + ids + '_cat" value="' + esc(t ? t.category : '') + '" placeholder="如：小说章节 / 自定义"></div>' +
        '<div class="form-group"><label>内容（支持变量 {world_setting} {character_setting} {history_content} {material_text} {target_word} {user_demand} {selected_text} 等）</label><textarea id="' + ids + '_content" rows="8" style="font-size:12px">' + esc(t ? t.content : '') + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
            var name = document.getElementById(ids + '_name').value.trim();
            var cat = document.getElementById(ids + '_cat').value.trim() || '自定义';
            var content = document.getElementById(ids + '_content').value;
            if (!name || !content.trim()) { UI.toast('名称和内容必填', 'warn'); return; }
            ov.remove();
            TemplateUI.save(id, name, cat, content);
          }
        }
      ]
    });
  },
  save: async function (id, name, category, content) {
    try {
      if (id) {
        var t = await API.updateTemplate(id, { name: name, category: category, content: content });
        Object.assign(Store.state.templates.find(function (x) { return x.id === id; }) || {}, t);
      } else {
        var nt = await API.createTemplate({ name: name, category: category, content: content });
        Store.state.templates.push(nt);
      }
      this.init();
      UI.toast('模板已保存', 'success');
    } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
  },
  del: function (id) {
    UI.confirm('删除模板', '确认删除该自定义模板？', async function () {
      try {
        await API.deleteTemplate(id);
        Store.state.templates = Store.state.templates.filter(function (x) { return x.id !== id; });
        TemplateUI.init();
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  }
};
