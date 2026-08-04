package pipeline

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// AITellIssue 规则式 AI 味检测结果（移植自同类型产品 InkOS 的 ai-tells 思路）
type AITellIssue struct {
	Severity    string `json:"severity"`    // warning / info
	Category    string `json:"category"`    // 维度名
	Description string `json:"description"` // 描述
	Suggestion  string `json:"suggestion"`  // 修改建议
}

var (
	aiHedgeWords      = []string{"似乎", "可能", "或许", "大概", "某种程度上", "一定程度上", "在某种意义上"}
	aiTransitionWords = []string{"然而", "不过", "与此同时", "另一方面", "尽管如此", "话虽如此", "但值得注意的是"}
	aiMarkerWords     = []string{"仿佛", "宛如", "不禁", "显然", "竟然", "好像"}
	aiNotButRe        = regexp.MustCompile(`不是.{0,12}而是`)
	aiDashRe          = regexp.MustCompile(`——`)
)

// AnalyzeAITells 对文本做规则式 AI 味检测（纯规则、无 LLM 调用）
func AnalyzeAITells(content string) []AITellIssue {
	issues := []AITellIssue{}
	paragraphs := splitParagraphs(content)

	// dim 1: 段落等长（≥3 段，变异系数 <0.15）
	if len(paragraphs) >= 3 {
		lens := make([]float64, 0, len(paragraphs))
		var sum float64
		for _, p := range paragraphs {
			l := float64(len([]rune(p)))
			lens = append(lens, l)
			sum += l
		}
		mean := sum / float64(len(lens))
		if mean > 0 {
			var vsum float64
			for _, l := range lens {
				d := l - mean
				vsum += d * d
			}
			cv := math.Sqrt(vsum/float64(len(lens))) / mean
			if cv < 0.15 {
				issues = append(issues, AITellIssue{
					Severity:    "warning",
					Category:    "段落等长",
					Description: "段落长度变异系数仅" + trimFloat(cv) + "（阈值<0.15），各段长度过于均匀，呈AI生成特征",
					Suggestion:  "增加段落长度差异：短段落用于节奏加速或冲击，长段落用于沉浸描写",
				})
			}
		}
	}

	// dim 2: 套话词密度（>3 次/千字）
	total := len([]rune(content))
	if total > 0 {
		hedgeCount := countWordMatches(content, aiHedgeWords)
		density := float64(hedgeCount) / (float64(total) / 1000)
		if density > 3 {
			issues = append(issues, AITellIssue{
				Severity:    "warning",
				Category:    "套话密度",
				Description: "套话词（似乎/可能/或许/大概等）密度" + trimFloat(density) + "次/千字（阈值>3），语气过于模糊犹豫",
				Suggestion:  "用确定性叙述替代模糊表达：去掉「似乎」直接描述状态，用具体细节替代「可能」",
			})
		}
	}

	// dim 3: 转折词重复（同一转折词 ≥3 次）
	for _, w := range aiTransitionWords {
		if c := strings.Count(content, w); c >= 3 {
			issues = append(issues, AITellIssue{
				Severity:    "warning",
				Category:    "公式化转折",
				Description: "转折词「" + w + "」重复" + itoa(c) + "次，同一转折模式≥3次暴露AI生成痕迹",
				Suggestion:  "用情节自然转折替代转折词：动作切入、时间跳跃、视角切换，或换用不同过渡手法",
			})
		}
	}

	// dim 4: AI 标记词密度（仿佛/宛如/不禁/显然/竟然/好像，>3 次/千字）
	if total > 0 {
		markerCount := countWordMatches(content, aiMarkerWords)
		md := float64(markerCount) / (float64(total) / 1000)
		if md > 3 {
			issues = append(issues, AITellIssue{
				Severity:    "warning",
				Category:    "AI标记词堆砌",
				Description: "情绪中介词（仿佛/宛如/不禁/显然等）密度" + trimFloat(md) + "次/千字（阈值>3）",
				Suggestion:  "删除或替换为具体动作/细节：把「他不禁笑了」改为「他嘴角一扬」",
			})
		}
	}

	// dim 5: 列表式结构（≥3 句连续相同开头）
	sentences := splitSentences(content)
	if len(sentences) >= 3 {
		maxRun, run := 1, 1
		for i := 1; i < len(sentences); i++ {
			if sentencePrefix(sentences[i]) == sentencePrefix(sentences[i-1]) {
				run++
				if run > maxRun {
					maxRun = run
				}
			} else {
				run = 1
			}
		}
		if maxRun >= 3 {
			issues = append(issues, AITellIssue{
				Severity:    "info",
				Category:    "列表式结构",
				Description: "检测到" + itoa(maxRun) + "句连续以相同开头，呈现列表式AI生成结构",
				Suggestion:  "变换句式开头：用不同主语、时间词、动作词开头，打破列表感",
			})
		}
	}

	// dim 6: "不是……而是……"句式 + 破折号滥用
	if n := len(aiNotButRe.FindAllString(content, -1)); n >= 2 {
		issues = append(issues, AITellIssue{
			Severity:    "warning",
			Category:    "AI句式",
			Description: "「不是……而是……」句式出现" + itoa(n) + "次，为典型AI生成句式",
			Suggestion:  "改写为直接陈述或口语化表达",
		})
	}
	if n := len(aiDashRe.FindAllString(content, -1)); n >= 3 {
		issues = append(issues, AITellIssue{
			Severity:    "info",
			Category:    "破折号滥用",
			Description: "破折号「——」出现" + itoa(n) + "次，过量使用呈AI痕迹",
			Suggestion:  "删除破折号，改用逗号/句号切分，或重写该句",
		})
	}

	// dim 7: "了"字连排（连续 3+ 个分句都以"了"结尾）
	if c := strings.Count(content, "了"); total > 0 {
		if float64(c)/(float64(total)/1000) > 15 {
			issues = append(issues, AITellIssue{
				Severity:    "info",
				Category:    "「了」字堆砌",
				Description: "「了」字密度" + trimFloat(float64(c)/(float64(total)/1000)) + "次/千字，叙述显拖沓",
				Suggestion:  "删减「了」字：走→走过去，拿了→端起，喝了一口→灌了一口",
			})
		}
	}

	return issues
}

