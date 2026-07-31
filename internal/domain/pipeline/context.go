package pipeline

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/rag"
)

// buildContext 组装共享上下文：优先使用请求体显式传入的字段，缺失则从数据库按 project_id 加载
func (d *Dispatcher) buildContext(ctx context.Context, req GenerateRequest) ContextBundle {
	bundle := ContextBundle{
		WorldSetting:      req.WorldSetting,
		CharacterSetting:  req.CharacterSetting,
		HistoryContent:    req.HistoryContent,
		MaterialText:      req.MaterialText,
		PreviousSummaries: req.PreviousSummaries,
	}
	if req.ProjectID == "" {
		return bundle
	}
	// ChapterID 优先：加载对应章节内容作为前文
	if req.ChapterID != "" {
		if ch, err := d.store.GetChapter(ctx, req.ChapterID); err == nil && ch != nil {
			if strings.TrimSpace(bundle.HistoryContent) == "" {
				bundle.HistoryContent = ch.Content
			}
			// 加载章节关联的 tags/synopsis 作为补充上下文
			if ch.Tags != "" {
				bundle.WorldSetting = "【章节标签】" + ch.Tags + "\n" + bundle.WorldSetting
			}
		}
	}
	// ContextScope=withSummary 时拼接前面章节摘要
	if req.ContextScope == "withSummary" && strings.TrimSpace(req.PreviousSummaries) != "" {
		bundle.HistoryContent = "【前面章节摘要】\n" + req.PreviousSummaries + "\n\n" + bundle.HistoryContent
	}
	// ContextScope=smart 时自动分层
	if req.ContextScope == "smart" {
		if chs, err := d.store.ListChapters(ctx, req.ProjectID, ""); err == nil && len(chs) > 0 {
			var builder strings.Builder
			total := len(chs)
			for i := total - 1; i >= 0 && i >= total-2; i-- {
				if chs[i].Content != "" {
					builder.WriteString("【" + chs[i].Title + "（全文）】\n" + chs[i].Content + "\n\n")
				}
			}
			for i := total - 3; i >= 0 && i >= total-7; i-- {
				if chs[i].Content != "" {
					summary := strings.Join(strings.Split(chs[i].Content, "。")[:3], "。") + "。"
					builder.WriteString("【" + chs[i].Title + "（摘要）】" + summary + "\n\n")
				}
			}
			for i := 0; i < total && i < total-7; i++ {
				if chs[i].Title != "" {
					builder.WriteString("· " + chs[i].Title)
					if chs[i].Synopsis != "" {
						builder.WriteString("：" + chs[i].Synopsis)
					}
					builder.WriteString("\n")
				}
			}
			bundle.HistoryContent = builder.String()
		}
	}
	// ========== RAG 按需注入：向量语义检索（懒建索引） ==========
	if req.ContextScope == "smart" || req.ContextScope == "" {
		if d.rag != nil {
			// 懒建索引：项目尚无向量块时全量建一次
			n, _ := d.store.CountRAGChunks(ctx, req.ProjectID)
			if n == 0 {
				_ = d.rag.IndexChapters(ctx, req.ProjectID)
			}
			query := req.UserDemand + "\n" + req.SelectedText + "\n" + req.HistoryContent
			chunks, err := d.rag.Search(ctx, req.ProjectID, req.ChapterID, query, 3)
			if err == nil && len(chunks) > 0 {
				relevant := rag.BuildContextText(chunks)
				if relevant != "" {
					bundle.HistoryContent = bundle.HistoryContent + "\n\n【RAG相关记忆（向量检索）】\n" + relevant
				}
			}
		}
		// 实体检索兜底（向量未命中时补充精确匹配）
		if chs, err := d.store.ListChapters(ctx, req.ProjectID, ""); err == nil && len(chs) > 0 {
			relevant := d.ragRetrieve(ctx, req, chs)
			if relevant != "" {
				bundle.HistoryContent = bundle.HistoryContent + "\n\n【RAG相关记忆（实体匹配）】\n" + relevant
			}
		}
	}

	// 从数据库补充缺失项
	if strings.TrimSpace(bundle.WorldSetting) == "" {
		if t, err := d.store.WorldSettingsText(ctx, req.ProjectID); err == nil && t != "" {
			bundle.WorldSetting = t
		}
	}
	if strings.TrimSpace(bundle.CharacterSetting) == "" {
		if t, err := d.store.CharactersText(ctx, req.ProjectID); err == nil && t != "" {
			bundle.CharacterSetting = t
		}
	}
	if strings.TrimSpace(bundle.HistoryContent) == "" {
		if v, err := d.store.LatestVersion(ctx, req.ProjectID); err == nil && v != nil && v.Content != "" {
			bundle.HistoryContent = v.Content
		}
	}
	return bundle
}

