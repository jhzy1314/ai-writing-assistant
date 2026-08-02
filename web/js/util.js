/* ============ util.js：通用工具函数 ============ */
function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
  });
}
function fmtTime(s) {
  if (!s) return '';
  var d = new Date(String(s).replace(' ', 'T'));
  if (isNaN(d)) return s;
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}
function wordCount(text) {
  // 与后端 wordCount 口径一致：统计非空白字符数（含标点，排除空格/换行）
  return Array.from(text || '').filter(function (c) { return !/\s/.test(c); }).length;
}
function escAttr(s) {
  return String(s == null ? '' : s).replace(/[&<>"'`]/g, function (c) {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;', '`': '&#96;' }[c];
  });
}
function charCount(text) { return Array.from(text || '').length; }
function uid() { return 't' + Date.now().toString(36) + Math.random().toString(36).slice(2, 7); }
