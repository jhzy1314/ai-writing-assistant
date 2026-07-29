package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/domain/roles"
	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// HandleToolExecute POST /api/tools/execute
// 统一工具执行入口：接收 tool 名称 + 输入文本 + 可选参数，调用 Helper 角色执行并返回结果。
// 请求体：{ "tool": "clean|sort|extract|convert|count", "content": "...", "params": {} }
// 响应：  { "result": "...", "tool": "...", "model": "..." }
func (s *Server) HandleToolExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool    string `json:"tool"`
		Content string `json:"content"`
		Params  struct {
			From        string `json:"from"`
			To          string `json:"to"`
			Instruction string `json:"instruction"`
		} `json:"params"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content 不能为空")
		return
	}

	// 配额/限流校验
	guard, err := s.limiter.AllowRequest(r.Context(), clientIP(r))
	if err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	defer guard.Release()

	// 构建工具提示词
	userPrompt := buildToolPrompt(req.Tool, req.Content, req.Params.From, req.Params.To, req.Params.Instruction)
	if userPrompt == "" {
		writeError(w, http.StatusBadRequest, "不支持的工具类型: "+req.Tool)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
	defer cancel()

	// 调用 Helper 角色执行工具任务
	result, modelName, err := s.callHelperTool(ctx, userPrompt)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "工具执行失败: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"tool":   req.Tool,
		"result": result,
		"model":  modelName,
	})
}

// callHelperTool 调用 Helper 角色非流式生成，含备用模型降级
func (s *Server) callHelperTool(ctx context.Context, userPrompt string) (string, string, error) {
	agent := roles.NewRoleAgent(llm.RoleHelper, "light")
	adapters, err := s.registry.AdaptersForRole(ctx, llm.RoleHelper)
	if err != nil {
		return "", "", fmt.Errorf("Helper 无可用模型: %w", err)
	}
	var lastErr error
	for _, ad := range adapters {
		start := time.Now()
		text, usage, gErr := agent.Generate(ctx, ad, userPrompt)
		durMs := time.Since(start).Milliseconds()
		if gErr == nil {
			_ = s.store.InsertLog(ctx, database.GenerationLog{
				Role: string(llm.RoleHelper), ModelName: ad.Name(),
				PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
				DurationMs: int(durMs), Status: "ok",
			})
			_ = s.store.IncrUsage(ctx, ad.Name(), 1, usage.Total())
			return text, ad.Name(), nil
		}
		_ = s.store.InsertLog(ctx, database.GenerationLog{
			Role: string(llm.RoleHelper), ModelName: ad.Name(),
			DurationMs: int(durMs), Status: "error", ErrorMsg: gErr.Error(),
		})
		lastErr = gErr
	}
	return "", "", fmt.Errorf("Helper 全部模型调用失败: %w", lastErr)
}

// buildToolPrompt 按工具类型构建用户提示词
func buildToolPrompt(tool, content, from, to, instruction string) string {
	switch tool {
	case "clean":
		return fmt.Sprintf(`【工具：文本清洗】
请去除以下文本中的AI多余话术、代码围栏标记、解释性段落、多余开场白和结尾语，仅保留纯净正文。
若文本本身已是洁净正文，则原样返回。

待清洗文本：
%s`, content)

	case "convert":
		if from == "" {
			from = "markdown"
		}
		if to == "" {
			to = "纯文本"
		}
		return fmt.Sprintf(`【工具：格式转换】
将以下文本从 %s 格式转换为 %s 格式。
%s

待转换文本：
%s`, from, to, instruction, content)

	case "sort":
		return fmt.Sprintf(`【工具：章节排序】
以下是无序的章节列表，每章以"---"分隔，包含章节标题和正文片段。
请分析各章的剧情脉络、时间线、逻辑顺序，将章节按正确阅读顺序重新排列。
输出格式：
1. 排序后的章节标题序列（一行一个）
2. 简要排序理由

待排序章节：
%s`, content)

	case "extract":
		return fmt.Sprintf(`【工具：素材提取】
从以下文本中提取核心设定、人物信息、世界观要素、关键事件，输出结构化摘要。
%s

请按以下结构输出：
- 核心设定：
- 人物信息：
- 世界观要素：
- 关键事件：`, content)

	case "count":
		return fmt.Sprintf(`【工具：字数统计】
统计以下文本的精确字数（中文字符按1字计，英文单词按实际单词数计，标点不计入），只输出一个数字。

待统计文本：
%s`, content)

	case "summarize":
		return fmt.Sprintf(`你是一个小说设定提取助手。请仔细阅读下面的小说正文，从中提取所有能找到的人物信息和世界观信息。

最重要规则：不要用"未知"占位！如果正文中没有对应的信息，那一行就不要写。只写正文里确实有的内容。

小说正文：
---
%s
---

请按以下格式输出：

【人物卡列表】
姓名：<角色名>
<如果文中提到了性别，加一行>性别：<男/女>
<如果文中提到了外貌特征，加一行>外貌：<描述>
<如果文中提到了性格，加一行>性格：<描述>
<如果文中提到了身份背景，加一行>背景：<描述>
<如果有行为原则描写，加一行>行为底线：<描述>
<如果有与其他角色关系的描写，加一行>人际关系：<描述>
---
（下一个角色，用 --- 分隔）

【世界观设定】
标题：<从文中提取的世界观名称或直接写"当前设定">
<如果文中提到了时代>时代背景：<古代/现代/修仙时代等>
<如果有修炼体系描写>力量体系：<描述>
<如果有势力描写>势力分布：<描述>
<如果有地点描写>地理设定：<描述>
<如果有世界规则描写>世界规则：<描述>
---
（下一个世界设定，用 --- 分隔）

请务必从正文中提取真实信息。例如给出了"林风是青云门弟子"、"师父张玄"、"红发男子来攻打"等事实，就要如实地列出。`, content)

	default:
		return ""
	}
}
