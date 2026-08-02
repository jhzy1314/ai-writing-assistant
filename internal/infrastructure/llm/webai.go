package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebAIProvider 网页AI提供商配置
type WebAIProvider struct {
	Name        string // 提供商名称
	BaseURL     string // 基础URL
	Headers     func(cookie, token string) map[string]string // 请求头构造函数
	BuildBody   func(systemPrompt, userPrompt string) interface{} // 请求体构造函数
	ParseResponse func(resp *http.Response) (string, error) // 响应解析函数
	CustomCall  func(ctx context.Context, adapter *WebAIAdapter, systemPrompt, userPrompt string) (string, error) // 自定义调用（如 kimi 两步流程）
}

// WebAIProviders 预设的网页AI提供商
var WebAIProviders = map[string]*WebAIProvider{
	"kimi-free": {
		Name:    "Kimi免费版",
		BaseURL: "https://kimi.moonshot.cn/api/chat",
		Headers: func(cookie, token string) map[string]string {
			h := map[string]string{
				"Content-Type":  "application/json",
				"Accept":        "text/event-stream",
			}
			if cookie != "" {
				h["Cookie"] = cookie
			}
			if token != "" {
				h["Authorization"] = "Bearer " + token
			}
			return h
		},
		BuildBody: func(systemPrompt, userPrompt string) interface{} {
			return map[string]interface{}{
				"messages": []map[string]string{
					{"role": "system", "content": systemPrompt},
					{"role": "user", "content": userPrompt},
				},
				"stream": true,
			}
		},
		ParseResponse: parseSSEResponse,
		// kimi 网页版需要两步：先创建会话拿 chat_id，再向会话发消息（SSE 流）
		CustomCall: callKimiFree,
	},
	"doubao-free": {
		Name:    "豆包免费版",
		BaseURL: "https://www.doubao.com/chat/api",
		Headers: func(cookie, token string) map[string]string {
			h := map[string]string{
				"Content-Type": "application/json",
			}
			if cookie != "" {
				h["Cookie"] = cookie
			}
			if token != "" {
				h["Authorization"] = "Bearer " + token
			}
			return h
		},
		BuildBody: func(systemPrompt, userPrompt string) interface{} {
			return map[string]interface{}{
				"messages": []map[string]string{
					{"role": "system", "content": systemPrompt},
					{"role": "user", "content": userPrompt},
				},
				"stream": true,
			}
		},
		ParseResponse: parseSSEResponse,
	},
	"qwen-free": {
		Name:    "通义千问免费版",
		BaseURL: "https://tongyi.aliyun.com/api/chat",
		Headers: func(cookie, token string) map[string]string {
			h := map[string]string{
				"Content-Type": "application/json",
			}
			if cookie != "" {
				h["Cookie"] = cookie
			}
			if token != "" {
				h["Authorization"] = "Bearer " + token
			}
			return h
		},
		BuildBody: func(systemPrompt, userPrompt string) interface{} {
			return map[string]interface{}{
				"messages": []map[string]string{
					{"role": "system", "content": systemPrompt},
					{"role": "user", "content": userPrompt},
				},
				"stream": true,
			}
		},
		ParseResponse: parseSSEResponse,
	},
	"deepseek-free": {
		Name:    "DeepSeek免费版",
		BaseURL: "https://chat.deepseek.com/api/v0/chat/completions",
		Headers: func(cookie, token string) map[string]string {
			h := map[string]string{
				"Content-Type": "application/json",
			}
			if cookie != "" {
				h["Cookie"] = cookie
			}
			if token != "" {
				h["Authorization"] = "Bearer " + token
			}
			return h
		},
		BuildBody: func(systemPrompt, userPrompt string) interface{} {
			return map[string]interface{}{
				"messages": []map[string]string{
					{"role": "system", "content": systemPrompt},
					{"role": "user", "content": userPrompt},
				},
				"stream": true,
			}
		},
		ParseResponse: parseSSEResponse,
	},
	"zhipu-free": {
		Name:    "智谱清言免费版",
		BaseURL: "https://chatglm.cn/api/chat",
		Headers: func(cookie, token string) map[string]string {
			h := map[string]string{
				"Content-Type": "application/json",
			}
			if cookie != "" {
				h["Cookie"] = cookie
			}
			if token != "" {
				h["Authorization"] = "Bearer " + token
			}
			return h
		},
		BuildBody: func(systemPrompt, userPrompt string) interface{} {
			return map[string]interface{}{
				"messages": []map[string]string{
					{"role": "system", "content": systemPrompt},
					{"role": "user", "content": userPrompt},
				},
				"stream": true,
			}
		},
		ParseResponse: parseSSEResponse,
	},
}

