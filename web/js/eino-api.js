/* ============ Eino 增强 API（通过根服务代理，同端口） ============ */
var EinoAPI = {
  base: '',

  get: async function (path) {
    try {
      var r = await fetch(this.base + path, { credentials: 'include' });
      return r.ok ? r.json() : Promise.reject(await r.text());
    } catch (e) { return null; }
  },

  post: async function (path, body) {
    try {
      var r = await fetch(this.base + path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(body)
      });
      return r.ok ? r.json() : Promise.reject(await r.text());
    } catch (e) { return null; }
  },

  getRuntimeState: function (novelId) {
    return this.get('/api/v1/novels/' + novelId + '/state?format=markdown');
  },

  getStateJSON: function (novelId) {
    return this.get('/api/v1/novels/' + novelId + '/state');
  },

  detectAIGC: async function (content) {
    return this.post('/api/v1/detect-aigc', { content: content });
  },

  generateOutline: async function (idea, chapters) {
    return this.get('/api/v1/novels/outline?idea=' + encodeURIComponent(idea) + '&chapters=' + (chapters || 30));
  },

  getForecast: function (novelId, chapter, branches) {
    var q = '?chapter=' + (chapter || 1) + '&branches=' + (branches || 3);
    return this.get('/api/v1/novels/' + novelId + '/forecast' + q);
  }
};
