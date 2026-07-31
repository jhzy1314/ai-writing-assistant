/* ============ appearance.js：背景外观设置模块 ============ */
var Appearance = {
  _data: null,

  init: function () {
    this.load();
    var dz = document.getElementById('appearanceDropZone');
    if (dz) {
      dz.addEventListener('dragover', function (e) { e.preventDefault(); dz.classList.add('dragover'); });
      dz.addEventListener('dragleave', function () { dz.classList.remove('dragover'); });
      dz.addEventListener('drop', function (e) {
        e.preventDefault(); dz.classList.remove('dragover');
        var files = e.dataTransfer.files;
        if (files.length > 0) { Appearance.uploadFile(files[0]); }
      });
    }
  },

  load: async function () {
    try {
      var data = await API.getBackgrounds();
      this._data = data;
      this.renderAll(data);
      this.applyCurrent(data.current, data.current_files);
    } catch (e) {
      console.log('[Appearance] 加载失败:', e && e.message);
    }
  },

  renderAll: function (data) {
    this.renderGrid('bgSiderDark', 'sider', 'dark', data);
    this.renderGrid('bgSiderLight', 'sider', 'light', data);
    this.renderGrid('bgEmpty', 'empty', 'light', data);
    this.renderCustom(data.custom || []);
    this.updateLabels(data.current_files);
  },

  renderGrid: function (containerId, type, theme, data) {
    var container = document.getElementById(containerId);
    if (!container) return;
    var html = '';
    var library = data.library || [];
    var cf = data.current_files || {};
    var currentKey = type === 'sider' ? (theme === 'dark' ? cf.sider : cf.sider_light) : cf.empty;

    library.forEach(function (bg) {
      if (bg.type !== type || bg.theme !== theme) return;
      var isActive = (bg.file === currentKey);
      html += '<div class="bg-preview-card' + (isActive ? ' active' : '') + '" onclick="Appearance.apply(\'' + bg.type + '\',\'' + bg.theme + '\',\'' + bg.file + '\')">';
      html += '  <div class="bg-thumb" style="background-image:url(' + bg.url + ')">';
      if (theme === 'dark') html += '<span style="color:#888;font-size:10px">◑</span>';
      else html += '<span style="color:#bbb;font-size:10px">◐</span>';
      html += '  </div>';
      html += '  <div class="bg-label">' + esc(bg.name) + '</div>';
      html += '  <div class="bg-check">✓</div>';
      html += '</div>';
    });
    container.innerHTML = html || '<div style="font-size:10px;color:var(--faint);padding:4px 0">暂无可用背景</div>';
  },

  renderCustom: function (custom) {
    var container = document.getElementById('bgCustom');
    if (!container) return;
    if (!custom || custom.length === 0) {
      container.innerHTML = '<div style="font-size:10px;color:var(--faint);padding:4px 0">暂无自定义背景<br>点击下方上传或生成</div>';
      return;
    }
    var html = '';
    custom.forEach(function (bg) {
      html += '<div class="bg-preview-card" onclick="Appearance.apply(\'' + bg.type + '\',\'' + bg.theme + '\',\'' + bg.file + '\')">';
      html += '  <div class="bg-thumb" style="background-image:url(' + bg.url + ')">📄</div>';
      html += '  <div class="bg-label">' + esc(bg.name) + '</div>';
      html += '</div>';
    });
    container.innerHTML = html;
  },

  updateLabels: function (files) {
    if (!files) return;
    var el = function (id) { return document.getElementById(id); };
    if (el('curSider')) el('curSider').textContent = files.sider || '默认';
    if (el('curSiderLight')) el('curSiderLight').textContent = files.sider_light || '默认';
    if (el('curEmpty')) el('curEmpty').textContent = files.empty || '默认';
  },

  applyCurrent: function (current, files) {
    if (!current) return;
    var root = document.documentElement;
    if (current.sider_bg) {
      root.style.setProperty('--sider-bg', 'url(' + current.sider_bg + ')');
    }
    if (current.sider_light_bg) {
      root.style.setProperty('--sider-light-bg', 'url(' + current.sider_light_bg + ')');
    }
    if (current.empty_bg) {
      root.style.setProperty('--empty-bg', 'url(' + current.empty_bg + ')');
    }
  },

  // P3-1 修复：主题切换时背景变量联动。
  // 根据当前主题的明暗 mode，确保侧栏/空状态背景变量指向对应主题背景文件。
  // 若外观面板已配置自定义背景（_data.current_files），优先复用；否则用主题 CSS 默认值。
  applyTheme: function () {
    var root = document.documentElement;
    var mode = 'dark';
    try {
      if (typeof Themes !== 'undefined' && Themes.mode) mode = Themes.mode();
    } catch (e) {}
    var d = this._data;
    var files = (d && d.current_files) || {};
    // 亮色主题用 sider_light 背景，暗色用 sider 背景
    if (mode === 'light') {
      if (files.sider_light_bg) root.style.setProperty('--sider-bg', 'url(' + files.sider_light_bg + ')');
    } else {
      if (files.sider_bg) root.style.setProperty('--sider-bg', 'url(' + files.sider_bg + ')');
    }
    if (files.empty_bg) root.style.setProperty('--empty-bg', 'url(' + files.empty_bg + ')');
  },

  apply: async function (type, theme, file) {
    try {
      await API.setBackground(type, theme, file);
      UI.toast('背景已切换', 'success');
      await this.load();
    } catch (e) {
      UI.toast('切换失败: ' + e.message, 'error');
    }
  },

  uploadBg: function (type, theme) {
    var input = document.getElementById('appearanceFileInput');
    input.setAttribute('data-type', type);
    input.setAttribute('data-theme', theme);
    input.click();
  },

  onFileSelected: async function (e) {
    var file = e.target.files && e.target.files[0];
    if (!file) return;
    var type = e.target.getAttribute('data-type') || 'sider';
    var theme = e.target.getAttribute('data-theme') || 'dark';
    await this.uploadFile(file, type, theme);
    e.target.value = '';
  },

  uploadFile: async function (file, type, theme) {
    if (!type) type = 'sider';
    if (!theme) theme = 'dark';
    var fd = new FormData();
    fd.append('file', file);
    fd.append('type', type);
    fd.append('theme', theme);
    try {
      await API.uploadBackground(fd);
      UI.toast('上传成功并已应用', 'success');
      await this.load();
    } catch (e) {
      UI.toast('上传失败: ' + e.message, 'error');
    }
  },

  uploadCustom: function () {
    var input = document.getElementById('appearanceFileInput');
    input.setAttribute('data-type', '');
    input.setAttribute('data-theme', '');
    input.click();
  },

  generateBg: async function (type, theme) {
    var placeholderDiv = document.createElement('div');
    placeholderDiv.className = 'bg-preview-card';
    placeholderDiv.style.cssText = 'opacity:0.5;cursor:default';
    placeholderDiv.innerHTML = '<div class="bg-thumb" style="background:var(--panel3);display:flex;align-items:center;justify-content:center;font-size:24px">🎲</div><div class="bg-label" style="font-size:10px">生成中...</div>';
    var customEl = document.getElementById('bgCustom');
    if (customEl) customEl.appendChild(placeholderDiv);
    try {
      UI.toast('正在从 Picsum 生成背景...', '');
      var data = await API.generateBackground(type, theme, '');
      if (data && data.url) {
        UI.toast('生成成功并已应用', 'success');
        await this.load();
      }
    } catch (e) {
      if (customEl && placeholderDiv.parentNode) {
        placeholderDiv.innerHTML = '<div class="bg-thumb" style="background:var(--panel3);display:flex;align-items:center;justify-content:center;color:var(--muted);font-size:11px">生成失败<br>请检查网络</div><div class="bg-label" style="font-size:10px;color:var(--danger)">失败</div>';
      }
      UI.toast('背景生成失败，请检查网络后重试', 'warn');
    }
  },

  randomBg: async function (type, theme) {
    try {
      UI.toast('正在随机切换...', '');
      await API.randomBackground(type, theme);
      UI.toast('随机切换成功', 'success');
      await this.load();
    } catch (e) {
      UI.toast('随机切换失败，自动使用系统模板', 'warn');
      await this.resetAll();
    }
  },

  resetAll: async function () {
    try {
      await API.resetBackgrounds();
      UI.toast('已恢复默认背景', 'success');
      var root = document.documentElement;
      root.style.removeProperty('--sider-bg');
      root.style.removeProperty('--sider-light-bg');
      root.style.removeProperty('--empty-bg');
      await this.load();
    } catch (e) {
      UI.toast('恢复失败: ' + e.message, 'error');
    }
  }
};

function esc(str) {
  if (!str) return '';
  var div = document.createElement('div');
  div.appendChild(document.createTextNode(str));
  return div.innerHTML;
}
