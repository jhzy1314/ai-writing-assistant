package quality

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ============================================================
// quality/aiflow.go —— 零成本确定性 AI 味检测
// 把 WorkerPrompt 的「去AI味写作规约」与 Verifier 的 AI 味清单
// 代码化为正则/统计规则，生成完成后立即检查（不调模型、免费、稳定）。
// 阈值与规约对齐：惊讶词/填充词密度/转折词重复/破折号/段落节奏。
// ============================================================

// Issue 一条 AI 味信号
type Issue struct {
	Type    string // 类别（用于提示文案）
	Count   int    // 命中次数
	Details string // 命中明细，如 "仿佛×2, 忽然×1"
}

// Analysis 一次文本的 AI 味分析结果
type Analysis struct {
	Issues []Issue
	Pass   bool // 无任何命中
}

// Analyze 对正文执行确定性 AI 味检查（线程安全，纯函数）
func Analyze(content string) Analysis {
	var issues []Issue
	add := func(typ string, count int, details string) {
		if count > 0 {
			issues = append(issues, Issue{Type: typ, Count: count, Details: details})
		}
	}

	// 0. 字数（非空白字符数，含标点；口径与项目 wordCount 一致）
	wc := WordCount(content)

	// 1. 惊讶词（Worker 规约第 5 条：情绪中介词每千字 ≤2 次；总 ≥3 或单词 ≥2 即提示）
	hits := countHits(content, SurpriseMarkers)
	if total := sumCounts(hits); total >= 3 {
		add("惊讶词堆砌", total, joinHits(hits))
	} else {
		for w, n := range hits {
			if n >= 2 {
				add("惊讶词堆砌", n, w+"×"+itoa(n))
				break
			}
		}
	}

	// 2. 元叙事/编剧旁白（"接下来就是""故事发展到了"…）
	if m := regexHits(content, MetaPatterns); len(m) > 0 {
		add("元叙事旁白", len(m), strings.Join(m, ", "))
	}

	// 3. 说教词
	if s := countHits(content, SermonWords); len(s) > 0 {
		add("说教词", len(s), joinHits(s))
	}

	// 4. AI 填充词密度（Verifier 清单：>3 次/千字）
	if wc >= 200 { // 短文忽略密度指标
		filler := FillerCount(content)
		if density := float64(filler) / float64(wc) * 1000; density > 3 {
			add("AI填充词过密", filler, fmt.Sprintf("%.1f 次/千字", density))
		}
	}

	// 5. 转折词重复（Verifier 清单：单个 ≥3 次才报，避免正常行文误报）
	transHits := countHits(content, transitionWords)
	var transDetails []string
	transTotal := 0
	for w, n := range transHits {
		if n >= 3 {
			transDetails = append(transDetails, w+"×"+itoa(n))
			transTotal += n
		}
	}
	if len(transDetails) > 0 {
		add("转折词重复", transTotal, strings.Join(transDetails, ", "))
	}

	// 6. 集体震惊模式
	if sh := regexHits(content, ShockPatterns); len(sh) > 0 {
		add("集体震惊套板", len(sh), strings.Join(sh, ", "))
	}

	// 7. 破折号堆砌（Worker 规约第 5 条：—— 每千字 ≤1 次；按密度判断，短文 ≥2 次才提示，避免单个破折号误报）
	if dash := strings.Count(content, "——"); dash >= 2 && dash > maxInt(1, wc/1000) {
		add("破折号堆砌", dash, fmt.Sprintf("——×%d（%.1f 次/千字）", dash, float64(dash)/float64(maxInt(wc, 1))*1000))
	}

	// 8. "不是……而是……"句式（Worker 规约第 5 条明令禁止）
	if nb := len(notButRe.FindAllString(content, -1)); nb > 0 {
		add("「不是…而是…」句式", nb, "×"+itoa(nb))
	}

	// 9. 段落节奏（Worker 规约第 7 条：叙事段 40-120 字；短段占比过高或连续短段）
	if p := paragraphRhythm(content); p != nil {
		add(p.Type, p.Count, p.Details)
	}

	// 10. 句长过于均匀（AI 特征：句式整齐、无长短变化）
	if u := sentenceUniformity(content); u != nil {
		add(u.Type, u.Count, u.Details)
	}

	return Analysis{Issues: issues, Pass: len(issues) == 0}
}

// ---------- 规则定义（阈值与 Worker/Verifier 规约对齐；规则表导出供评测工具复用，保证单一来源） ----------

// PatternRule 正则规则 + 可读示例（用于提示文案）
type PatternRule struct {
	Re    *regexp.Regexp
	Label string
}

// SurpriseMarkers 惊讶/情绪中介词
var SurpriseMarkers = []string{"仿佛", "忽然", "竟然", "猛地", "猛然", "不禁", "宛如"}

// MetaPatterns 元叙事/编剧旁白模式
var MetaPatterns = []PatternRule{
	{regexp.MustCompile(`到这里[，,]?算是`), "到这里算是…"},
	{regexp.MustCompile(`接下来[，,]?(?:就是|将会|即将)`), "接下来就是/即将…"},
	{regexp.MustCompile(`(?:后面|之后)[，,]?(?:会|将|还会)`), "后面会/还会…"},
	{regexp.MustCompile(`(?:故事|剧情)(?:发展)?到了`), "故事/剧情到了…"},
	{regexp.MustCompile(`读者[，,]?(?:可能|应该|也许)`), "读者可能/也许…"},
	{regexp.MustCompile(`我们[，,]?(?:可以|不妨|来看)`), "我们可以/不妨…"},
}

