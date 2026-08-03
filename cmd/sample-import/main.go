package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// ============================================================
// cmd/sample-import —— 文风样本导入工具
// 读取用户自购的正版书籍文件（txt/epub），按章节切分、清洗、
// 提取片段，输出 samples.json（随后由 /api/style-samples/import 入库）。
// 仅本地个人使用，样本不分发。
// 用法: go run ./cmd/sample-import -out samples.json <file1> <file2> ...
// ============================================================

// StyleProfile 作品风格档案（作品 → 文风特征元数据，人工维护）
type StyleProfile struct {
	Author   string   `json:"author"`
	Genre    string   `json:"genre"`     // 流派
	Tags     []string `json:"tags"`      // 标签
	Style    string   `json:"style"`     // 文风特征描述
	Samples  int      `json:"samples"`   // 计划抽取样本数
	SkipTail bool     `json:"skip_tail"` // 是否跳过章末 PS/求票段落
}

// 作品风格档案：按文件名/内容关键词匹配
var profiles = []struct {
	key     string // 匹配关键词（书名或作者）
	profile StyleProfile
}{
	{"龙族", StyleProfile{Author: "江南", Genre: "青春幻想/热血", Tags: []string{"龙族", "江南", "青春", "热血", "幻想"}, Style: "少年感强、热血燃向、画面感浓烈、幽默与悲壮交织、擅长群像与名场面台词", Samples: 15, SkipTail: true}},
	{"诡秘之主", StyleProfile{Author: "爱潜水的乌贼", Genre: "克苏鲁/维多利亚奇幻", Tags: []string{"诡秘之主", "乌贼", "克苏鲁", "维多利亚", "悬疑", "蒸汽朋克"}, Style: "维多利亚时代背景、克苏鲁式诡异氛围、心理描写细腻、伏笔严密、叙事节奏沉稳、冷幽默", Samples: 15, SkipTail: true}},
	{"惊悚乐园", StyleProfile{Author: "三天两觉", Genre: "无限流/悬疑", Tags: []string{"惊悚乐园", "三天两觉", "无限流", "悬疑", "幽默"}, Style: "无限流副本结构、悬疑氛围与吐槽幽默并存、对话机锋、节奏明快", Samples: 12, SkipTail: true}},
	{"我真没想重生啊", StyleProfile{Author: "柳岸花又明", Genre: "都市重生/年代文", Tags: []string{"我真没想重生啊", "柳岸花又明", "都市", "重生", "年代"}, Style: "都市重生、年代质感、人物成长线、生活细节真实、幽默与温情并存", Samples: 12, SkipTail: true}},
	{"同桌凶猛", StyleProfile{Author: "柳下挥", Genre: "都市校园/轻松", Tags: []string{"同桌凶猛", "柳下挥", "都市", "校园", "轻松"}, Style: "轻松搞笑、对话机锋、节奏明快、都市生活流、幽默吐槽", Samples: 12, SkipTail: true}},
	{"白夜行", StyleProfile{Author: "东野圭吾", Genre: "悬疑推理", Tags: []string{"白夜行", "东野圭吾", "悬疑", "推理", "暗黑"}, Style: "冷峻克制、双线叙事、心理暗流涌动、细节伏笔绵密、悲剧宿命感、对话简短留白", Samples: 12, SkipTail: false}},
	{"此间的少年", StyleProfile{Author: "江南", Genre: "青春校园", Tags: []string{"此间的少年", "江南", "青春", "校园", "幽默"}, Style: "校园青春、轻松幽默、日常细节丰富、人物群像鲜活、金庸人名梗彩蛋", Samples: 10, SkipTail: true}},
	{"我真不是她徒弟", StyleProfile{Author: "孤儿管家", Genre: "仙侠/日常", Tags: []string{"我真不是她徒弟", "仙侠", "日常", "轻松"}, Style: "仙侠日常向、轻松幽默、师徒互动、生活细节丰富", Samples: 8, SkipTail: true}},
}

// Sample 一条文风样本（与 database.StyleSample 字段对应）
type Sample struct {
	Title      string `json:"title"`       // 样本名（作品·章节）
	Author     string `json:"author"`      // 作者
	Category   string `json:"category"`    // 风格分类（流派）
	Style      string `json:"style"`       // 文风特征描述（拼入 category 展示）
	Content    string `json:"content"`     // 片段正文
	SourceFile string `json:"source_file"` // 来源文件
}

var chapterRe = regexp.MustCompile(`(?m)^\s*(?:[0-9]+\.\s*)?(第\s*[0-9一二三四五六七八九十百千万零两]+\s*[章回节卷部集]|[卷部集]\s*[0-9一二三四五六七八九十百千万零两]+|楔子|序章|尾声|番外)[^\n]*$`)

// 精确噪声行（整行匹配）
var noiseExact = map[string]bool{
	"本章完": true, "（本章完）": true, "(本章完)": true, "未完待续": true, "章节完": true,
}

// 求票/求订阅前缀
var noisePrefixes = []string{"求月票", "求推荐票", "求订阅", "求收藏", "求保底"}

