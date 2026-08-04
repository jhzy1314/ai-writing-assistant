package api

// analyze_task.go —— 拆书异步任务：全文分块通读（不截断）+ 进度上报 + 轮询
//
// 拆书不再截断首尾各 3 万字，而是把全书按块切分、逐块 AI 特征提取，
// 特征过长时分级压缩，最后统一汇总为最终拆书报告。任务在后台 goroutine
// 执行，前端通过 GET /api/library/analyze/task/{id} 轮询进度。

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/go-chi/chi/v5"
)

// analyzeBlockRunes 拆书单块字符数：固定 8 万字（实测 glm-4-flash 处理 8 万字输入 6.7 秒无压力）。
// 不按 context_limit 配置限制——模型自身能处理多少由它自己决定，超限会报错提示，绝不静默截断。
const analyzeBlockRunes = 80000

// maxFeatureRunes 各块特征总长上限；超过则先做一轮中间压缩（防止汇总输入超上下文）
const maxFeatureRunes = 22000

// blockTimeout 单块 AI 调用超时（快模型一般几十秒，放宽到 10 分钟防卡死）
const blockTimeout = 10 * time.Minute

// analyzeTask 一次拆书任务的状态（轮询接口读取）
type analyzeTask struct {
	mu        sync.Mutex
	ID        string
	Stage     string // 阶段文案
	Percent   int    // 0-100
	Done      bool
	OK        bool
	Result    string
	Model     string
	Err       string
	Chars     int // 全书字符数
	Blocks    int // 分块数
	Saved     int // 自动入库素材条数
	UpdatedAt time.Time
}

func (t *analyzeTask) set(stage string, percent int) {
	t.mu.Lock()
	t.Stage = stage
	t.Percent = percent
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

func (t *analyzeTask) setMeta(chars, blocks int) {
	t.mu.Lock()
	t.Chars = chars
	t.Blocks = blocks
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

func (t *analyzeTask) finish(ok bool, result, model, errMsg string, chars, blocks int) {
	t.mu.Lock()
	t.Done = true
	t.OK = ok
	t.Result = result
	t.Model = model
	t.Err = errMsg
	t.Chars = chars
	t.Blocks = blocks
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

func (t *analyzeTask) isDone() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Done
}

// snapshot 轮询接口返回的状态
func (t *analyzeTask) snapshot() map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := map[string]interface{}{
		"id": t.ID, "stage": t.Stage, "percent": t.Percent,
		"done": t.Done, "ok": t.OK,
		"chars": t.Chars, "blocks": t.Blocks, "saved": t.Saved,
		"updated_at": t.UpdatedAt.Unix(),
	}
	if t.Result != "" {
		m["result"] = t.Result
	}
	if t.Model != "" {
		m["model"] = t.Model
	}
	if t.Err != "" {
		m["error"] = t.Err
	}
	return m
}

// analyzeTaskManager 内存任务注册表（单机工具，重启即清空，可接受）
type analyzeTaskManager struct {
	mu    sync.Mutex
	seq   int
	tasks map[string]*analyzeTask
}

var analyzeTasks = &analyzeTaskManager{tasks: map[string]*analyzeTask{}}

// start 注册任务并后台执行 fn，返回 task_id；只保留最近 50 个任务
func (m *analyzeTaskManager) start(fn func(t *analyzeTask)) string {
	m.mu.Lock()
	m.seq++
	id := fmt.Sprintf("a%d", m.seq)
	t := &analyzeTask{ID: id, Stage: "准备中…", UpdatedAt: time.Now()}
	m.tasks[id] = t
	// 清理：删掉最旧的已完成任务（进行中的任务绝不删）
	if len(m.tasks) > 50 {
		var oldest string
		var oldestAt time.Time
		for k, v := range m.tasks {
			if !v.isDone() {
				continue
			}
			if oldest == "" || v.UpdatedAt.Before(oldestAt) {
				oldest = k
				oldestAt = v.UpdatedAt
			}
		}
		if oldest != "" {
			delete(m.tasks, oldest)
		}
	}
	m.mu.Unlock()
	go func() { fn(t) }()
	return id
}

func (m *analyzeTaskManager) get(id string) *analyzeTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks[id]
}

