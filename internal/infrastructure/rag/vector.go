package rag

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

// ============================================================
// rag/vector.go —— 轻量本地向量化（无外部 embedding 依赖）
// 中文 2-gram 特征 + TF-IDF 权重 + 余弦相似度
// ============================================================

// Vector 稀疏向量：特征哈希 -> 权重
type Vector map[int]float64

// 特征维度（哈希空间大小）
const featureDim = 1 << 16

// tokenize 中文文本切分：保留中文字符的 2-gram + 单个汉字 + 英文词
func tokenize(text string) []string {
	runes := []rune(strings.ToLower(text))
	var tokens []string
	// 2-gram（中文核心特征）
	for i := 0; i < len(runes)-1; i++ {
		a, b := runes[i], runes[i+1]
		if isHan(a) && isHan(b) {
			tokens = append(tokens, string([]rune{a, b}))
		}
	}
	// 单个汉字（处理 1 字实体）
	for _, r := range runes {
		if isHan(r) {
			tokens = append(tokens, string(r))
		}
	}
	// 英文/数字词
	for _, w := range strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z'))
	}) {
		w = strings.ToLower(w)
		if len(w) >= 2 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

func isHan(r rune) bool { return r >= 0x4E00 && r <= 0x9FFF }

// hash 字符串哈希到特征空间
func hash(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h % featureDim
}

// Embed 文本 -> 稀疏向量（词频加权，含 idf 可选）
func Embed(text string) Vector {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return Vector{}
	}
	vec := Vector{}
	for _, t := range tokens {
		vec[hash(t)]++
	}
	// 归一化到 [0,1]
	maxF := 0.0
	for _, f := range vec {
		if float64(f) > maxF {
			maxF = float64(f)
		}
	}
	for k, f := range vec {
		vec[k] = float64(f) / maxF // 子线性归一化（类似 BM25 的饱和）
	}
	return vec
}

// Cosine 余弦相似度
func Cosine(a, b Vector) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	// 用小向量做外层遍历
	small, big := a, b
	if len(a) > len(b) {
		small, big = b, a
	}
	dot := 0.0
	for k, v := range small {
		if w, ok := big[k]; ok {
			dot += v * w
		}
	}
	na := norm(a)
	nb := norm(b)
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (na * nb)
}

func norm(v Vector) float64 {
	s := 0.0
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

// Serialize 向量 -> JSON bytes（存库）
func (v Vector) Serialize() ([]byte, error) {
	type item struct {
		K int     `json:"k"`
		V float64 `json:"v"`
	}
	items := make([]item, 0, len(v))
	for k, val := range v {
		items = append(items, item{k, val})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].K < items[j].K })
	return json.Marshal(items)
}

// Deserialize JSON bytes -> 向量
func Deserialize(data []byte) (Vector, error) {
	type item struct {
		K int     `json:"k"`
		V float64 `json:"v"`
	}
	var items []item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	v := Vector{}
	for _, it := range items {
		v[it.K] = it.V
	}
	return v, nil
}