// 水印/广告子串
var noiseSubstrs = []string{"知轩藏书", "速读谷", "更多精校小说", "用户上传", "本书首发", "提供给你无错章节", "手动滑稽"}

var brRe = regexp.MustCompile(`<br\s*/?>|</?br>`)

// decodeText 检测编码：UTF-8 合法则直用，否则按 GBK 解码（中文网文 txt 常见编码）
func decodeText(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	dec := simplifiedchinese.GBK.NewDecoder()
	out, _, err := transform.Bytes(dec, data)
	if err != nil {
		return string(data) // 解码失败则原样返回
	}
	return string(out)
}

// readTXT 读取 txt 文件并清洗水印头
func readTXT(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := decodeText(data)
	// 去掉文件头水印（==== 分隔的版权声明/网站广告块）；仅在文件头 2000 字内查找，避免误伤正文分隔线
	head := text
	if len([]rune(head)) > 2000 {
		head = string([]rune(head)[:2000])
	}
	if i := strings.Index(head, "-----------------------------------"); i >= 0 {
		if j := strings.Index(text[i:], "\n"); j >= 0 {
			text = text[i+j+1:]
		}
	}
	// 去掉 <br> 类残留标签
	text = brRe.ReplaceAllString(text, "\n")
	return text, nil
}

// readEPUB 解包 epub，拼接全部 xhtml 文本（去标签）
func readEPUB(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	var parts []string
	entryCount := 0
	for _, f := range zr.File {
		lower := strings.ToLower(f.Name)
		if !strings.HasSuffix(lower, ".xhtml") && !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
			continue
		}
		entryCount++
		if entryCount > 500 {
			break // 恶意/异常 epub 的 HTML 条目数上限（防内存膨胀；图片/字体等不计）
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(rc, 2<<20))
		rc.Close()
		parts = append(parts, stripHTML(string(data)))
	}
	return strings.Join(parts, "\n"), nil
}

// stripHTML 粗略去除 HTML 标签与实体（样本提取用途足够）
var scriptStyleRe = regexp.MustCompile(`(?s)<(script|style)[^>]*>.*?</(script|style)>`)
var anyTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = scriptStyleRe.ReplaceAllString(s, "")
	s = anyTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return s
}

// isNoise 判断行是否为水印/求票/注释噪声（精确行/前缀/子串三级匹配，避免误删正常正文）
func isNoise(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return true
	}
	if noiseExact[t] {
		return true
	}
	for _, p := range noisePrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	for _, s := range noiseSubstrs {
		if strings.Contains(t, s) {
			return true
		}
	}
	// 纯标点/符号行
	if len([]rune(t)) <= 2 {
		return true
	}
	return false
}

// splitChapters 按章节标题切分文本
func splitChapters(text string) []string {
	// 找到所有标题行位置
	var titles []int
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if chapterRe.MatchString(l) {
			titles = append(titles, i)
		}
	}
	if len(titles) == 0 {
		return []string{text}
	}
	var chs []string
	for i, ti := range titles {
		start := ti + 1
		end := len(lines)
		if i+1 < len(titles) {
			end = titles[i+1]
		}
		body := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(body) != "" {
			chs = append(chs, body)
		}
	}
	return chs
}

// cleanChapter 清洗章节正文：去噪声行、去尾部求票段
func cleanChapter(content string, skipTail bool) string {
	lines := strings.Split(content, "\n")
	var kept []string
	for _, l := range lines {
		if isNoise(l) {
			continue
		}
		kept = append(kept, strings.TrimSpace(l))
	}
	text := strings.Join(kept, "\n")
	// 去掉末尾的 PS/求票段（按常见模式截断）
	if skipTail {
		if i := strings.Index(text, "PS："); i >= 0 {
			text = text[:i]
		}
		if i := strings.Index(text, "PS:"); i >= 0 {
			text = text[:i]
		}
	}
	return strings.TrimSpace(text)
}

// extractSamples 从一本书提取样本（每章前 maxLen 字，最多 maxSamples 条）
func extractSamples(file, bookName string, profile StyleProfile, maxLen, maxSamples int) []Sample {
	var full string
	var err error
	switch strings.ToLower(filepath.Ext(file)) {
	case ".epub":
		full, err = readEPUB(file)
	default:
		full, err = readTXT(file)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 读取失败 %s: %v\n", file, err)
		return nil
	}
	chs := splitChapters(full)
	var samples []Sample
	used := 0
	for ci, ch := range chs {
		if used >= maxSamples {
			break
		}
		clean := cleanChapter(ch, profile.SkipTail)
		r := []rune(clean)
		if len(r) < 150 {
			continue // 太短跳过（多为空章/目录）
		}
		if len(r) > maxLen {
			// 截断时回退到最近的句子边界（句号/叹号/问号/引号/换行），避免样本断在句中
			cut := maxLen
			for i := maxLen; i > maxLen-40 && i > 0; i-- {
				if r[i-1] == '。' || r[i-1] == '！' || r[i-1] == '？' || r[i-1] == '\n' || r[i-1] == '”' || r[i-1] == '"' {
					cut = i
					break
				}
			}
			r = r[:cut]
		}
		samples = append(samples, Sample{
			Title:      fmt.Sprintf("%s·第%d章", bookName, ci+1),
			Author:     profile.Author,
			Category:   profile.Genre,
			Style:      profile.Style,
			Content:    string(r),
			SourceFile: filepath.Base(file),
		})
		used++
	}
	return samples
}