// HandleAnalyzeTask GET /api/library/analyze/task/{id} 轮询拆书任务进度
func (s *Server) HandleAnalyzeTask(w http.ResponseWriter, r *http.Request) {
	t := analyzeTasks.get(chi.URLParam(r, "id"))
	if t == nil {
		writeError(w, http.StatusNotFound, "任务不存在或已过期（服务可能已重启）")
		return
	}
	writeOK(w, t.snapshot())
}

// runBookAnalyze 全文拆书主流程：分块通读 → 特征合并 → 最终汇总 → 四类素材自动入库
// preferredModel 非空时优先用指定模型（如免费模型），失败自动回退 helper 角色绑定；
// bookName 为书文件名（去扩展名），作为素材入库的 source_file 与作品名
func (s *Server) runBookAnalyze(t *analyzeTask, text, preferredModel, bookName string) {
	chars := len([]rune(text))
	blocks := splitBlocks(text, analyzeBlockRunes)
	t.setMeta(chars, len(blocks))

	// 单块：全书一次读完，直接出最终报告（保持原 book_analyze 输出格式）
	if len(blocks) == 1 {
		t.set("AI 通读全书（全文分析）…", 35)
		prompt := buildToolPrompt("book_analyze", text, "", "", "")
		ctx, cancel := context.WithTimeout(context.Background(), blockTimeout)
		result, model, err := s.callHelperTool(ctx, prompt, preferredModel, "book_analyze")
		cancel()
		if err != nil {
			t.finish(false, "", "", "拆书解析失败: "+err.Error(), chars, 1)
			return
		}
		s.autoSaveAnalyzed(result, bookName, t)
		t.finish(true, result, model, "", chars, 1)
		return
	}

	// 多块：逐块特征提取（真实进度：10% → 80%）
	features := make([]string, 0, len(blocks))
	for i, blk := range blocks {
		pct := 10 + 70*(i+1)/len(blocks)
		t.set(fmt.Sprintf("AI 通读全书 第 %d/%d 块…", i+1, len(blocks)), pct)
		fp := buildAnalyzePassPrompt(blk, i+1, len(blocks))
		ctx, cancel := context.WithTimeout(context.Background(), blockTimeout)
		f, _, err := s.callHelperTool(ctx, fp, preferredModel, "book_analyze_pass")
		cancel()
		if err != nil {
			t.finish(false, "", "", fmt.Sprintf("第 %d/%d 块解析失败: %s", i+1, len(blocks), err.Error()), chars, len(blocks))
			return
		}
		features = append(features, f)
	}

	// 特征过长：分级压缩（82%）
	t.set("整理全书要点…", 82)
	features = s.mergeFeatures(features, preferredModel)

	// 最终汇总（90% → 100%）
	t.set("提炼最终拆书报告…", 90)
	sp := buildAnalyzeSummaryPrompt(strings.Join(features, "\n---\n"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*blockTimeout)
	result, model, err := s.callHelperTool(ctx, sp, preferredModel, "book_analyze_summary")
	cancel()
	if err != nil {
		t.finish(false, "", "", "汇总拆书报告失败: "+err.Error(), chars, len(blocks))
		return
	}
	s.autoSaveAnalyzed(result, bookName, t)
	t.finish(true, result, model, "", chars, len(blocks))
}

// autoSaveAnalyzed 拆书结果四类素材自动入库（幂等：按 source_file 重建，重拆不重复）
func (s *Server) autoSaveAnalyzed(result, bookName string, t *analyzeTask) {
	if strings.TrimSpace(result) == "" || strings.TrimSpace(bookName) == "" {
		return
	}
	n, err := s.saveAnalyzedMaterials(context.Background(), result, bookName)
	if err != nil {
		log.Printf("[analyze] 素材自动入库失败: %v", err)
		return
	}
	if n > 0 {
		t.set(fmt.Sprintf("✅ 已自动保存 %d 条拆书素材", n), 99)
		t.mu.Lock()
		t.Saved = n
		t.mu.Unlock()
	}
}

// saveAnalyzedMaterials 解析拆书报告四部分 → 关键片段逐条 + 人物卡/世界观/伏笔整段，写入文风样本库
// 注意：Go regexp 不支持 lookahead，全部用字符串解析
func (s *Server) saveAnalyzedMaterials(ctx context.Context, result, bookName string) (int, error) {
	sec := func(key string) string {
		marker := "【" + key + "】"
		idx := strings.Index(result, marker)
		if idx < 0 {
			return ""
		}
		rest := result[idx+len(marker):]
		if n := strings.Index(rest, "【"); n >= 0 {
			rest = rest[:n]
		}
		return strings.TrimSpace(rest)
	}
	var samples []database.StyleSample
	if frag := sec("标志性片段"); frag != "" {
		for i, b := range splitAnalyzeFragments(frag) {
			body := analyzeFragmentBody(b)
			if body == "" {
				continue
			}
			samples = append(samples, database.StyleSample{
				Title: bookName + " · " + analyzeFragmentTitle(b, i), Category: "其他",
				SourceFile: bookName, Kind: database.KindFragment, Content: body,
			})
		}
	}
	if c := sec("主要角色"); c != "" {
		samples = append(samples, database.StyleSample{Title: bookName + " · 主要角色", Category: "其他", SourceFile: bookName, Kind: database.KindCharacter, Content: c})
	}
	if w := sec("世界观"); w != "" {
		samples = append(samples, database.StyleSample{Title: bookName + " · 世界观", Category: "其他", SourceFile: bookName, Kind: database.KindWorld, Content: w})
	}
	if f := sec("伏笔设计"); f != "" {
		samples = append(samples, database.StyleSample{Title: bookName + " · 伏笔设计", Category: "其他", SourceFile: bookName, Kind: database.KindForeshadow, Content: f})
	}
	if len(samples) == 0 {
		return 0, nil
	}
	return s.store.ReplaceStyleSamples(ctx, samples)
}

// splitAnalyzeFragments 按 "换行+数字." 切分标志性片段列表（字符串扫描，避免 regexp lookahead 限制）
func splitAnalyzeFragments(frag string) []string {
	var out []string
	start := 0
	for i := 1; i < len(frag); i++ {
		if frag[i-1] == '\n' && frag[i] >= '0' && frag[i] <= '9' && i+1 < len(frag) && frag[i+1] == '.' {
			if b := strings.TrimSpace(frag[start:i]); b != "" {
				out = append(out, b)
			}
			start = i
		}
	}
	if b := strings.TrimSpace(frag[start:]); b != "" {
		out = append(out, b)
	}
	return out
}

// analyzeFragmentTitle 提取片段标题（"1. 标题" → "标题"）
func analyzeFragmentTitle(b string, i int) string {
	if m := regexp.MustCompile(`(?m)^[\d]+\.\s*(.+)$`).FindStringSubmatch(b); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return fmt.Sprintf("片段%d", i+1)
}

// analyzeFragmentBody 提取片段正文（去掉序号行，取【片段正文】后的内容、到【风格要点】前）
func analyzeFragmentBody(b string) string {
	body := regexp.MustCompile(`(?m)^[\d]+\.\s*.+\n`).ReplaceAllString(b, "")
	body = strings.TrimSpace(body)
	if idx := strings.Index(body, "片段正文"); idx >= 0 {
		seg := body[idx+len("片段正文"):]
		if c := strings.Index(seg, "："); c >= 0 {
			seg = seg[c+1:]
		} else if c := strings.Index(seg, ":"); c >= 0 {
			seg = seg[c+1:]
		}
		if k := strings.Index(seg, "风格要点"); k >= 0 {
			seg = seg[:k]
		}
		seg = strings.TrimSpace(seg)
		if seg != "" {
			body = seg
		}
	}
	return body
}

// mergeFeatures 特征总长超限时，按 8 份一组压缩成中间纪要，循环直到收敛
func (s *Server) mergeFeatures(features []string, preferredModel string) []string {
	for {
		total := 0
		for _, f := range features {
			total += len([]rune(f))
		}
		if total <= maxFeatureRunes || len(features) <= 1 {
			return features
		}
		var next []string
		for i := 0; i < len(features); i += 8 {
			end := i + 8
			if end > len(features) {
				end = len(features)
			}
			part := strings.Join(features[i:end], "\n---\n")
			ctx, cancel := context.WithTimeout(context.Background(), blockTimeout)
			m, _, err := s.callHelperTool(ctx, buildMergePrompt(part), preferredModel, "book_analyze_merge")
			cancel()
			if err != nil {
				return features // 压缩失败则保持现状返回（宁缺毋滥）
			}
			next = append(next, m)
		}
		features = next
	}
}

// splitBlocks 按 rune 切块，块尾尽量落在段落边界（最近换行，最多回退 2000 字）
func splitBlocks(text string, blockSize int) []string {
	runes := []rune(text)
	if len(runes) <= blockSize {
		return []string{text}
	}
	var blocks []string
	start := 0
	for start < len(runes) {
		end := start + blockSize
		if end < len(runes) {
			cut := end
			for j := end; j > start && j > end-2000; j-- {
				if runes[j] == '\n' {
					cut = j + 1
					break
				}
			}
			end = cut
		} else {
			end = len(runes)
		}
		blocks = append(blocks, string(runes[start:end]))
		start = end
	}
	return blocks
}

// buildAnalyzePassPrompt 分块特征提取：只报本块的新信息，输出精简
func buildAnalyzePassPrompt(block string, idx, total int) string {
	return fmt.Sprintf(`你是资深小说拆书编辑的分块分析助手。这是全书第 %d/%d 块的正文（一大段连续文本）。请只提取**本块内**的信息，输出精简特征（总长控制在 500 字内，用词节约）：

【片段候选】最多 2 段（本块若无高质量片段写"无"）
<标题>|来源：<章节/大致位置>|片段正文：<原文摘录 150-300 字>|风格要点：<句式/节奏/视角/用词，1 句>

【角色线索】本块新出现或关键塑造的角色（前面块已报过的不要重复）
姓名-定位-塑造手法（每行一个，最多 10 个；只报本块新信息）

【世界观线索】本块新出现的设定/规则/环境（最多 3 条，每行一条）

【伏笔线索】本块疑似埋设的伏笔（最多 3 条，每行一条：内容|类型）

要求：只报本块的新信息；没把握写"无"；不要复述剧情，聚焦可借鉴的表达与结构。

本块正文：
%s`, idx, total, block)
}

// buildMergePrompt 中间纪要压缩：多份特征合并去重
func buildMergePrompt(part string) string {
	return fmt.Sprintf(`你是小说拆书编辑的要点合并助手。以下是一本书不同部分的分块特征提取结果（用 --- 分隔）。请合并去重为一份精简的中间纪要（总长控制在 2200 字内）：

- 片段候选：每类场景（开篇/高潮/日常/转折/白描）保留最代表的一段
- 角色/世界观/伏笔线索：去重合并，重复的只留描述最完整的一条
- 保持【片段候选】【角色线索】【世界观线索】【伏笔线索】四个小节结构
- 只保留书中真实出现的内容，不要编造

输入：
%s`, part)
}

// buildAnalyzeSummaryPrompt 最终汇总：基于全书纪要输出标准四部分报告
func buildAnalyzeSummaryPrompt(features string) string {
	return fmt.Sprintf(`你是资深小说拆书编辑。以下是一本书**全部分块**的分析纪要（已覆盖全书内容，未截断）。请基于纪要输出最终拆书报告，严格按以下格式：

【标志性片段】（3-5 段，务必覆盖书中**不同类型/场景**：开篇氛围、高潮冲突、日常对话、情绪转折、场景白描等）
1. <片段标题>
来源：<第几章/大致位置>
片段正文：<原文摘录 150-300 字，保留原文文字与节奏>
风格要点：<句式/节奏/视角/用词特征，1-2 句>
2. <…>
…（纪要中片段候选不足 3 段时，用纪要信息概括补足并在标题后标注"（概述）"）

【主要角色】（列出本书**全部**主要角色与重要配角——群像/长篇小说通常 8-20 个，按重要性排序；只漏掉纯龙套路人）
姓名：<角色名>
定位：<主角/重要配角等>
性格：<1-2 句>
塑造手法：<作者如何写出这个人物：细节/行为/对话/反差，1-2 句，最值得借鉴>
（每个角色重复以上四行，用 --- 分隔）

【世界观】
核心设定：<最独特的世界观/设定核心，1-2 句>
构建手法：<如何让设定可信/有趣：从什么细节切入、如何层层展开，2-3 句>

【伏笔设计】
1. <伏笔内容> | 埋设位置：<第几章/场景> | 类型：<物件/对话/细节/意象> | 作用：<回收后如何影响剧情>
2. <…>
…

要求：全部基于纪要中的原文信息，不要编造；标志性片段尽量用原文摘录；聚焦"值得借鉴的方法"而非剧情复述。

全书分析纪要：
%s`, features)
}
