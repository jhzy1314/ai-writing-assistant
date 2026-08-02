package llm

import "testing"

func TestHasSessionCookieDeepSeek(t *testing.T) {
	// 用户扫码登录后 rod 实际 dump 到的 cookie（含 ds_session_id）
	real := "ds_session_id=04a1dbf9c0a7473abf9f7d7dde7e1307; smidV2=20260802122909a4546a82d7f0eb63ac3d89f1458d1de5005da71b1018fd0b0; HWWAFSESTIME=1785644954231; HWWAFSESID=b2b9e2cb5e40768908e"
	if !hasSessionCookie(real, "deepseek-free") {
		t.Fatal("FAIL: 应识别 ds_session_id 为已登录")
	}
	t.Log("PASS: deepseek ds_session_id 识别成功")

	// 未登录的 kimi 首页（之前误判为 completed 的场景）
	notLoggedIn := "doodle_asset=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; Hm_lvt_xxx=1; HMACCOUNT=abc; theme=dark; next-sidebar-publisher-shortcut-region=true"
	if hasSessionCookie(notLoggedIn, "kimi-free") {
		t.Fatal("FAIL: 未登录 kimi 不应判定为已登录")
	}
	t.Log("PASS: 未登录 kimi 不误判")
}