// matchProfile 按文件内容匹配风格档案（文件名不可靠，如 clipboard 临时名）
func matchProfile(file string) (string, StyleProfile, bool) {
	ext := strings.ToLower(filepath.Ext(file))
	var head string
	if ext == ".epub" {
		if full, err := readEPUB(file); err == nil {
			head = full
		}
	} else {
		if data, err := os.ReadFile(file); err == nil {
			head = decodeText(data)
		}
	}
	if len(head) > 8000 {
		head = head[:8000]
	}
	for _, p := range profiles {
		if strings.Contains(head, p.key) {
			return p.key, p.profile, true
		}
	}
	// 未匹配时打印解码后头部供确认书名（声明块可能较长）
	headRunes := []rune(head)
	if len(headRunes) > 300 {
		headRunes = headRunes[:300]
	}
	fmt.Fprintf(os.Stderr, "⚠ 未匹配档案，文件头部: %s\n", strings.ReplaceAll(string(headRunes), "\n", " "))
	return "", StyleProfile{}, false
}

func main() {
	out := flag.String("out", "samples.json", "输出 JSON 路径")
	maxLen := flag.Int("max-len", 600, "单样本最大字数")
	dbPath := flag.String("db", "", "直接入库的 SQLite 路径（提供时跳过 JSON 输出）")
	rebuild := flag.Bool("rebuild", false, "入库前清空 style_samples（配合 -db 使用，重建样本）")
	flag.Parse()
	files := flag.Args()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "用法: go run ./cmd/sample-import [-db data/ai-novel.db] [-out samples.json] <书籍文件...>")
		os.Exit(1)
	}

	var all []Sample
	for _, f := range files {
		key, profile, ok := matchProfile(f)
		if !ok {
			fmt.Fprintf(os.Stderr, "⚠ 未匹配风格档案，跳过: %s\n", f)
			continue
		}
		samples := extractSamples(f, key, profile, *maxLen, profile.Samples)
		if len(samples) == 0 {
			fmt.Fprintf(os.Stderr, "⚠ 未提取到样本: %s\n", f)
			continue
		}
		all = append(all, samples...)
		fmt.Printf("✓ %s: 提取 %d 条样本（%s）\n", key, len(samples), f)
	}
	if len(all) == 0 {
		fmt.Fprintln(os.Stderr, "未提取到任何样本")
		os.Exit(1)
	}
	if *dbPath != "" {
		// 直接入库（幂等：同 source_file+title 跳过；-rebuild 先清空重建）
		imported, err := importToDB(*dbPath, all, *rebuild)
		if err != nil {
			fmt.Fprintf(os.Stderr, "入库失败: %v\n", err)
			os.Exit(1)
		}
		if *rebuild {
			fmt.Printf("✅ 重建 %d 条（原子替换）→ %s\n", imported, *dbPath)
		} else {
			fmt.Printf("✅ 入库 %d 条（跳过重复）→ %s\n", imported, *dbPath)
		}
		return
	}
	data, _ := json.MarshalIndent(all, "", "  ")
	if err := os.WriteFile(*out, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写输出失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 共 %d 条样本 → %s\n", len(all), *out)
}

// importToDB 直接写入本地 SQLite（经 database 包，自动建表；rebuild 时先清空）
func importToDB(dbPath string, samples []Sample, rebuild bool) (int, error) {
	db, err := database.Open(context.Background(), dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	store := database.NewStore(db)
	ctx := context.Background()
	if rebuild {
		// 原子重建：事务内删除本次 source_file 的行 + 插入（失败不丢旧数据）
		var items []database.StyleSample
		for _, s := range samples {
			cat := s.Category
			if s.Style != "" {
				cat = cat + "｜" + s.Style
			}
			if len([]rune(cat)) > 60 {
				cat = string([]rune(cat)[:60])
			}
			items = append(items, database.StyleSample{
				Title:      s.Title,
				Author:     s.Author,
				Category:   cat,
				SourceFile: s.SourceFile,
				Content:    s.Content,
			})
		}
		return store.ReplaceStyleSamples(ctx, items)
	}
	var items []database.StyleSample
	for _, s := range samples {
		cat := s.Category
		if s.Style != "" {
			cat = cat + "｜" + s.Style
		}
		if len([]rune(cat)) > 60 {
			cat = string([]rune(cat)[:60])
		}
		items = append(items, database.StyleSample{
			Title:      s.Title,
			Author:     s.Author,
			Category:   cat,
			SourceFile: s.SourceFile,
			Content:    s.Content,
		})
	}
	return store.ImportStyleSamples(context.Background(), items)
}
