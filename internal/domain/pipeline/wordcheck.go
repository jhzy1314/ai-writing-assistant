package pipeline

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

// CheckOutlineWordCount 从 Thinker 产出的大纲中提取各节点预估字数，与目标对比做±30%阈值校验。
func CheckOutlineWordCount(outline string, targetWord int, skipCheck bool) *OutlineWordEstimate {
	if skipCheck || targetWord <= 0 || outline == "" {
		return nil
	}
	suggested := 0
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`全文建议.数汇总[：:]\s*(\d+)`),
		regexp.MustCompile(`建议总.数[：:]\s*(\d+)`),
		regexp.MustCompile(`汇总[：:]\s*(\d+)`),
		regexp.MustCompile(`预估总.数[：:]\s*(\d+)`),
	} {
		if m := re.FindStringSubmatch(outline); len(m) >= 2 {
			suggested, _ = strconv.Atoi(m[1])
			break
		}
	}
	if suggested <= 0 {
		nodeRe := regexp.MustCompile(`预估.数[：:]*\s*(\d+)`)
		matches := nodeRe.FindAllStringSubmatch(outline, -1)
		for _, m := range matches {
			if len(m) >= 2 {
				if v, err := strconv.Atoi(m[1]); err == nil {
					suggested += v
				}
			}
		}
	}
	if suggested <= 0 {
		return nil
	}
	ratio := float64(suggested) / float64(targetWord)
	delta := ratio
	if delta < 1 {
		delta = 1 / delta
	}
	if delta <= 1.3 {
		return nil
	}
	oe := &OutlineWordEstimate{
		SuggestedTotal: suggested,
		TargetWord:     targetWord,
		Mismatch:       true,
		Ratio:          math.Round(ratio*100) / 100,
	}
	if ratio < 1.0 {
		oe.Advice = fmt.Sprintf("大纲建议 %d 字，目标 %d 字不足（%.0f%%），建议增加字数或精简大纲节点",
			suggested, targetWord, ratio*100)
	} else {
		oe.Advice = fmt.Sprintf("大纲建议 %d 字，目标 %d 字超额（%.0f%%），建议缩减目标或扩充大纲内容",
			suggested, targetWord, ratio*100)
	}
	return oe
}

// TruncateBySentence 在目标字数附近按句子边界截断文本，保持句子完整不从中切断。
func TruncateBySentence(text string, maxLen int) string {
	if maxLen <= 0 || len([]rune(text)) <= maxLen {
		return text
	}
	runes := []rune(text)
	endMarks := []rune{'。', '！', '？', '…', '\n', '」', '.', '!', '?', '，', ','}
	cutPos := maxLen
	searchStart := maxLen - 80
	if searchStart < 0 {
		searchStart = 0
	}
	for i := maxLen - 1; i >= searchStart; i-- {
		for _, m := range endMarks {
			if runes[i] == m {
				cutPos = i + 1
				goto found
			}
		}
	}
	for i := maxLen; i < len(runes) && i < maxLen+150; i++ {
		for _, m := range endMarks {
			if runes[i] == m {
				cutPos = i + 1
				goto found
			}
		}
	}
found:
	return string(runes[:cutPos])
}
