/* ============ pages-characters.js：人物卡管理页面 ============ */
var CharacterPage = {
  init: function () {
    var p = Store.state.currentProject;
    if (!p) return;
    this.load();
  },
  load: async function () {
    var p = Store.state.currentProject;
    if (!p) { this.showEmpty('请先在左侧选中一个项目'); return; }
    try {
      Store.state.characters = await API.listCharacters(p.id);
      this.render();
    } catch (e) {
      document.getElementById('charGrid').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">⚠️</div><div>加载失败</div></div>';
    }
  },
  render: function () {
    var grid = document.getElementById('charGrid');
    var search = (document.getElementById('charSearch') || { value: '' }).value.toLowerCase();
    var chars = Store.state.characters || [];
    if (search) {
      chars = chars.filter(function (c) {
        var n = (c.name || '').toLowerCase();
        var d = (c.description || c.bio || '').toLowerCase();
        return n.indexOf(search) >= 0 || d.indexOf(search) >= 0;
      });
    }
    if (!chars.length) {
      grid.innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">👤</div><div>' + (search ? '未找到匹配人物' : '暂无人物卡，点击上方按钮创建') + '</div></div>';
      return;
    }
    var html = '';
    chars.forEach(function (c) {
      html += '<div class="char-card" onclick="CharacterPage.edit(\'' + c.id + '\')">';
      html += '<div class="char-card-avatar" style="background:' + CharacterPage.colorFor(c.name) + '">' + (c.name || '?')[0] + '</div>';
      html += '<div class="char-card-body">';
      html += '<div class="char-card-name">' + esc(c.name || '未命名') + '</div>';
      var desc = c.description || c.bio || c.role || '';
      if (desc.length > 80) desc = desc.substring(0, 80) + '...';
      html += '<div class="char-card-desc">' + esc(desc) + '</div>';
      html += '<div class="char-card-meta">';
      if (c.gender) html += '<span class="char-tag">' + esc(c.gender) + '</span>';
      if (c.role) html += '<span class="char-tag char-tag-role">' + esc(c.role) + '</span>';
      html += '</div>';
      html += '</div>';
      html += '<div class="char-card-acts">';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();CharacterPage.edit(\'' + c.id + '\')" title="编辑">✏️</span>';
      html += '<span class="char-act-btn" onclick="event.stopPropagation();CharacterPage.del(\'' + c.id + '\')" title="删除">🗑</span>';
      html += '</div>';
      html += '</div>';
    });
    grid.innerHTML = html;
  },
  colorFor: function (name) {
    var colors = ['#e8945a', '#4facfe', '#43e97b', '#fa709a', '#fee140', '#667eea', '#f5576c', '#6a3093'];
    var hash = 0;
    for (var i = 0; i < (name || 'a').length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
    return colors[Math.abs(hash) % colors.length];
  },
  showCreate: function () {
    this.edit(null);
  },
  edit: function (id) {
    var c = null;
    if (id) {
      var chars = Store.state.characters || [];
      for (var i = 0; i < chars.length; i++) { if (chars[i].id === id) { c = chars[i]; break; } }
    }
    var isNew = !c;
    var html = '<div class="form-group"><label>姓名 *</label><input id="charName" value="' + escAttr(c ? c.name || '' : '') + '"></div>';
    html += '<div class="form-row"><div class="form-group"><label>性别</label><select id="charGender"><option value="">--</option><option value="男"' + (c && c.gender === '男' ? ' selected' : '') + '>男</option><option value="女"' + (c && c.gender === '女' ? ' selected' : '') + '>女</option><option value="其他"' + (c && c.gender === '其他' ? ' selected' : '') + '>其他</option></select></div>';
    html += '<div class="form-group"><label>年龄</label><input id="charAge" value="' + escAttr(c ? c.age || '' : '') + '"></div></div>';
    html += '<div class="form-group"><label>角色定位</label><input id="charRole" value="' + escAttr(c ? c.role || '' : '') + '" placeholder="主角/反派/配角..."></div>';
    html += '<div class="form-group"><label>外貌特征</label><textarea id="charAppearance" rows="2">' + esc(c ? c.appearance || '' : '') + '</textarea></div>';
    html += '<div class="form-group"><label>性格描述</label><textarea id="charPersonality" rows="2">' + esc(c ? c.personality || '' : '') + '</textarea></div>';
    html += '<div class="form-group"><label>背景故事</label><textarea id="charBackground" rows="3">' + esc(c ? c.background || '' : '') + '</textarea></div>';
    html += '<div class="form-group"><label>能力/技能</label><textarea id="charAbilities" rows="2">' + esc(c ? c.abilities || '' : '') + '</textarea></div>';
    html += '<div class="form-group"><label>关系网络</label><textarea id="charRelations" rows="2" placeholder="人物A: 师徒关系...">' + esc(c ? c.relations || '' : '') + '</textarea></div>';
    html += '<div class="form-group"><label>备注</label><textarea id="charNotes" rows="2">' + esc(c ? c.notes || '' : '') + '</textarea></div>';

    var self = this;
    UI.modal({
      title: (isNew ? '新建' : '编辑') + '人物卡',
      body: html,
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: isNew ? '创建' : '保存', cls: 'btn-primary', onClick: function (m, ov) {
          var data = {
            name: document.getElementById('charName').value.trim(),
            gender: document.getElementById('charGender').value,
            age: document.getElementById('charAge').value,
            role: document.getElementById('charRole').value.trim(),
            appearance: document.getElementById('charAppearance').value.trim(),
            personality: document.getElementById('charPersonality').value.trim(),
            background: document.getElementById('charBackground').value.trim(),
            abilities: document.getElementById('charAbilities').value.trim(),
            relations: document.getElementById('charRelations').value.trim(),
            notes: document.getElementById('charNotes').value.trim()
          };
          if (!data.name) { UI.toast('请输入人物姓名', 'warn'); return; }
          data.description = data.role;
          data.gender = data.gender || undefined;
          ov.remove();
          self.save(id, data);
        }}
      ]
    });
  },
  save: async function (id, data) {
    var p = Store.state.currentProject;
    if (!p) { UI.toast('请先在左侧选择项目', 'warn'); return; }
    data.project_id = p.id;
    try {
      if (id) { await API.updateCharacter(id, data); }
      else { await API.createCharacter(data); }
      UI.toast(id ? '已保存' : '已创建', 'success');
      this.load();
    } catch (e) { UI.toast('保存失败: ' + e.message, 'error'); }
  },
  del: function (id) {
    var self = this;
    UI.confirm('删除人物卡', '确定删除此人物卡？此操作不可撤销！', function () {
      API.deleteCharacter(id).then(function () {
        UI.toast('已删除', 'success');
        self.load();
      }).catch(function (e) { UI.toast('删除失败: ' + e.message, 'error'); });
    });
  },
  importFromText: function () {
    var self = this;
    UI.modal({
      title: '从正文提取人物卡',
      body: '<p style="font-size:12px;color:var(--muted);margin-bottom:10px">粘贴正文片段，AI 将自动识别并提取人物信息</p><textarea id="charImportText" rows="8" style="width:100%" placeholder="粘贴正文内容..."></textarea>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '提取', cls: 'btn-primary', onClick: function (m, ov) {
          var text = document.getElementById('charImportText').value.trim();
          if (!text) { UI.toast('请输入正文内容', 'warn'); return; }
          ov.remove();
          self.doImport(text);
        }}
      ]
    });
  },
  doImport: async function (text) {
    UI.toast('正在分析人物...', 'info');
    try {
      var resp = await API.req('POST', '/api/tools/execute', {
        tool: 'extract_characters',
        project_id: Store.state.currentProject.id,
        text: text
      });
      if (resp && resp.result) {
        var self = this;
        UI.modal({
          title: '提取结果',
          body: '<div class="result-box" style="max-height:60vh;overflow-y:auto;white-space:pre-wrap">' + esc(typeof resp.result === 'string' ? resp.result : JSON.stringify(resp.result, null, 2)) + '</div>',
          actions: [
            { id: 'cancel', label: '关闭' },
            { id: 'refresh', label: '刷新列表', cls: 'btn-primary', onClick: function (m, ov) { ov.remove(); self.load(); } }
          ]
        });
      }
    } catch (e) { UI.toast('提取失败: ' + e.message, 'error'); }
  },
  showEmpty: function (msg) {
    document.getElementById('charGrid').innerHTML = '<div class="page-empty-state"><div class="page-empty-icon">👤</div><div>' + msg + '</div></div>';
  }
};
