package pipeline

import (
	"context"
	"fmt"
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
	// 历史前文：加载项目全部章节正文（用户明确要求：读全文、不设预算，保证剧情一致性）
	var allChs []database.ChapterWithVolume
	if chs, err := d.store.ListChapters(ctx, req.ProjectID, ""); err == nil && len(chs) > 0 {
		allChs = chs
		var hb strings.Builder
		for i := range chs {
			if strings.TrimSpace(chs[i].Content) == "" {
				continue
			}
			hb.WriteString(chs[i].Title + "\n")
			hb.WriteString(chs[i].Content + "\n\n")
		}
		if hb.Len() > 0 {
			bundle.HistoryContent = hb.String()
		}
	}
	// 叙事视角识别：统计第一人称叙述者标记（"后来惊鸿跟我说"这类框架句），识别项目主导视角
	if len(allChs) > 0 {
		firstPersonChs := 0
		firstPersonHits := 0
		for i := range allChs {
			c := allChs[i].Content
			if strings.Contains(c, "跟我说") || strings.Contains(c, "跟我") ||
				strings.Contains(c, "我后来") || strings.Contains(c, "我说你") ||
				strings.Contains(c, "我们") || strings.Contains(c, "我心想") {
				firstPersonChs++
			}
			firstPersonHits += strings.Count(c, "跟我说") + strings.Count(c, "我后来")
		}
		if firstPersonChs >= 2 && float64(firstPersonChs)/float64(len(allChs)) >= 0.25 {
			bundle.MaterialText = "【叙事风格参考】本项目前几章带有第一人称叙述者“我”（惊鸿的同学/朋友）讲述故事的框架（如“后来惊鸿跟我说”），后半部分转为第三人称。续写时自然延续这种风格即可：该用“我”的叙述框架时就用，该纯第三人称场景时就第三人称，不必刻意、不要生硬，像前文一样灵活切换。\n" + bundle.MaterialText
		}
	}
	// ChapterID 对应章节的 tags/synopsis 作为补充上下文
	if req.ChapterID != "" {
		if ch, err := d.store.GetChapter(ctx, req.ChapterID); err == nil && ch != nil {
			if ch.Tags != "" {
				bundle.WorldSetting = "【章节标签】" + ch.Tags + "\n" + bundle.WorldSetting
			}
		}
	}
	// 章节摘要（synopsis）注入：自动收集各章 synopsis 作为前情提要（用户前端填入或未来 AI 自动提炼）；
	// 无 synopsis 时回退到前端 withSummary 传入的 PreviousSummaries
	autoSummaries := collectSynopses(allChs)
	if strings.TrimSpace(autoSummaries) != "" {
		bundle.HistoryContent = "【前面章节摘要】\n" + autoSummaries + "\n\n" + bundle.HistoryContent
	} else if req.ContextScope == "withSummary" && strings.TrimSpace(req.PreviousSummaries) != "" {
		bundle.HistoryContent = "【前面章节摘要】\n" + req.PreviousSummaries + "\n\n" + bundle.HistoryContent
	}
	// ========== RAG 按需注入：向量语义检索（懒建索引） ==========
	if req.ContextScope == "smart" || req.ContextScope == "" || req.ContextScope == "full" {
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
	// ========== 素材库融合：按需求语义检索写作素材注入（去AI味） ==========
	if req.ProjectID != "" {
		if items, err := d.store.ListAllWritingMaterialVectors(ctx, req.ProjectID); err == nil && len(items) > 0 {
			query := req.UserDemand + "\n" + req.SelectedText
			qvec := rag.Embed(query)
			type scored struct {
				m     database.WritingMaterial
				score float64
			}
			var results []scored
			for _, m := range items {
				vec, err := rag.Deserialize([]byte(m.Vector))
				if err != nil || len(vec) == 0 {
					continue
				}
				score := rag.Cosine(qvec, vec)
				if score > 0.05 {
					results = append(results, scored{m, score})
				}
			}
			if len(results) > 0 {
				sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
				if len(results) > 3 {
					results = results[:3]
				}
				var b strings.Builder
				for _, r := range results {
					b.WriteString(fmt.Sprintf("[%s] %s", r.m.Category, r.m.Content))
					if r.m.Source != "" {
						b.WriteString("（来源:" + r.m.Source + "）")
					}
					b.WriteString("\n")
				}
				bundle.MaterialText = bundle.MaterialText + "\n\n【素材库融合（仿写参考，仅借鉴表达方式，不得整段照抄）】\n" + b.String()
			}
		}
	}
	// ========== 文风样本库注入：用户自选样本作为参考（本地知识库） ==========
	// 拆书素材按类型分组注入：关键片段/人物卡/世界观/伏笔设计，全部原文读取（不截断）。
	// 这些来自用户拆解的其他书籍——只作创作手法参考，严禁挪用其人物/设定/剧情。
	if len(req.StyleSampleIDs) > 0 {
		if samples, err := d.store.GetStyleSamplesByIDs(ctx, req.StyleSampleIDs); err == nil && len(samples) > 0 {
			var frags, chars, worlds, fores []database.StyleSample
			for _, sm := range samples {
				switch sm.Kind {
				case database.KindCharacter:
					chars = append(chars, sm)
				case database.KindWorld:
					worlds = append(worlds, sm)
				case database.KindForeshadow:
					fores = append(fores, sm)
				default:
					frags = append(frags, sm)
				}
			}
			bundle.MaterialText = bundle.MaterialText +
				"\n\n【外部书籍拆书参考（重要：以下素材来自用户拆解的其他小说，仅供你借鉴创作手法——学习它们如何塑造人物、如何构建世界观、如何埋设伏笔、如何组织文字节奏。你的作品人物、世界观、剧情必须全部来自用户自己的设定，严禁挪用参考书中的任何人物、地名、设定、伏笔或剧情；若参考书内容与用户自己设定冲突，一律以用户自己的设定为准）】"
			writeGroup := func(label string, list []database.StyleSample) {
				if len(list) == 0 {
					return
				}
				var b strings.Builder
				b.WriteString("\n\n【" + label + "】\n")
				for _, sm := range list {
					b.WriteString("《" + sm.Title + "》\n")
					b.WriteString(strings.TrimSpace(sm.Content) + "\n\n")
				}
				bundle.MaterialText = bundle.MaterialText + b.String()
			}
			writeGroup("文风参考样本（关键片段，仅学表达方式与叙事节奏，不得整段照抄）", frags)
			writeGroup("拆书参考·人物卡（仅学人物塑造手法，参考书人物一律不得出现在你的作品中）", chars)
			writeGroup("拆书参考·世界观（仅学世界观构建手法，你的世界观必须用用户自己的设定）", worlds)
			writeGroup("拆书参考·伏笔设计（仅学伏笔埋设与回收手法，不得挪用参考书的具体伏笔）", fores)
		}
	}

	// ========== 自动文风参考：用户未选拆书素材/文风章节时，采样项目已有正文作为文风参考 ==========
	// 项目正文本身就是最好的文风样本——续写必须像前文，而不是像一篇新文章。
	if len(req.StyleSampleIDs) == 0 && strings.TrimSpace(req.MaterialText) == "" && req.ProjectID != "" {
		if chs, err := d.store.ListChapters(ctx, req.ProjectID, ""); err == nil && len(chs) > 0 {
			const headRunes = 1500
			var b strings.Builder
			// 第一章开头（作品定调处，最能体现整体文风）
			if strings.TrimSpace(chs[0].Content) != "" {
				b.WriteString("\n【文风参考·作品开篇】\n")
				b.WriteString(truncateRunes(strings.TrimSpace(chs[0].Content), headRunes))
				b.WriteString("\n")
			}
			// 最近一章开头（当前叙事语感；若当前章就是最后一章则跳过，避免与正文冲突）
			last := chs[len(chs)-1]
			if last.ID != req.ChapterID && strings.TrimSpace(last.Content) != "" {
				b.WriteString("\n【文风参考·最近章节】\n")
				b.WriteString(truncateRunes(strings.TrimSpace(last.Content), headRunes))
				b.WriteString("\n")
			}
			if b.Len() > 0 {
				bundle.MaterialText = "【项目正文·文风参考（硬性要求）】以下是你正在创作的这部小说的既有正文片段。续写必须严格模仿它的文风，逐条对照自检：\n" +
					"1. 事件密集：用具体的事、动作、对话推进剧情，禁止停留在心理独白和内心戏里出不来；\n" +
					"2. 白描克制：少用「像…」比喻，每 1000 字最多 1 处；禁止「动作很自然，甚至有点刻意地自然」这类作者跳出来解说的句子；\n" +
					"3. 对话自然：对话简短、生活化、带具体信息（谁做了什么事、说了什么话），用对话交代剧情；\n" +
					"4. 直接进入事件：禁止用环境氛围渲染开场（如「风从缝隙挤进来，先冻脚踝」式抒情开头），第一句就写发生了什么事；\n" +
					"5. 情绪用动作表达：不写「他屏住呼吸」「他心跳先快了一下」这类直接心理标注，用他做了什么来暗示。\n" +
					"新内容必须读起来像同一部作品的自然延续，而不是一篇新文章。" + b.String() + bundle.MaterialText
			}
		}
	}

	// ========== 伏笔提醒：未回收伏笔注入续写上下文 ==========
	if req.ProjectID != "" {
		if fs, err := d.store.ListForeshadows(ctx, req.ProjectID); err == nil {
			var b strings.Builder
			for _, f := range fs {
				if f.Status == database.ForeshadowPending {
					b.WriteString(fmt.Sprintf("· %s：%s\n", f.Title, f.Description))
				}
			}
			if b.Len() > 0 {
				bundle.HistoryContent = bundle.HistoryContent + "\n\n【未回收伏笔提醒（请在适当情节中回收，勿遗忘）】\n" + b.String()
			}
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

// collectSynopses 收集项目各章节的 synopsis（章节摘要），拼接为前情提要（仅收录非空摘要）
func collectSynopses(chs []database.ChapterWithVolume) string {
	var b strings.Builder
	for i := range chs {
		s := strings.TrimSpace(chs[i].Synopsis)
		if s == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(chs[i].Title + "：" + s)
	}
	return b.String()
}

// truncateRunes 截断到 n 个 rune（中文安全）
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

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