// detectPipeline 按强制规则自动判定流水线（run_mode=auto 时调用）
func detectPipeline(req GenerateRequest) PipelineName {
	// 1. 文本<500字 / 局部改写 → 轻量化快速模式
	if req.SelectedText != "" && runeLen(req.SelectedText) < 500 {
		return PipelineLight
	}
	demand := req.UserDemand
	// 2. 用户标注严谨/正式/学术/公文 → 严谨模式
	if containsAny(demand, "严谨", "正式", "学术", "公文", "论文", "报告", "纪实") {
		return PipelineStrict
	}
	// 3. 用户强调文笔/氛围感/文学性 → 文艺创作模式
	// （N4 细化：补充韵味/意境/美感/画面感/细腻等文学表达触发词）
	if containsAny(demand,
		"文笔", "氛围", "文学性", "散文", "诗意", "情感故事", "短视频脚本",
		"韵味", "意境", "美感", "画面感", "细腻", "优美", "含蓄", "留白", "张力",
		"慢镜头", "细节描写", "环境描写", "心理描写", "文风", "文采") {
		return PipelineArt
	}
	// 4. 小说/故事创作且无标注 → 标准通用创作
	return PipelineStandard
}

// resolvePipeline 解析最终流水线（结合显式 run_mode 与自动判定）
func resolvePipeline(req GenerateRequest, lightCharLimit int) PipelineName {
	switch req.RunMode {
	case ModeDraft:
		return PipelineDraft
	case ModeCollab:
		return PipelineCollab
	case ModeOrchestrated:
		return PipelineOrchestrated
	case ModeStrict:
		return PipelineStrict
	case ModeArt:
		return PipelineArt
	case ModeLight:
		return PipelineLight
	case ModeManual:
		return PipelineManual
	case ModeAuto:
		// 自动判定补充：输入文本过短也走轻量
		if req.SelectedText == "" && req.TargetWord > 0 && req.TargetWord < lightCharLimit && isLightDemand(req.UserDemand) {
			return PipelineLight
		}
		return detectPipeline(req)
	default:
		return detectPipeline(req)
	}
}

