package pipeline

import (
	"context"
	"strings"
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
	if containsAny(demand, "文笔", "氛围", "文学性", "散文", "诗意", "情感故事", "短视频脚本") {
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
