package rag

import (
	"sort"
	"strings"
)

// Memory 一条记忆片段
type Memory struct {
	ID       string
	Text     string
	Source   string // 章节标题
}

// Retriever 本地 TF-IDF 记忆检索器
// 纯内存运算，零外部依赖。从 index 中召回与 query 最相关的 topK 条记忆
type Retriever struct {
	index     map[string]float64 // term -> idf
	docs      []Memory
	termDoc   []map[string]int // 每篇文档的词频
}

// NewRetriever 构造检索器
func NewRetriever() *Retriever {
	return &Retriever{index: make(map[string]float64)}
}

// Index 构建倒排索引，入参为全部记忆文档
func (r *Retriever) Index(docs []Memory) {
	r.docs = docs
	r.termDoc = make([]map[string]int, len(docs))
	df := make(map[string]int)
	for i, d := range docs {
		tokens := tokenize(d.Text)
		tf := make(map[string]int)
		seen := make(map[string]bool)
		for _, t := range tokens {
			tf[t]++
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
		r.termDoc[i] = tf
	}
	total := float64(len(docs))
	for term, count := range df {
		r.index[term] = idf(total, count)
	}
}

// Search 检索与 query 最相关的 topK 条记忆
func (r *Retriever) Search(query string, topK int) []Memory {
	if len(r.docs) == 0 || topK <= 0 {
		return nil
	}
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return nil
	}
	type scored struct {
		idx   int
		score float64
	}
	scores := make([]scored, len(r.docs))
	for i := range r.docs {
		scores[i] = scored{idx: i, score: r.tfidfScore(i, qTokens)}
	}
	sort.Slice(scores, func(a, b int) bool { return scores[a].score > scores[b].score })
	if topK > len(scores) {
		topK = len(scores)
	}
	result := make([]Memory, 0, topK)
	for i := 0; i < topK; i++ {
		if scores[i].score <= 0 {
			break
		}
		result = append(result, r.docs[scores[i].idx])
	}
	return result
}

func (r *Retriever) tfidfScore(docIdx int, queryTokens []string) float64 {
	tf := r.termDoc[docIdx]
	if tf == nil {
		return 0
	}
	docLen := 0
	for _, c := range tf {
		docLen += c
	}
	if docLen == 0 {
		return 0
	}
	var score float64
	for _, t := range queryTokens {
		tfVal := float64(tf[t]) / float64(docLen)
		score += tfVal * r.index[t]
	}
	return score
}

func tokenize(text string) []string {
	var tokens []string
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ' ' || r == '\t' ||
			r == '。' || r == '，' || r == '！' || r == '？' || r == '；' || r == '：'
	}) {
		word = strings.TrimSpace(word)
		if len([]rune(word)) >= 2 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

func idf(total, df float64) float64 {
	if df <= 0 {
		return 0
	}
	return 1.0 + (total / df)
}
