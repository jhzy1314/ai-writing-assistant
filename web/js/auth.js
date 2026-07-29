/* ============ auth.js：密码认证 ============ */
var Auth = {
  check: async function () {
    try {
      var r = await fetch('/api/auth/check');
      return await r.json();
    } catch (e) { return { required: false, authenticated: true }; }
  },
  showLogin: function () {
    var root = document.getElementById('modalRoot');
    root.innerHTML = '<div class="auth-overlay" id="authOverlay"><div class="auth-card">' +
      '<div class="auth-logo">✦</div>' +
      '<h2 style="margin:0 0 4px;font-size:20px">AI Novel Studio</h2>' +
      '<p style="color:var(--muted);font-size:12px;margin:0 0 18px">请输入访问密码</p>' +
      '<input type="password" id="authPassword" class="auth-input" placeholder="密码" onkeydown="if(event.key===\'Enter\')Auth.login()" autofocus>' +
      '<div class="auth-error" id="authError" style="display:none"></div>' +
      '<button class="btn btn-primary btn-block" id="authBtn" onclick="Auth.login()" style="margin-top:12px">登录</button>' +
      '</div></div>';
    setTimeout(function () {
      var inp = document.getElementById('authPassword');
      if (inp) inp.focus();
    }, 200);
  },
  login: async function () {
    var pw = document.getElementById('authPassword').value;
    if (!pw) { this.showError('请输入密码'); return; }
    var btn = document.getElementById('authBtn');
    btn.disabled = true; btn.textContent = '验证中…';
    try {
      var r = await fetch('/api/auth/login', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: pw })
      });
      if (!r.ok) {
        var d = await r.json().catch(function () { return { error: '密码错误' }; });
        this.showError(d.error || '密码错误');
        btn.disabled = false; btn.textContent = '登录';
        return;
      }
      document.getElementById('authOverlay').remove();
      initApp();
    } catch (e) {
      this.showError('网络错误，请重试');
      btn.disabled = false; btn.textContent = '登录';
    }
  },
  showError: function (msg) {
    var el = document.getElementById('authError');
    el.textContent = msg; el.style.display = '';
    setTimeout(function () { el.style.display = 'none'; }, 3000);
  }
};