// SermonWords 说教词
var SermonWords = []string{"显然", "毋庸置疑", "不言而喻", "众所周知", "不难看出"}

// ShockPatterns 集体震惊套板
var ShockPatterns = []PatternRule{
	{regexp.MustCompile(`(?:全场|众人|所有人|在场的人)[，,]?(?:都|全|齐齐|纷纷)?(?:震惊|惊呆|倒吸凉气|目瞪口呆|哗然|惊呼)`), "全场震惊/众人惊呆"},
	{regexp.MustCompile(`(?:全场|一片)[，,]?(?:寂静|哗然|沸腾|震动)`), "全场寂静/哗然"},
}

var aiFillerWords = []string{"似乎", "可能", "或许", "大概", "某种程度上"}

var transitionWords = []string{"然而", "不过", "与此同时", "另一方面"}

var notButRe = regexp.MustCompile(`不是[^。！？\n]{0,20}而是`)

// ---------- 统计工具 ----------

// WordCount 非空白字符数（含标点），与数据库 wordCount 完全同口径（unicode.IsSpace）
func WordCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		n++
	}
	return n
}

// FillerCount 统计全部 AI 填充词出现次数（已排除「不可能/可能性/尽可能/只可能/也可能/还可能」等子串）
func FillerCount(s string) int {
	total := 0
	for _, w := range aiFillerWords {
		total += countFiller(s, w)
	}
	return total
}

func countHits(s string, subs []string) map[string]int {
	out := make(map[string]int, len(subs))
	for _, sub := range subs {
		if n := strings.Count(s, sub); n > 0 {
			out[sub] = n
		}
	}
	return out
}

func sumCounts(m map[string]int) int {
	t := 0
	for _, n := range m {
		t += n
	}
	return t
}

func joinHits(m map[string]int) string {
	var parts []string
	for w, n := range m {
		parts = append(parts, w+"×"+itoa(n))
	}
	return strings.Join(parts, ", ")
}

func regexHits(s string, rules []PatternRule) []string {
	var out []string
	for _, r := range rules {
		if r.Re.MatchString(s) {
			out = append(out, r.Label)
		}
	}
	return out
}

// countFiller 统计填充词出现次数，排除常见子串误计。
// 「可能」前接 不/尽/只/也/还（不可能/尽可能/只可能/也可能/还可能）或后接 性（可能性）时视为子串，不计数；
// 按相邻 rune 判断而非删除拼接，杜绝「可不可能能」类文本的假阳性。
func countFiller(s, word string) int {
	if word != "可能" {
		return strings.Count(s, word)
	}
	rs := []rune(s)
	count := 0
	for i := 0; i+1 < len(rs); i++ {
		if rs[i] == '可' && rs[i+1] == '能' {
			if i > 0 {
				switch rs[i-1] {
				case '不', '尽', '只', '也', '还':
					continue
				}
			}
			if i+2 < len(rs) && rs[i+2] == '性' { // 可能性
				continue
			}
			count++
		}
	}
	return count
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---------- 段落节奏 ----------

func paragraphRhythm(content string) *Issue {
	paras := strings.FieldsFunc(content, func(r rune) bool { return r == '\n' || r == '\r' })
	if len(paras) < 5 {
		return nil
	}
	var short, shortStreak, maxStreak int
	for _, p := range paras {
		if WordCount(p) < 30 {
			short++
			shortStreak++
			if shortStreak > maxStreak {
				maxStreak = shortStreak
			}
		} else {
			shortStreak = 0
		}
	}
	ratio := float64(short) / float64(len(paras))
	switch {
	case ratio > 0.7 && short >= 5:
		return &Issue{Type: "短段过密", Count: short, Details: fmt.Sprintf("短段占比 %.0f%%", ratio*100)}
	case maxStreak >= 4:
		return &Issue{Type: "连续短段", Count: maxStreak, Details: fmt.Sprintf("连续 %d 段短段", maxStreak)}
	}
	return nil
}

// ---------- 句长均匀度 ----------

// sentenceUniformity 句长过于均匀（stddev/mean 过低）是 AI 句式特征
func sentenceUniformity(content string) *Issue {
	parts := regexp.MustCompile(`[^。！？!?\n]+[。！？!?]?`).FindAllString(content, -1)
	var lens []float64
	for _, p := range parts {
		l := float64(len([]rune(strings.TrimSpace(p))))
		if l >= 8 { // 忽略极短句（对话语气词等）
			lens = append(lens, l)
		}
	}
	if len(lens) < 10 {
		return nil
	}
	var sum float64
	for _, l := range lens {
		sum += l
	}
	mean := sum / float64(len(lens))
	var v float64
	for _, l := range lens {
		v += (l - mean) * (l - mean)
	}
	std := math.Sqrt(v / float64(len(lens)))
	if mean > 0 && std/mean < 0.3 {
		return &Issue{Type: "句长过于均匀", Count: len(lens), Details: fmt.Sprintf("句长方差/均值 %.2f（AI 句式特征）", std/mean)}
	}
	return nil
}

func itoa(n int) string { return strconv.Itoa(n) }
