package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/config"
	"github.com/ai-novel/studio/internal/infrastructure/quality"

	_ "modernc.org/sqlite"
)

// ============================================================
// cmd/eval —— 生成质量评测工具（可重复运行）
// 评估侧三件套：
//  1. 确定性断言（零成本）：AI 标记词/元叙事/说教词/段落节奏/字数
//  2. LLM-as-judge 多维评分：连贯性/人设一致/伏笔回收/文笔/节奏（中文 rubric, temperature=0）
//  3. golden set：从本地真实章节取样；可加 -fail 注入故意改坏的样本验证灵敏度
// 用法: go run ./cmd/eval -db data/ai-novel.db -config configs -out eval_report.md [-chapters 3] [-fail] [-no-judge]
// ============================================================

// ---------- 1. 确定性断言（规则表复用 internal/infrastructure/quality 单一来源，零 LLM 成本） ----------

type determinResult struct {
	WordCount     int      `json:"word_count"`
	SurpriseHits  []string `json:"surprise_hits"`
	MetaHits      []string `json:"meta_hits"`
	SermonHits    []string `json:"sermon_hits"`
	FillerDensity float64  `json:"filler_density_per_1k"`
	ShortParRatio float64  `json:"short_paragraph_ratio"`
	AvgParLen     float64  `json:"avg_paragraph_len"`
	ShockHits     []string `json:"shock_hits"`
}

func countHits(s string, subs []string) []string {
	var out []string
	for _, sub := range subs {
		if n := strings.Count(s, sub); n > 0 {
			out = append(out, fmt.Sprintf("%s×%d", sub, n))
		}
	}
	return out
}

func regexHits(s string, rules []quality.PatternRule) []string {
	var out []string
	for _, r := range rules {
		if r.Re.MatchString(s) {
			out = append(out, r.Label)
		}
	}
	return out
}

func determinCheck(content string) determinResult {
	d := determinResult{WordCount: quality.WordCount(content)}
	d.SurpriseHits = countHits(content, quality.SurpriseMarkers)
	d.MetaHits = regexHits(content, quality.MetaPatterns)
	d.SermonHits = countHits(content, quality.SermonWords)
	d.ShockHits = regexHits(content, quality.ShockPatterns)
	// 填充词统一走 quality.FillerCount（排除「不可能/可能性」等子串，口径与运行时检测一致）
	d.FillerDensity = float64(quality.FillerCount(content)) / float64(maxInt(d.WordCount, 1)) * 1000
	paras := strings.FieldsFunc(content, func(r rune) bool { return r == '\n' || r == '\r' })
	if len(paras) > 0 {
		var short, total int
		for _, p := range paras {
			l := quality.WordCount(p)
			total += l
			if l < 30 {
				short++
			}
		}
		d.ShortParRatio = float64(short) / float64(len(paras))
		d.AvgParLen = float64(total) / float64(len(paras))
	}
	return d
}

// ---------- 2. LLM-as-judge 多维评分（中文 rubric，temperature=0） ----------

const judgeSystem = `你是中文网文质量评审专家。请按以下 5 个维度对给定章节正文独立评分（每维 1-5 分）：
1. 连贯性（coherence）：因果链、时间线、空间逻辑是否断裂；
2. 人设一致（character）：角色行为/语言是否符合人物设定，有无 OOC；
3. 伏笔回收（foreshadowing）：本章是否埋设或回收伏笔，呼应前文；
4. 文笔风格（style）：语言表现力、句式节奏、AI 腔程度（套话/模板感越强分越低）；
5. 节奏（pacing）：张弛有度、段落密度合理、无拖沓或仓促。
规则：只输出 JSON 对象，格式：
{"scores":{"coherence":1-5,"character":1-5,"foreshadowing":1-5,"style":1-5,"pacing":1-5},"reasons":{"coherence":"…","character":"…","foreshadowing":"…","style":"…","pacing":"…"},"overall":0-100,"summary":"总体评价（60字内）"}
不要输出任何其他文字。`