// isLightDemand 判断需求是否为轻量化任务（缩写/扩写/摘要/改写/翻译等关键词）
func isLightDemand(demand string) bool {
	return containsAny(demand, "缩写", "扩写", "摘要", "总结", "提取关键词", "改写", "翻译", "润色这段", "修改这段")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func runeLen(s string) int { return len([]rune(s)) }

func trimSpace(s string) string { return strings.TrimSpace(s) }

// needsSegmentation 目标生成文本>4000字需自动分段
func needsSegmentation(req GenerateRequest) bool {
	return req.TargetWord > 4000
}

// segmentSize 单段最大字数（控制在 token 上限内）
const segmentSize = 3500

// numSegments 计算分段数
func numSegments(req GenerateRequest) int {
	if req.TargetWord <= 0 {
		return 1
	}
	n := req.TargetWord / segmentSize
	if req.TargetWord%segmentSize != 0 {
		n++
	}
	return n
}

// ragRetrieve RAG 按需注入：从用户需求/选中文本/最近前文提取关键实体，
// 扫描全书章节，将命中实体的相关片段按相关度排序返回（每段截取 snippet 长度）。
// 不依赖向量库：实体关键词匹配（书名号/人物卡/专有名词），零成本、即时生效。
func (d *Dispatcher) ragRetrieve(ctx context.Context, req GenerateRequest, chs []database.ChapterWithVolume) string {
	// 1. 提取候选实体词
	terms := d.ragExtractTerms(ctx, req, chs)
	if len(terms) == 0 {
		return ""
	}

	// 2. 确定当前章节索引（排除当前章与最近 2 章，避免重复注入前文）
	curIdx := -1
	for i := range chs {
		if chs[i].ID == req.ChapterID {
			curIdx = i
			break
		}
	}

	// 3. 扫描早前章节，统计命中
	type hit struct {
		idx    int
		title  string
		score  int
		snippet string
	}
	var hits []hit
	start := 0
	if curIdx >= 0 {
		start = curIdx - 2 // 从当前章往前 2 章之前开始（最近 2 章已有全文）
		if start < 0 {
			start = 0
		}
	}
	for i := 0; i < start; i++ {
		c := chs[i]
		if strings.TrimSpace(c.Content) == "" {
			continue
		}
		score := 0
		for _, term := range terms {
			if strings.Contains(c.Content, term) {
				score += len([]rune(term)) // 长实体权重更高
			}
		}
		if score > 0 {
			hits = append(hits, hit{idx: i, title: c.Title, score: score, snippet: snippetAround(c.Content, terms)})
		}
	}

	// 4. 按相关度排序（分数高在前），取前 3 段
	sort.Slice(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	if len(hits) > 3 {
		hits = hits[:3]
	}
	if len(hits) == 0 {
		return ""
	}

	// 5. 拼装
	var b strings.Builder
	for _, h := range hits {
		b.WriteString("【第" + itoa(h.idx+1) + "章 " + h.title + "（相关片段）】\n" + h.snippet + "\n\n")
	}
	return strings.TrimSpace(b.String())
}

// ragExtractTerms 提取检索实体：书名号内容 + 人物卡名字 + 需求中的长名词
func (d *Dispatcher) ragExtractTerms(ctx context.Context, req GenerateRequest, chs []database.ChapterWithVolume) []string {
	seen := map[string]bool{}
	var terms []string
	add := func(w string) {
		w = strings.TrimSpace(w)
		if len([]rune(w)) >= 2 && !seen[w] {
			seen[w] = true
			terms = append(terms, w)
		}
	}

	src := req.UserDemand + "\n" + req.SelectedText

	// a. 书名号《》内容
	re := regexp.MustCompile(`《([^》]+)》`)
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		add(m[1])
	}

	// b. 人物卡名字（后端查库）
	if chars, err := d.store.ListCharacters(ctx, req.ProjectID); err == nil {
		for _, ch := range chars {
			if ch.Name != "" {
				add(ch.Name)
			}
		}
	}

	// c. 需求里的连续中文词（>=2字），取出现次数多的
	freq := map[string]int{}
	words := regexp.MustCompile(`[\p{Han}]{2,6}`).FindAllString(src, -1)
	for _, w := range words {
		freq[w]++
	}
	for w, n := range freq {
		if n >= 2 {
			add(w)
		}
	}

	if len(terms) > 12 {
		terms = terms[:12]
	}
	return terms
}

// snippetAround 截取命中实体附近的片段（实体前后各 ~80 字）
func snippetAround(content string, terms []string) string {
	bestPos := -1
	bestTerm := ""
	for _, term := range terms {
		if pos := strings.Index(content, term); pos >= 0 {
			if bestPos < 0 || pos < bestPos {
				bestPos = pos
				bestTerm = term
			}
		}
	}
	if bestPos < 0 || bestTerm == "" {
		return content[:minInt(120, len([]rune(content)))]
	}
	runes := []rune(content)
	start := bestPos - 60
	if start < 0 {
		start = 0
	}
	end := bestPos + len([]rune(bestTerm)) + 60
	if end > len(runes) {
		end = len(runes)
	}
	if end-start > 240 {
		end = start + 240
	}
	seg := string(runes[start:end])
	if start > 0 {
		seg = "…" + seg
	}
	if end < len(runes) {
		seg = seg + "…"
	}
	return seg
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