// callKimiFree 调用 Kimi 网页版：先创建会话拿 chat_id，再向会话发消息读取 SSE 流
func callKimiFree(ctx context.Context, a *WebAIAdapter, systemPrompt, userPrompt string) (string, error) {
	base := "https://kimi.moonshot.cn"
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "text/event-stream",
		"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		"Origin":       base,
		"Referer":      base + "/",
	}
	if a.cookie != "" {
		headers["Cookie"] = a.cookie
	}
	if a.sessionToken != "" {
		headers["Authorization"] = "Bearer " + a.sessionToken
	}

	// 1) 创建会话
	createBody := map[string]interface{}{
		"Name":     "new-chat",
		"messages": []map[string]string{{"role": "user", "content": userPrompt}},
		"stream":   false,
	}
	cb, _ := json.Marshal(createBody)
	createReq, err := http.NewRequestWithContext(ctx, "POST", base+"/api/chat", bytes.NewReader(cb))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range headers {
		createReq.Header.Set(k, v)
	}
	createResp, err := a.client.Do(createReq)
	if err != nil {
		return "", fmt.Errorf("创建会话失败: %w", err)
	}
	createData, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("创建会话失败，状态码: %d, 响应: %s", createResp.StatusCode, string(createData))
	}
	var chatObj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createData, &chatObj); err != nil || chatObj.ID == "" {
		return "", fmt.Errorf("解析会话ID失败: %s", string(createData))
	}

	// 2) 发送消息（SSE 流）
	// Kimi 网页版不支持独立 system 消息（会导致回复为空），合并进 user
	fullPrompt := userPrompt
	if systemPrompt != "" {
		fullPrompt = systemPrompt + "\n\n" + userPrompt
	}
	msgBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": fullPrompt},
		},
		"kimiplus_id": "kimi",
		"use_search":  false,
		"stream":      true,
		"is_learning": false,
	}
	mb, _ := json.Marshal(msgBody)
	msgReq, err := http.NewRequestWithContext(ctx, "POST", base+"/api/chat/"+chatObj.ID+"/completion/stream", bytes.NewReader(mb))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range headers {
		msgReq.Header.Set(k, v)
	}
	msgResp, err := a.client.Do(msgReq)
	if err != nil {
		return "", fmt.Errorf("发送消息失败: %w", err)
	}
	defer msgResp.Body.Close()
	if msgResp.StatusCode != http.StatusOK {
		bd, _ := io.ReadAll(msgResp.Body)
		return "", fmt.Errorf("发送消息失败，状态码: %d, 响应: %s", msgResp.StatusCode, string(bd))
	}
	return parseKimiSSE(msgResp.Body)
}

// parseKimiSSE 解析 Kimi SSE 流，提取 event=cmpl 的 text 字段
func parseKimiSSE(body io.Reader) (string, error) {
	var result strings.Builder
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var evt struct {
			Event string `json:"event"`
			Text  string `json:"text"`
		}
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}
		if evt.Event == "cmpl" && evt.Text != "" {
			result.WriteString(evt.Text)
		}
		if evt.Event == "done" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return result.String(), err
	}
	return result.String(), nil
}

// parseSSEResponse 解析SSE流式响应
func parseSSEResponse(resp *http.Response) (string, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	var result strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk map[string]interface{}
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		// 尝试从不同的响应格式中提取文本
		if choices, ok := chunk["choices"].([]interface{}); ok {
			for _, choice := range choices {
				if c, ok := choice.(map[string]interface{}); ok {
					if delta, ok := c["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok {
							result.WriteString(content)
						}
					}
				}
			}
		}
		if data, ok := chunk["data"].(string); ok {
			result.WriteString(data)
		}
	}
	return result.String(), nil
}

// WebAIAdapter 网页AI适配器
type WebAIAdapter struct {
	name         string
	provider     string
	cookie       string
	sessionToken string
	requestURL   string
	maxTokens    int
	timeout      time.Duration
	client       *http.Client
}

