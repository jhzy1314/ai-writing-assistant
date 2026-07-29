/* ============ resources.js：人物卡 / 世界观 / 素材 管理 + 上下文拼接 ============ */
/* 人物卡字段（姓名、外貌、性格、背景、行为底线、备注）打包进后端 description 字段
   世界观字段（标题、世界规则、势力分布、地理设定、力量体系）打包进后端 content 字段 */

var Context = {
  worldSetting: function () {
    return Store.state.worldSettings
      .filter(function (w) { return Store.state.selection.worldSettings.has(w.id); })
      .map(function (w) { return '《' + w.title + '》\n' + w.content; })
      .join('\n\n');
  },
  characters: function () {
    return Store.state.characters
      .filter(function (c) { return Store.state.selection.characters.has(c.id); })
      .map(function (c) { return '【' + c.name + '】\n' + c.description; })
      .join('\n\n');
  },
  materials: function () {
    return Store.state.materials
      .filter(function (m) { return Store.state.selection.materials.has(m.id); })
      .map(function (m) { return '《' + m.name + '》\n' + m.content; })
      .join('\n\n');
  }
};

var ResourceUI = {
  /* ---- 人物卡 ---- */
  editCharacter: function (id) {
    var c = id ? Store.state.characters.find(function (x) { return x.id === id; }) : null;
    var f = c ? this.unpackChar(c.description) : { appearance: '', personality: '', background: '', bottomline: '', notes: '' };
    var ids = 'c_' + uid();
    UI.modal({
      title: c ? '编辑人物卡' : '新建人物卡', wide: '520px',
      body: '<div class="form-group"><label>姓名 *</label><input id="' + ids + '_name" value="' + esc(c ? c.name : '') + '"></div>' +
        '<div class="form-group"><label>外貌</label><textarea id="' + ids + '_appearance" rows="2">' + esc(f.appearance) + '</textarea></div>' +
        '<div class="form-group"><label>性格</label><textarea id="' + ids + '_personality" rows="2">' + esc(f.personality) + '</textarea></div>' +
        '<div class="form-group"><label>背景</label><textarea id="' + ids + '_background" rows="2">' + esc(f.background) + '</textarea></div>' +
        '<div class="form-group"><label>行为底线</label><textarea id="' + ids + '_bottomline" rows="2">' + esc(f.bottomline) + '</textarea></div>' +
        '<div class="form-group"><label>备注</label><textarea id="' + ids + '_notes" rows="2">' + esc(f.notes) + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' }
      ].concat(c ? [{ id: 'del', label: '删除', cls: 'btn-danger', onClick: function (m, ov) { ov.remove(); ResourceUI.delCharacter(id); } }] : []).concat([
        {
          id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
            var name = document.getElementById(ids + '_name').value.trim();
            if (!name) { UI.toast('请输入姓名', 'warn'); return; }
            var fields = {
              appearance: document.getElementById(ids + '_appearance').value,
              personality: document.getElementById(ids + '_personality').value,
              background: document.getElementById(ids + '_background').value,
              bottomline: document.getElementById(ids + '_bottomline').value,
              notes: document.getElementById(ids + '_notes').value
            };
            ov.remove();
            ResourceUI.saveCharacter(id, name, fields);
          }
        }
      ])
    });
  },
  packChar: function (f) {
    var parts = [];
    if (f.appearance) parts.push('外貌：' + f.appearance);
    if (f.personality) parts.push('性格：' + f.personality);
    if (f.background) parts.push('背景：' + f.background);
    if (f.bottomline) parts.push('行为底线：' + f.bottomline);
    if (f.notes) parts.push('备注：' + f.notes);
    return parts.join('\n');
  },
  unpackChar: function (desc) {
    var f = { appearance: '', personality: '', background: '', bottomline: '', notes: '' };
    var map = { '外貌': 'appearance', '性格': 'personality', '背景': 'background', '行为底线': 'bottomline', '备注': 'notes' };
    (desc || '').split('\n').forEach(function (line) {
      var m = line.match(/^(外貌|性格|背景|行为底线|备注)：(.*)$/);
      if (m) f[map[m[1]]] = m[2];
    });
    return f;
  },
  saveCharacter: async function (id, name, fields) {
    var desc = this.packChar(fields);
    var pid = Store.state.currentProject.id;
    try {
      if (id) {
        var c = await API.updateCharacter(id, { name: name, description: desc });
        Object.assign(Store.state.characters.find(function (x) { return x.id === id; }) || {}, c);
      } else {
        var nc = await API.createCharacter({ project_id: pid, name: name, description: desc });
        Store.state.characters.push(nc);
      }
      Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
      UI.toast('人物卡已保存', 'success');
    } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
  },
  delCharacter: function (id) {
    var c = Store.state.characters.find(function (x) { return x.id === id; });
    if (!c) return;
    UI.confirm('删除人物卡', '确认删除「' + c.name + '」？', async function () {
      try {
        await API.deleteCharacter(id);
        Store.state.characters = Store.state.characters.filter(function (x) { return x.id !== id; });
        Store.state.selection.characters.delete(id);
        Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
        Store.saveSelection(Store.state.currentProject.id);
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  },

  /* ---- 世界观 ---- */
  editWorld: function (id) {
    var w = id ? Store.state.worldSettings.find(function (x) { return x.id === id; }) : null;
    var f = w ? this.unpackWorld(w.content) : { rules: '', forces: '', geography: '', powers: '' };
    var ids = 'w_' + uid();
    UI.modal({
      title: w ? '编辑世界观' : '新建世界观', wide: '560px',
      body: '<div class="form-group"><label>标题 *</label><input id="' + ids + '_title" value="' + esc(w ? w.title : '') + '"></div>' +
        '<div class="form-group"><label>世界规则</label><textarea id="' + ids + '_rules" rows="2">' + esc(f.rules) + '</textarea></div>' +
        '<div class="form-group"><label>势力分布</label><textarea id="' + ids + '_forces" rows="2">' + esc(f.forces) + '</textarea></div>' +
        '<div class="form-group"><label>地理设定</label><textarea id="' + ids + '_geography" rows="2">' + esc(f.geography) + '</textarea></div>' +
        '<div class="form-group"><label>力量体系</label><textarea id="' + ids + '_powers" rows="2">' + esc(f.powers) + '</textarea></div>',
      actions: [
        { id: 'cancel', label: '取消' }
      ].concat(w ? [{ id: 'del', label: '删除', cls: 'btn-danger', onClick: function (m, ov) { ov.remove(); ResourceUI.delWorld(id); } }] : []).concat([
        {
          id: 'ok', label: '保存', cls: 'btn-primary', onClick: function (m, ov) {
            var title = document.getElementById(ids + '_title').value.trim();
            if (!title) { UI.toast('请输入标题', 'warn'); return; }
            var fields = {
              rules: document.getElementById(ids + '_rules').value,
              forces: document.getElementById(ids + '_forces').value,
              geography: document.getElementById(ids + '_geography').value,
              powers: document.getElementById(ids + '_powers').value
            };
            ov.remove();
            ResourceUI.saveWorld(id, title, fields);
          }
        }
      ])
    });
  },
  packWorld: function (f) {
    var parts = [];
    if (f.rules) parts.push('世界规则：' + f.rules);
    if (f.forces) parts.push('势力分布：' + f.forces);
    if (f.geography) parts.push('地理设定：' + f.geography);
    if (f.powers) parts.push('力量体系：' + f.powers);
    return parts.join('\n');
  },
  unpackWorld: function (content) {
    var f = { rules: '', forces: '', geography: '', powers: '' };
    var map = { '世界规则': 'rules', '势力分布': 'forces', '地理设定': 'geography', '力量体系': 'powers' };
    (content || '').split('\n').forEach(function (line) {
      var m = line.match(/^(世界规则|势力分布|地理设定|力量体系)：(.*)$/);
      if (m) f[map[m[1]]] = m[2];
    });
    return f;
  },
  saveWorld: async function (id, title, fields) {
    var content = this.packWorld(fields);
    var pid = Store.state.currentProject.id;
    try {
      if (id) {
        var w = await API.updateWorldSetting(id, { title: title, content: content });
        Object.assign(Store.state.worldSettings.find(function (x) { return x.id === id; }) || {}, w);
      } else {
        var nw = await API.createWorldSetting({ project_id: pid, title: title, content: content });
        Store.state.worldSettings.push(nw);
      }
      Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
      UI.toast('世界观已保存', 'success');
    } catch (e) { UI.toast('保存失败：' + e.message, 'error'); }
  },
  delWorld: function (id) {
    var w = Store.state.worldSettings.find(function (x) { return x.id === id; });
    if (!w) return;
    UI.confirm('删除世界观', '确认删除「' + w.title + '」？', async function () {
      try {
        await API.deleteWorldSetting(id);
        Store.state.worldSettings = Store.state.worldSettings.filter(function (x) { return x.id !== id; });
        Store.state.selection.worldSettings.delete(id);
        Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
        Store.saveSelection(Store.state.currentProject.id);
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  },

  /* ---- 素材 ---- */
  uploadMaterial: function () {
    if (!Store.state.currentProject) { UI.toast('请先选择项目', 'warn'); return; }
    var idf = 'mf_' + uid();
    UI.modal({
      title: '上传素材文档',
      sub: '支持 TXT / Markdown / Word(.docx)。上传后可勾选作为生成上下文。',
      body: '<div class="form-group">' +
        '<label class="btn btn-ghost btn-block" style="cursor:pointer;text-align:center">选择文件…' +
        '<input type="file" id="' + idf + '" accept=".txt,.md,.markdown,.docx" style="display:none" onchange="document.getElementById(\'' + idf + '_n\').textContent=this.files[0]?this.files[0].name:\'\'">' +
        '</label><div id="' + idf + '_n" style="font-size:11.5px;color:var(--muted);margin-top:6px"></div></div>' +
        '<div class="form-group"><label>素材名称（可选）</label><input id="' + idf + '_name" placeholder="留空使用文件名"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        {
          id: 'ok', label: '上传', cls: 'btn-primary', onClick: function (m, ov) {
            var f = document.getElementById(idf).files[0];
            if (!f) { UI.toast('请选择文件', 'warn'); return; }
            ov.remove();
            ResourceUI.doUpload(f, document.getElementById(idf + '_name').value);
          }
        }
      ]
    });
  },
  doUpload: async function (file, name) {
    var pid = Store.state.currentProject.id;
    var fd = new FormData();
    fd.append('project_id', pid);
    fd.append('file', file);
    if (name) fd.append('name', name);
    try {
      UI.toast('上传中…', '');
      var m = await API.uploadMaterial(fd);
      Store.state.materials.push(m);
      Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
      UI.toast('素材已上传', 'success');
    } catch (e) { UI.toast('上传失败：' + e.message, 'error'); }
  },
  previewMaterial: function (id) {
    var m = Store.state.materials.find(function (x) { return x.id === id; });
    if (!m) return;
    UI.modal({ title: '素材预览 · ' + m.name, body: '<div class="result-box">' + esc(m.content || '(空)') + '</div>', wide: '640px' });
  },
  delMaterial: function (id) {
    var m = Store.state.materials.find(function (x) { return x.id === id; });
    if (!m) return;
    UI.confirm('删除素材', '确认删除「' + m.name + '」？', async function () {
      try {
        await API.deleteMaterial(id);
        Store.state.materials = Store.state.materials.filter(function (x) { return x.id !== id; });
        Store.state.selection.materials.delete(id);
        Sidebar.renderResources(); RightPanel.renderContext(); ProjectUI.updateMeta();
        Store.saveSelection(Store.state.currentProject.id);
        UI.toast('已删除', 'success');
      } catch (e) { UI.toast('删除失败：' + e.message, 'error'); }
    });
  }
};
