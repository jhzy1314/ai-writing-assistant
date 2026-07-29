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
  text = text || '';
  var cjk = (text.match(/[\u4e00-\u9fff\u3040-\u30ff]/g) || []).length;
  var en = (text.match(/[a-zA-Z]+/g) || []).length;
  return cjk + en;
}
function charCount(text) { return Array.from(text || '').length; }
function uid() { return 't' + Date.now().toString(36) + Math.random().toString(36).slice(2, 7); }
