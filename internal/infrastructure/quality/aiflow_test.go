package quality

import (
	"strings"
	"testing"
)

// TestAnalyzePass 正常文本不应误报
func TestAnalyzePass(t *testing.T) {
	clean := `林云推开教室后门，风裹着晚自习的喧闹涌出来。他靠在墙边等了三分钟，才看见惊鸿抱着作业本走下楼梯。
"今天怎么这么慢？"他接过那摞本子，纸页边角被捏得发皱。
惊鸿低头踢了踢台阶："数学老师拖堂。"
走廊尽头有人喊他们去食堂，两人一前一后走进暮色里。`
	a := Analyze(clean)
	if !a.Pass {
		t.Fatalf("正常文本不应命中 AI 味信号: %+v", a.Issues)
	}
}

// TestAnalyzeAIish AI 味文本应命中多项
func TestAnalyzeAIish(t *testing.T) {
	bad := `林风忽然变成了冷酷的杀手，仿佛一切都变了，竟然没有任何征兆，猛地拔出刀，不禁让人倒吸凉气。
全场震惊，众人纷纷目瞪口呆，显然这是重大转折。毋庸置疑，这个故事接下来将会发生重大转折，显然这是重大转折。
不是巧合而是命运，不是偶然而是注定。——仿佛一切都是一场梦。——一切仿佛回到原点。——仿佛从未发生过。
然而事情并没有那么简单，然而他并不知道，然而这一切才刚刚开始，然而真相还远未揭晓。`
	a := Analyze(bad)
	if a.Pass {
		t.Fatal("AI 味文本应命中信号")
	}
	found := map[string]bool{}
	for _, is := range a.Issues {
		found[is.Type] = true
	}
	for _, want := range []string{"惊讶词堆砌", "元叙事旁白", "说教词", "集体震惊套板", "破折号堆砌", "「不是…而是…」句式", "转折词重复"} {
		if !found[want] {
			t.Errorf("应命中「%s」, 实际: %v", want, found)
		}
	}
}

// TestAnalyzeSentenceUniformity 句长过于均匀应命中
func TestAnalyzeSentenceUniformity(t *testing.T) {
	var b string
	// 构造 14 句长度相近的句子（每句 16 字左右）
	seed := []string{
		"他沿着长街慢慢走着，", "路灯把影子拉得很长，", "风从巷口吹过来，",
		"口袋里装着半封信，", "信纸被揉得很皱，", "字迹在昏光下模糊，",
		"他想起那年秋天，", "银杏落满了校门口，", "她站在树下等他，",
		"手里攥着两张票，", "后来他们没去看成，", "票根一直留着，",
		"如今他站在这里，", "街还是那条街，",
	}
	for _, s := range seed {
		b += s + "他沿着长街慢慢走着。" // 统一收尾保持句长相近
	}
	a := Analyze(b)
	found := false
	for _, is := range a.Issues {
		if is.Type == "句长过于均匀" {
			found = true
		}
	}
	if !found {
		t.Fatalf("句长均匀文本应命中: %+v", a.Issues)
	}
}

// TestAnalyzeShortParagraphs 连续短段应命中
func TestAnalyzeShortParagraphs(t *testing.T) {
	content := "他推开门。\n屋里没有人。\n桌上的茶还温着。\n窗开着。\n风把帘子吹起来。\n他愣在原地。"
	a := Analyze(content)
	found := false
	for _, is := range a.Issues {
		if is.Type == "连续短段" || is.Type == "短段过密" {
			found = true
		}
	}
	if !found {
		t.Fatalf("连续短段应命中: %+v", a.Issues)
	}
}

// TestAnalyzeTransitionBoundary 转折词 1-2 次不应误报（阈值 ≥3 的边界回归）
func TestAnalyzeTransitionBoundary(t *testing.T) {
	text := `林云推门进来，然而教室里空无一人。他放下书包，不过也没多想，继续往座位走去。`
	a := Analyze(text)
	for _, is := range a.Issues {
		if is.Type == "转折词重复" {
			t.Fatalf("转折词仅 1-2 次不应命中: %+v", a.Issues)
		}
	}
}

// TestAnalyzeDashBoundary 单个破折号（短文）不应误报「堆砌」
func TestAnalyzeDashBoundary(t *testing.T) {
	text := `林云顿了顿——他没想到会在这里遇见她。`
	a := Analyze(text)
	for _, is := range a.Issues {
		if is.Type == "破折号堆砌" {
			t.Fatalf("单个破折号不应命中: %+v", a.Issues)
		}
	}
}

// TestCountFiller 填充词排除子串误计：「可能」不计入「不可能/可能性/尽可能/只可能/也可能/还可能」
func TestCountFiller(t *testing.T) {
	text := `这件事不可能成功，可能性很低，我们尽可能试试，只可能失败，也可能翻盘，但结果可能不如人意。`
	// 原文含 7 个「可能」子串，但只有最后一个独立「可能」应计入
	if got := countFiller(text, "可能"); got != 1 {
		t.Fatalf("「可能」应计 1 次（排除不可能/可能性/尽可能/只可能/也可能），实际 %d", got)
	}
	// 拼接场景：删除法会假阳性，「可不可能能」中独立「可能」为 0 次
	if got := countFiller("可不可能能", "可能"); got != 0 {
		t.Fatalf("「可不可能能」不应计「可能」，实际 %d", got)
	}
	// 句首独立「可能」应计
	if got := countFiller("可能他会来", "可能"); got != 1 {
		t.Fatalf("句首「可能」应计 1 次，实际 %d", got)
	}
	t.Logf("诊断: 不可能=%d 可能性=%d 尽可能=%d 只可能=%d 也可能=%d 结果可能=%d",
		countFiller("不可能", "可能"), countFiller("可能性", "可能"), countFiller("尽可能", "可能"),
		countFiller("只可能", "可能"), countFiller("也可能", "可能"), countFiller("结果可能", "可能"))
}

// TestAnalyzeDashDensity 破折号密度口径：~2400 字时 2 个不报、3 个报
func TestAnalyzeDashDensity(t *testing.T) {
	// 构造 ~2400 字无 AI 味文本（重复普通句子，不触发其它规则）
	fill := strings.Repeat("他沿着长街慢慢走着，路灯把影子拉得很长。", 120)
	// 2 个破折号（≤1/千字）不应报
	a2 := Analyze(fill + "——他顿了顿。——她没说话。")
	for _, is := range a2.Issues {
		if is.Type == "破折号堆砌" {
			t.Fatalf("长文 2 个破折号不应命中: %+v", a2.Issues)
		}
	}
	// 3 个破折号（>1/千字）应报
	a3 := Analyze(fill + "——他顿了顿。——她没说话。——风停了。")
	found := false
	for _, is := range a3.Issues {
		if is.Type == "破折号堆砌" {
			found = true
		}
	}
	if !found {
		t.Fatalf("长文 3 个破折号应命中: %+v", a3.Issues)
	}
}