// NewWebAIAdapter 构造网页AI适配器
func NewWebAIAdapter(name, provider, cookie, sessionToken, requestURL string, maxTokens, timeoutSeconds int) (*WebAIAdapter, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	if maxTokens <= 0 {
		maxTokens = 4000
	}
	return &WebAIAdapter{
		name:         name,
		provider:     provider,
		cookie:       cookie,
		sessionToken: sessionToken,
		requestURL:   requestURL,
		maxTokens:    maxTokens,
		timeout:      time.Duration(timeoutSeconds) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}, nil
}

func (a *WebAIAdapter) Name() string     { return a.name }
func (a *WebAIAdapter) Vendor() string   { return a.provider }
func (a *WebAIAdapter) Endpoint() string { return a.requestURL }
// SetThinking 网页AI无思考开关，空实现满足接口
func (a *WebAIAdapter) SetThinking(enabled bool) {}

// Generate 非流式生成
func (a *WebAIAdapter) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, Usage, error) {
	text, err := a.callWebAI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", Usage{}, fmt.Errorf("网页AI %s 调用失败: %w", a.name, err)
	}
	if text == "" {
		return "", Usage{}, fmt.Errorf("网页AI %s 返回空响应", a.name)
	}
	usage := Usage{
		PromptTokens:     EstimateTokens(systemPrompt + userPrompt),
		CompletionTokens: EstimateTokens(text),
	}
	return text, usage, nil
}

// Stream 流式生成
func (a *WebAIAdapter) Stream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error) {
	out := make(chan StreamChunk, 16)
	go func() {
		defer close(out)
		text, err := a.callWebAI(ctx, systemPrompt, userPrompt)
		if err != nil {
			out <- StreamChunk{Err: err}
			return
		}
		// 模拟流式输出，分块发送
		runes := []rune(text)
		chunkSize := 10
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			select {
			case <-ctx.Done():
				out <- StreamChunk{Err: ctx.Err()}
				return
			default:
				out <- StreamChunk{Text: string(runes[i:end])}
			}
		}
		// 发送用量信息
		out <- StreamChunk{
			Usage: &Usage{
				PromptTokens:     EstimateTokens(systemPrompt + userPrompt),
				CompletionTokens: EstimateTokens(text),
			},
			Done: true,
		}
	}()
	return out, nil
}

// callWebAI 调用网页AI接口
func (a *WebAIAdapter) callWebAI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	provider, ok := WebAIProviders[a.provider]
	if !ok {
		// 尝试使用自定义URL
		return a.callCustomWebAI(ctx, systemPrompt, userPrompt)
	}

	// 自定义调用（kimi 等需要先建会话再发消息）
	if provider.CustomCall != nil {
		return provider.CustomCall(ctx, a, systemPrompt, userPrompt)
	}

	url := a.requestURL
	if url == "" {
		url = provider.BaseURL
	}

	headers := provider.Headers(a.cookie, a.sessionToken)
	body := provider.BuildBody(systemPrompt, userPrompt)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("构造请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	return provider.ParseResponse(resp)
}

// callCustomWebAI 调用自定义网页AI
func (a *WebAIAdapter) callCustomWebAI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if a.cookie != "" {
		headers["Cookie"] = a.cookie
	}
	if a.sessionToken != "" {
		headers["Authorization"] = "Bearer " + a.sessionToken
	}

	body := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream": false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("构造请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.requestURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 尝试从不同的响应格式中提取文本
	if choices, ok := result["choices"].([]interface{}); ok {
		for _, choice := range choices {
			if c, ok := choice.(map[string]interface{}); ok {
				if message, ok := c["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok {
						return content, nil
					}
				}
			}
		}
	}
	if content, ok := result["content"].(string); ok {
		return content, nil
	}
	if data, ok := result["data"].(string); ok {
		return data, nil
	}

	return "", fmt.Errorf("无法解析响应内容")
}

// TestConnection 测试连接
func (a *WebAIAdapter) TestConnection(ctx context.Context) error {
	_, _, err := a.Generate(ctx, "你是一个助手。", "回复OK")
	return err
}

// GetProvider 获取提供商配置
func GetProvider(name string) (*WebAIProvider, bool) {
	p, ok := WebAIProviders[name]
	return p, ok
}

// ListProviders 列出所有提供商
func ListProviders() map[string]*WebAIProvider {
	return WebAIProviders
}
