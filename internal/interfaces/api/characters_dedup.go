package api

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// normCharName 规范化人物名：去空白/引号/括号等标点 + 去尾部重复段（"天一天一"→"天一"）
func normCharName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		switch r {
		case '"', '\'', '“', '”', '‘', '’', '（', '）', '(', ')', '《', '》', '【', '】', '[', ']', '：', ':', '，', ',', '。', '.', '！', '!', '？', '?':
			return -1
		}
		return r
	}, s)
	runes := []rune(s)
	for i := 1; i <= len(runes)/2; i++ {
		if string(runes[len(runes)-i:]) == string(runes[len(runes)-2*i:len(runes)-i]) {
			runes = runes[:len(runes)-i]
			break
		}
	}
	return string(runes)
}

// isDupCharName 别名变体判定：精确相等 / 字符重排相同 / 短名（≥2字）被长名包含
func isDupCharName(a, b string) bool {
	if a == "" || b == "" || a == b {
		return a != "" && b != "" && a == b
	}
	ra, rb := []rune(a), []rune(b)
	// 字符重排相同（叙述者我 vs 我叙述者）
	if len(ra) == len(rb) {
		sortRunes := func(rs []rune) string {
			cp := make([]rune, len(rs))
			copy(cp, rs)
			for i := range cp {
				for j := i + 1; j < len(cp); j++ {
					if cp[j] < cp[i] {
						cp[i], cp[j] = cp[j], cp[i]
					}
				}
			}
			return string(cp)
		}
		if sortRunes(ra) == sortRunes(rb) {
			return true
		}
	}
	// 短名（≥2字）被长名包含
	short, long := a, b
	if len(ra) > len(rb) {
		short, long = b, a
	}
	if len([]rune(short)) >= 2 && strings.Contains(long, short) {
		return true
	}
	return false
}

// HandleCharDuplicates GET /api/characters/duplicates?project_id=xxx
// 检测人物卡重复组（规范化 + 别名变体），每组返回建议保留项与重复项
func (s *Server) HandleCharDuplicates(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "project_id 不能为空")
		return
	}
	chars, err := s.store.ListCharacters(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type dupGroup struct {
		KeepID   string                `json:"keep_id"`
		KeepName string                `json:"keep_name"`
		Items    []database.Character  `json:"items"`
	}
	var groups []dupGroup
	used := map[string]bool{}
	for i := range chars {
		if used[chars[i].ID] {
			continue
		}
		ni := normCharName(chars[i].Name)
		if ni == "" {
			continue
		}
		grp := []database.Character{chars[i]}
		used[chars[i].ID] = true
		for j := range chars {
			if i == j || used[chars[j].ID] {
				continue
			}
			if isDupCharName(ni, normCharName(chars[j].Name)) {
				grp = append(grp, chars[j])
				used[chars[j].ID] = true
			}
		}
		if len(grp) > 1 {
			groups = append(groups, dupGroup{KeepID: grp[0].ID, KeepName: grp[0].Name, Items: grp})
		}
	}
	writeOK(w, map[string]interface{}{"groups": groups})
}

// HandleCharMerge POST /api/characters/merge
// {"project_id":"...","keep_id":"...","merge_ids":["..."]} → 把 merge_ids 的描述并入 keep，删除 merge
func (s *Server) HandleCharMerge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string   `json:"project_id"`
		KeepID    string   `json:"keep_id"`
		MergeIDs  []string `json:"merge_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.KeepID == "" || len(req.MergeIDs) == 0 {
		writeError(w, http.StatusBadRequest, "keep_id 与 merge_ids 不能为空")
		return
	}
	ctx := r.Context()
	chars, err := s.store.ListCharacters(ctx, req.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	byID := map[string]database.Character{}
	for _, c := range chars {
		byID[c.ID] = c
	}
	keep, ok := byID[req.KeepID]
	if !ok {
		writeError(w, http.StatusBadRequest, "保留的人物卡不存在")
		return
	}
	mergedDesc := keep.Description
	mergedCount := 0
	for _, mid := range req.MergeIDs {
		mm, ok := byID[mid]
		if !ok || mid == req.KeepID {
			continue
		}
		if strings.TrimSpace(mm.Description) != "" {
			mergedDesc += "\n【已合并 " + mm.Name + "】" + mm.Description
		}
		if err := s.store.DeleteCharacter(ctx, mid); err != nil {
			continue
		}
		mergedCount++
	}
	if mergedCount > 0 {
		desc := mergedDesc
		if _, err := s.store.UpdateCharacter(ctx, req.KeepID, nil, &desc, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "更新保留卡失败: "+err.Error())
			return
		}
	}
	writeOK(w, map[string]interface{}{"merged": mergedCount})
}
