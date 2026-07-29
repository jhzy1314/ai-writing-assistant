/* ============ selection.js：选中文字悬浮快捷指令 ============ */
/* 选中文字快捷指令模板（规格第二章 1.2 / 第七章） */
var SELECTION_TEMPLATES = {
  expand: '扩写下面这段文字，增加细节描写，保留原本剧情、核心意思不变，文字自然流畅：【{selected_text}】',
  condense: '精简这段文字，去除冗余语句，保留全部核心信息，不要丢失关键剧情：【{selected_text}】',
  rewrite: '改写下面文本，保持剧情不变，调整行文风格为【{style_input}】。文本：【{selected_text}】',
  polish: '优化下面文字语句通顺度，修正语病，不改动剧情与内容：【{selected_text}】',
  continue: '承接下面文本继续向后续写，保持文风统一，人设统一，不要重复上文内容：【{selected_text}】',
  summary: '为这段文字生成简短摘要，提炼核心内容：【{selected_text}】',
  atmosphere: '调整下面文字的氛围，使其符合【{atmosphere_input}】的意境，保留原剧情与核心信息：【{selected_text}】'
};

var SelectionActions = {
  fillTemplate: function (key, extra) {
    var sel = Editor.getSelectedText();
    var tpl = SELECTION_TEMPLATES[key];
    var content = tpl.replace(/\{selected_text\}/g, sel || '');
    if (extra) {
      content = content.replace(/\{style_input\}/g, extra.style || '')
        .replace(/\{atmosphere_input\}/g, extra.atmosphere || '');
    }
    document.getElementById('instructionInput').value = content;
    Editor.syncInstructionHeight(document.getElementById('instructionInput'));
    Editor.hideSelToolbar();
    if (sel && Array.from(sel).length > 500 && Store.state.composer.runMode === 'light') {
      UI.toast('选中文本超过 500 字，建议切换为「智能协同」模式', 'warn');
    }
    // 自动切换到轻量模式并触发生成
    Store.state.composer.runMode = 'light';
    document.getElementById('modeSelect').value = 'light';
    Composer.onModeChange('light', true);
    UI.toast('已填充指令（轻量模式），即将生成…', '');
    setTimeout(function () { Composer.generate(); }, 300);
  },
  expand: function () { this.fillTemplate('expand'); },
  condense: function () { this.fillTemplate('condense'); },
  polish: function () { this.fillTemplate('polish'); },
  continue_: function () { this.fillTemplate('continue'); },
  summary: function () { this.fillTemplate('summary'); },
  rewrite: function () {
    var id = 'rw_' + uid();
    UI.modal({
      title: '改写文风',
      body: '<div class="form-group"><label>目标文风描述（如：武侠古风 / 现代白话 / 诗意冷峻）</label><input id="' + id + '" placeholder="输入文风描述"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '确定', cls: 'btn-primary', onClick: function (m, ov) { var v = document.getElementById(id).value.trim(); if (!v) { UI.toast('请输入文风', 'warn'); return; } ov.remove(); SelectionActions.fillTemplate('rewrite', { style: v }); } }
      ]
    });
  },
  atmosphere: function () {
    var id = 'at_' + uid();
    UI.modal({
      title: '调整氛围',
      body: '<div class="form-group"><label>目标氛围描述（如：压抑沉重 / 欢快明丽 / 神秘诡谲）</label><input id="' + id + '" placeholder="输入氛围描述"></div>',
      actions: [
        { id: 'cancel', label: '取消' },
        { id: 'ok', label: '确定', cls: 'btn-primary', onClick: function (m, ov) { var v = document.getElementById(id).value.trim(); if (!v) { UI.toast('请输入氛围', 'warn'); return; } ov.remove(); SelectionActions.fillTemplate('atmosphere', { atmosphere: v }); } }
      ]
    });
  }
};