type judgeScores struct {
	Scores  map[string]int    `json:"scores"`
	Reasons map[string]string `json:"reasons"`
	Overall int               `json:"overall"`
	Summary string            `json:"summary"`
}

type judgeResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func judgeChapter(ctx context.Context, endpoint, apiKey, model, title, content string) (judgeScores, error) {
	var js judgeScores
	user := fmt.Sprintf("章节标题：%s\n\n正文：\n%s", title, truncateRunes(content, 6000))
	body, _ := json.Marshal(map[string]interface{}{
		"model":       model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": judgeSystem},
			{"role": "user", "content": user},
		},
	})
	url := strings.TrimSuffix(endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return js, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return js, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return js, fmt.Errorf("judge API %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var jr judgeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&jr); err != nil { // 响应体限 1MB
		return js, err
	}
	if len(jr.Choices) == 0 {
		return js, fmt.Errorf("judge 无输出")
	}
	text := jr.Choices[0].Message.Content
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return js, fmt.Errorf("judge 输出非 JSON: %s", truncateRunes(text, 120))
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &js); err != nil {
		return js, fmt.Errorf("judge JSON 解析失败: %v", err)
	}
	if len(js.Scores) != 5 {
		return js, fmt.Errorf("judge 评分维度不完整: %v", js.Scores)
	}
	return js, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// mdSafe 清理标题/摘要中的 markdown 表格特殊字符（| 与换行会破坏表格结构）
func mdSafe(s string) string {
	s = strings.ReplaceAll(s, "|", "｜")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---------- 3. 样本加载（只读打开，不跑迁移；-golden 用内置固定样本） ----------

// goldenSamples 内置固定样本（不依赖本地库），用于改 prompt/换模型前后的回归对比
func goldenSamples() []chapterSample {
	return []chapterSample{
		{Title: "黄金样本·自然文风（期望高分）", Content: `林云推开教室后门，风裹着晚自习的喧闹涌出来。他靠在墙边等了三分钟，才看见惊鸿抱着作业本走下楼梯。
"今天怎么这么慢？"他接过那摞本子，纸页边角被捏得发皱。
惊鸿低头踢了踢台阶："数学老师拖堂。"
走廊尽头有人喊他们去食堂，两人一前一后走进暮色里。路灯一盏盏亮起来，把影子拉得很长。`},
		{Title: "普通样本·常见网文段落", Content: `周然推门进来的时候，屋里已经坐满了人。他扫了一圈，在角落找到空位坐下，把包放在桌上。
"你怎么才来？"旁边的景明凑过来，压低声音，"刚才张姐问了你两次。"
周然摇摇头，没接话。他掏出手机看了眼时间，又放回口袋。窗外的雨还在下，玻璃上蒙着一层水汽。`},
		{Title: "失败样本·AI味（期望低分）", Content: `林风忽然变成了冷酷的杀手，仿佛一切都变了，竟然没有任何征兆，猛地拔出刀，不禁让人倒吸凉气。
全场震惊，众人纷纷目瞪口呆，显然这是重大转折。毋庸置疑，这个故事接下来将会发生重大转折。
不是巧合而是命运，不是偶然而是注定。——仿佛一切都是一场梦。——一切仿佛回到原点。
然而事情并没有那么简单，然而他并不知道，然而这一切才刚刚开始。`},
	}
}

type chapterSample struct {
	Title   string
	Content string
}

func loadSamples(dbPath string, maxChapters int, fail bool) ([]chapterSample, error) {
	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("打开数据库失败（只读模式）: %w", err)
	}
	rows, err := db.Query(`SELECT p.id, COUNT(c.id) FROM projects p
		LEFT JOIN chapters c ON c.project_id=p.id AND c.is_deleted=0 AND c.content!=''
		GROUP BY p.id ORDER BY COUNT(c.id) DESC`)
	if err != nil {
		return nil, err
	}
	type proj struct {
		id  string
		chs int
	}
	var projs []proj
	for rows.Next() {
		var p proj
		if err := rows.Scan(&p.id, &p.chs); err != nil {
			rows.Close()
			return nil, err
		}
		projs = append(projs, p)
	}
	rows.Close()
	if len(projs) == 0 {
		return nil, fmt.Errorf("数据库中没有项目")
	}
	best := projs[0]
	chRows, err := db.Query(`SELECT title, content FROM chapters
		WHERE project_id=? AND is_deleted=0 AND content!='' ORDER BY sort_order LIMIT ?`, best.id, maxChapters)
	if err != nil {
		return nil, err
	}
	defer chRows.Close()
	var samples []chapterSample
	for chRows.Next() {
		var s chapterSample
		if err := chRows.Scan(&s.Title, &s.Content); err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}
	if err := chRows.Err(); err != nil {
		return nil, err
	}
	if fail && len(samples) > 0 {
		bad := samples[0]
		bad.Title = bad.Title + "【失败样本·故意改坏】"
		r := []rune(bad.Content)
		if len(r) > 30 {
			r = r[30:]
		}
		bad.Content = "林风突然变成了一个冷酷无情的杀手，性格与之前判若两人，时间也倒退回了三年前，仿佛一切都没有发生过。仿佛、忽然、竟然、猛地，众人纷纷倒吸凉气，全场震惊。显然，毋庸置疑，这个故事接下来将会发生重大转折。" + string(r)
		samples = append(samples, bad)
	}
	return samples, nil
}

