package api

import "net/http"

func (s *Server) HandleAppearanceStatus(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	writeOK(w, s.appearance.Status())
}

func (s *Server) HandleAppearanceDownload(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	results := s.appearance.DownloadAll()
	hasError := false
	for _, rs := range results {
		if rs.Error != "" {
			hasError = true
			break
		}
	}
	if hasError {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "partial",
			"results": results,
		})
		return
	}
	writeOK(w, map[string]interface{}{
		"status":  "ok",
		"results": results,
	})
}

func (s *Server) HandleGetBackgrounds(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	customBackgrounds, err := s.appearance.ListCustom()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	library := s.appearance.GetLibrary()
	current := s.appearance.GetCurrentState()
	writeOK(w, map[string]interface{}{
		"library":  library,
		"custom":   customBackgrounds,
		"current":  current,
	})
}

func (s *Server) HandleSetBackground(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	var req struct {
		Type       string `json:"type"`
		Theme      string `json:"theme"`
		File       string `json:"file"`
		SiderBg    string `json:"sider_bg"`
		SiderLight string `json:"sider_light_bg"`
		EmptyBg    string `json:"empty_bg"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	setOne := func(bgType, theme, file string) error {
		if file != "" {
			return s.appearance.SetBackground(bgType, theme, file)
		}
		return nil
	}

	var firstErr error

	if req.SiderBg != "" || req.SiderLight != "" || req.EmptyBg != "" {
		if err := setOne("sider", "dark", req.SiderBg); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := setOne("sider", "light", req.SiderLight); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := setOne("empty", "light", req.EmptyBg); err != nil && firstErr == nil {
			firstErr = err
		}
	} else {
		if req.Type == "" {
			writeError(w, http.StatusBadRequest, "未指定背景类型或文件名")
			return
		}
		if req.Theme == "" {
			req.Theme = "dark"
		}
		if err := s.appearance.SetBackground(req.Type, req.Theme, req.File); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if firstErr != nil {
		writeError(w, http.StatusBadRequest, firstErr.Error())
		return
	}
	writeOK(w, map[string]string{"status": "ok"})
}

func (s *Server) HandleUploadBackground(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "上传解析失败: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少文件字段")
		return
	}
	defer file.Close()
	bgType := r.FormValue("type")
	if bgType == "" {
		bgType = "sider"
	}
	theme := r.FormValue("theme")
	if theme == "" {
		theme = "dark"
	}
	result, err := s.appearance.UploadBackground(bgType, theme, file, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, result)
}

func (s *Server) HandleGenerateBackground(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	var req struct {
		Type  string `json:"type"`
		Theme string `json:"theme"`
		Prompt string `json:"prompt"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	if req.Type == "" {
		req.Type = "sider"
	}
	if req.Theme == "" {
		req.Theme = "dark"
	}
	result, err := s.appearance.GenerateFromPicsum(req.Type, req.Theme, req.Prompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成失败: "+err.Error())
		return
	}
	writeOK(w, result)
}

func (s *Server) HandleRandomBackground(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	bgType := r.URL.Query().Get("type")
	if bgType == "" {
		bgType = "sider"
	}
	theme := r.URL.Query().Get("theme")
	if theme == "" {
		theme = "dark"
	}
	result, err := s.appearance.RandomBackground(bgType, theme)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "随机选择失败: "+err.Error())
		return
	}
	writeOK(w, result)
}

func (s *Server) HandleResetDefault(w http.ResponseWriter, r *http.Request) {
	if s.appearance == nil {
		writeError(w, http.StatusNotFound, "外观服务未启用")
		return
	}
	if err := s.appearance.ResetToDefault(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "ok"})
}
