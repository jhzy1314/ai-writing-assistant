package database

import (
	"testing"

	"github.com/ai-novel/studio/internal/infrastructure/quality"
)

// TestWordCountParity 钉死口径：quality.WordCount 必须与数据库 wordCount 完全一致
// （生成流水线的 AI 味密度计算与 DB 字数统计共用同一口径，防止漂移）
func TestWordCountParity(t *testing.T) {
	cases := []string{
		"你好，世界！",
		"你好，世界！\n  换行 空格",
		"Hello world! 中英 mixed 123",
		"全角空格\u3000和制表符\t和换行\n",
		"不间断空格\u00a0与普通空格 ",
		"行分隔符\u2028段落分隔符\u2029",
		"",
		"   \t\n\r  ",
		"「引号」标点，。！？；：（）——……",
	}
	for _, c := range cases {
		want := wordCount(c)
		got := quality.WordCount(c)
		if want != got {
			t.Errorf("口径漂移: wordCount(%q)=%d, quality.WordCount=%d", c, want, got)
		}
	}
}