// ---------- 主流程 ----------

func main() {
	dbPath := flag.String("db", "data/ai-novel.db", "SQLite 数据库路径")
	cfgDir := flag.String("config", "configs", "配置文件目录")
	out := flag.String("out", "eval_report.md", "报告输出路径")
	chapters := flag.Int("chapters", 3, "评测章节数（从数据库取样）")
	fail := flag.Bool("fail", false, "注入故意改坏的失败样本")
	golden := flag.Bool("golden", false, "使用内置固定样本（回归对比用，不读数据库）")
	skipJudge := flag.Bool("no-judge", false, "跳过 LLM 评分（仅确定性断言）")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cfg, err := config.LoadConfig(*cfgDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
	var apiKey, model, endpoint string
	for _, m := range cfg.Models {
		if strings.Contains(strings.ToLower(m.Name), "deepseek") && m.APIKey != "" {
			apiKey, model, endpoint = m.APIKey, m.Name, m.APIEndpoint
			break
		}
	}
	if apiKey == "" {
		for _, m := range cfg.Models {
			if m.APIKey != "" {
				apiKey, model, endpoint = m.APIKey, m.Name, m.APIEndpoint
				break
			}
		}
	}

	var samples []chapterSample
	if *golden {
		samples = goldenSamples()
	} else {
		loaded, err := loadSamples(*dbPath, *chapters, *fail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载样本失败: %v\n", err)
			os.Exit(1)
		}
		samples = loaded
	}
	if len(samples) == 0 {
		fmt.Fprintln(os.Stderr, "没有可评测的章节")
		os.Exit(1)
	}

	var b strings.Builder
	b.WriteString("# AI 写作助手 生成质量评测报告\n\n")
	mode := "数据库取样"
	dbLabel := *dbPath
	if *golden {
		mode = "golden 内置固定样本（回归对比）"
		dbLabel = "（不使用数据库）"
	}
	b.WriteString(fmt.Sprintf("- 生成时间：%s\n- 数据库：%s\n- 样本：%d 章（%s）\n- Judge 模型：%s\n", time.Now().Format("2006-01-02 15:04:05"), dbLabel, len(samples), mode, model))
	if *fail && !*golden {
		b.WriteString("- ⚠️ 含 1 个故意改坏的失败样本（用于验证 judge/断言灵敏度）\n")
	}
	b.WriteString("\n---\n\n")

	// 一、确定性断言
	b.WriteString("## 一、确定性断言（零成本规则检查）\n\n")
	b.WriteString("| 章节 | 字数 | 惊讶词 | 元叙事 | 说教词 | AI填充词/千字 | 短段比 | 均段长 | 集体震惊 |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	flagText := func(hits []string) string {
		if len(hits) == 0 {
			return "✓"
		}
		return "⚠ " + strings.Join(hits, ", ")
	}
	for _, s := range samples {
		d := determinCheck(s.Content)
		b.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s | %.1f | %.0f%% | %.0f | %s |\n",
			mdSafe(s.Title), d.WordCount, flagText(d.SurpriseHits), flagText(d.MetaHits), flagText(d.SermonHits),
			d.FillerDensity, d.ShortParRatio*100, d.AvgParLen, flagText(d.ShockHits)))
	}
	b.WriteString("\n> 判定：惊讶词（仿佛/忽然/竟然…）≥3 处、元叙事（「接下来/故事发展到了」…）、说教词、填充词密度>3/千字、集体震惊模式 —— 均为 AI 味信号（借鉴 inkos post-write-validator 思路）。\n\n---\n\n")

	// 二、LLM-as-judge
	b.WriteString("## 二、LLM-as-judge 多维评分（1-5 分/维，总分 0-100）\n\n")
	dims := []string{"coherence", "character", "foreshadowing", "style", "pacing"}
	dimNames := map[string]string{"coherence": "连贯性", "character": "人设一致", "foreshadowing": "伏笔回收", "style": "文笔风格", "pacing": "节奏"}

	if *skipJudge || apiKey == "" {
		b.WriteString("> 已跳过 LLM 评分（-no-judge 或未找到 API key）。\n")
	} else {
		// 一次调用缓存结果（表格 + 理由共用）；按样本索引存储，避免重名章节标题互相覆盖
		type judgeOutcome struct {
			js  judgeScores
			err error
		}
		judged := make([]judgeOutcome, len(samples))
		for i, s := range samples {
			js, jerr := judgeChapter(ctx, endpoint, apiKey, model, s.Title, s.Content)
			judged[i] = judgeOutcome{js, jerr}
		}

		b.WriteString("| 章节 | 连贯性 | 人设一致 | 伏笔回收 | 文笔风格 | 节奏 | 总分 | 总评 |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		for i, s := range samples {
			jo := judged[i]
			if jo.err != nil {
				b.WriteString(fmt.Sprintf("| %s | — | — | — | — | — | **失败**: %s |\n", mdSafe(s.Title), mdSafe(jo.err.Error())))
				continue
			}
			cells := make([]string, 0, len(dims))
			for _, d := range dims {
				sc := jo.js.Scores[d]
				mark := "🔴"
				if sc >= 4 {
					mark = "🟢"
				} else if sc >= 3 {
					mark = "🟡"
				}
				cells = append(cells, fmt.Sprintf("%s%d", mark, sc))
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n", mdSafe(s.Title), strings.Join(cells, " | "), jo.js.Overall, mdSafe(jo.js.Summary)))
		}

		b.WriteString("\n### 分维度理由\n\n")
		for i, s := range samples {
			jo := judged[i]
			if jo.err != nil {
				continue
			}
			b.WriteString(fmt.Sprintf("**%s**（%d/100）\n", mdSafe(s.Title), jo.js.Overall))
			for _, d := range dims {
				b.WriteString(fmt.Sprintf("- **%s**（%d/5）：%s\n", dimNames[d], jo.js.Scores[d], mdSafe(jo.js.Reasons[d])))
			}
			b.WriteString("\n")
		}
	}

	// 三、结论
	b.WriteString("---\n\n## 三、结论\n\n")
	b.WriteString("> 参考阈值：总分 ≥80 通过、70-79 需修改、<70 不达标（与本项目 Verifier 的 85 分门槛近似，评测略宽松）。\n")
	dir := filepath.Dir(*out)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "创建输出目录失败: %v\n", err)
			os.Exit(1)
		}
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写报告失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 评测完成，报告: %s（%d 章）\n", *out, len(samples))
}
