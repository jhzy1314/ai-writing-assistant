package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/domain/pipeline"
	"github.com/ai-novel/studio/internal/domain/roles"
	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// HandleGenerate POST /api/generate —— SSE 流式返回执行阶段信息 + 最终文本
func (s *Server) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	var req pipeline.GenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.UserDemand) == "" && strings.TrimSpace(req.SelectedText) == "" {
		writeError(w, http.StatusBadRequest, "user_demand 与 selected_text 至少需提供一个")
		return
	}
	if req.RunMode == "" {
		req.RunMode = pipeline.ModeAuto
	}

	// SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "当前环境不支持流式输出")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	ip := clientIP(r)
	events := s.dispatcher.Run(ctx, req, ip)

	// 客户端断开时取消生成
	go func() {
		<-r.Context().Done()
		cancel()
	}()

	for ev := range events {
		data, _ := json.Marshal(ev)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// HandleVerify POST /api/verify —— 逻辑自检独立接口
// 请求体：{ "content", "world_setting", "character_setting" }
// 响应：问题清单 + 修改建议
func (s *Server) HandleVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content          string `json:"content"`
		WorldSetting     string `json:"world_setting"`
		CharacterSetting string `json:"character_setting"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		writeError(w, http.StatusBadRequest, "content 不能为空")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
	defer cancel()

	// 配额/限流/并发校验（与 /api/generate 一致）
	guard, err := s.limiter.AllowRequest(ctx, clientIP(r))
	if err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	defer guard.Release()

	agent := roles.NewRoleAgent(llm.RoleVerifier, string(pipeline.PipelineStandard))
	userPrompt := buildVerifyStandalonePrompt(body.Content, body.WorldSetting, body.CharacterSetting)

	adapters, err := s.registry.AdaptersForRole(ctx, llm.RoleVerifier)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "校验官无可用模型: "+err.Error())
		return
	}

	var lastErr error
	for _, ad := range adapters {
		start := time.Now()
		review, usage, gErr := agent.Generate(ctx, ad, userPrompt)
		durMs := time.Since(start).Milliseconds()
		if gErr == nil {
			// 成功：记录日志 + 累加用量
			_ = s.store.InsertLog(ctx, database.GenerationLog{
				Role: string(llm.RoleVerifier), ModelName: ad.Name(),
				PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
				DurationMs: int(durMs), Status: "ok",
			})
			_ = s.store.IncrUsage(ctx, ad.Name(), 1, usage.Total())
			passed := strings.Contains(review, "校验通过")
			resp := map[string]interface{}{
				"passed":      passed,
				"review":      review,
				"issues":      extractIssuesAPI(review),
				"suggestions": review,
				"model":       ad.Name(),
			}
			writeOK(w, resp)
			return
		}
		// 失败：记录日志，尝试下一备用模型
		_ = s.store.InsertLog(ctx, database.GenerationLog{
			Role: string(llm.RoleVerifier), ModelName: ad.Name(),
			DurationMs: int(durMs), Status: "error", ErrorMsg: gErr.Error(),
		})
		lastErr = gErr
	}
	writeError(w, http.StatusServiceUnavailable, "校验官全部模型调用失败，请稍后重试")
	_ = lastErr
}

func buildVerifyStandalonePrompt(content, world, character string) string {
	var b strings.Builder
	if strings.TrimSpace(world) != "" {
		b.WriteString("【世界观设定】\n")
		b.WriteString(world)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(character) != "" {
		b.WriteString("【人物卡】\n")
		b.WriteString(character)
		b.WriteString("\n\n")
	}
	b.WriteString("【待校验正文】\n")
	b.WriteString(content)
	b.WriteString("\n\n请逐项核查角色一致性、世界观冲突、剧情逻辑、文字质量，输出校验结果：")
	return b.String()
}

func extractIssuesAPI(review string) []string {
	lines := strings.Split(strings.TrimSpace(review), "\n")
	out := []string{}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.Contains(l, "校验通过") {
			continue
		}
		out = append(out, l)
	}
	if len(out) == 0 && strings.TrimSpace(review) != "" {
		out = append(out, review)
	}
	return out
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if i := strings.LastIndex(r.RemoteAddr, ":"); i > 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}
