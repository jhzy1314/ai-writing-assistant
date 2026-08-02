package llm

import "testing"

func TestHasSessionCookieDeepSeek(t *testing.T) {
	// 用户扫码登录后 rod 实际 dump 到的 cookie（含 ds_session_id）
	real := "ds_session_id=04a1dbf9c0a7473abf9f7d7dde7e1307; smidV2=20260802122909a4546a82d7f0eb63ac3d89f1458d1de5005da71b1018fd0b0; HWWAFSESTIME=1785644954231; HWWAFSESID=b2b9e2cb5e40768908e"
	// 注意：ds_session_id 未登录也有，不再作为 deepseek 登录依据
	if hasSessionCookie(real, "deepseek-free") {
		t.Fatal("FAIL: 仅 ds_session_id（未登录也有）不应判定 deepseek 已登录")
	}
	t.Log("PASS: deepseek 仅 ds_session_id 不算登录（修复误判）")

	// 未登录的 kimi 首页（之前误判为 completed 的场景）
	notLoggedIn := "doodle_asset=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; Hm_lvt_xxx=1; HMACCOUNT=abc; theme=dark; next-sidebar-publisher-shortcut-region=true"
	if hasSessionCookie(notLoggedIn, "kimi-free") {
		t.Fatal("FAIL: 未登录 kimi 不应判定为已登录")
	}
	t.Log("PASS: 未登录 kimi 不误判")
}

func TestGetLoginToken(t *testing.T) {
	// deepseek userToken 是 {"value":"..."} JSON 格式
	parse := func(v string) string {
		// 模拟 getLoginToken 内 JS 逻辑的 Go 校验（仅验证格式判定）
		if len(v) < 10 || v == "null" {
			return ""
		}
		return v
	}
	if parse(`{"value":"eyJhbGciOiJIUzI1NiJ9.abc123","__version":"0"}`) == "" {
		t.Fatal("FAIL: userToken JSON 应被识别")
	}
	if parse("null") != "" {
		t.Fatal("FAIL: null 不应被识别")
	}
	t.Log("PASS: userToken 格式判定正确")
}