func splitParagraphs(s string) []string {
	parts := strings.Split(s, "\n")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len([]rune(p)) > 0 {
			out = append(out, p)
		}
	}
	return out
}

func splitSentences(s string) []string {
	re := regexp.MustCompile(`[。！？\n]`)
	parts := re.Split(s, -1)
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len([]rune(p)) > 2 {
			out = append(out, p)
		}
	}
	return out
}

// ========== 句子级解释性旁白检测（供 AI 味闭环定位问题句） ==========

// SentenceIssue 规则检测定位到具体句子（让 Worker 只改问题句，不整段重写）
type SentenceIssue struct {
	ParaIndex int    `json:"para_index"` // 段落索引（从1）
	Sentence  string `json:"sentence"`   // 问题句原文（截断）
	Category  string `json:"category"`
	FixHint   string `json:"fix_hint"`
}

var (
	// 第三人称解说模式（作者跳出来替读者总结/点破心理）
	thirdPersonExplainRe = []*regexp.Regexp{
		regexp.MustCompile(`[他她](知道|明白|意识到|清楚|心想)[^，。！？!?]{2,14}`),
		regexp.MustCompile(`[他她](心里|内心)(明白|清楚)[^，。！？!?]{2,14}`),
		regexp.MustCompile(`这意味着[^，。！？!?]{2,14}`),
		regexp.MustCompile(`(实际上|事实上)[^，。！？!?]{2,14}`),
		regexp.MustCompile(`其实[他她][^，。！？!?]{2,14}`),
	}
	// 无主语总结词（任何视角下都偏 AI 腔，第一人称段也检测）
	authorSummaryRe = []*regexp.Regexp{
		regexp.MustCompile(`这意味着[^，。！？!?]{2,14}`),
		regexp.MustCompile(`(实际上|事实上)[^，。！？!?]{2,14}`),
	}
	// 引号内对话内容（白名单：人物说话不算作者旁白）
	quoteContentRe = regexp.MustCompile(`["“”'‘’「」『』][^"“”'‘’「」『』]{0,80}["“”'‘’「」『』]`)
)

// DetectSentenceIssues 规则检测"解释性旁白"并定位到段/句。
// 白名单（不误杀）：①引号内对话内容；②第一人称叙述段（转述体"我知道/我后来才明白"是正常写法，
// 只检测"这意味着/实际上/事实上"这类无主语总结词）。
func DetectSentenceIssues(content string) []SentenceIssue {
	var out []SentenceIssue
	paras := splitParagraphs(content)
	for i, p := range paras {
		noQuote := quoteContentRe.ReplaceAllString(p, " ") // 剥离对话
		if strings.TrimSpace(noQuote) == "" {
			continue
		}
		patterns := thirdPersonExplainRe
		if isFirstPersonPara(noQuote) {
			patterns = authorSummaryRe // 第一人称段只查无主语总结词
		}
		for _, s := range splitSentences(noQuote) {
			for _, re := range patterns {
				if re.MatchString(s) {
					out = append(out, SentenceIssue{
						ParaIndex: i + 1,
						Sentence:  truncateRunes(strings.TrimSpace(s), 40),
						Category:  "解释性旁白",
						FixHint:   "改为具体动作/细节，不要直接点破人物心理或总结含义",
					})
					break
				}
			}
		}
	}
	return out
}

// isFirstPersonPara 段内"我"出现≥2次且多于"他/她"，判定为第一人称叙述段（转述体/第一人称）
func isFirstPersonPara(p string) bool {
	me := strings.Count(p, "我")
	him := strings.Count(p, "他") + strings.Count(p, "她")
	return me >= 2 && me >= him
}

func sentencePrefix(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) < 2 {
		return string(r)
	}
	return string(r[:2])
}

func countWordMatches(s string, words []string) int {
	n := 0
	for _, w := range words {
		n += strings.Count(s, w)
	}
	return n
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
