package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	modelsMu sync.RWMutex
)

// OfficialModelItem 包含从 Google otAQ7b 返回的权威模型项
type OfficialModelItem struct {
	HexID    string `json:"hex_id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Desc     string `json:"desc"`
	Mode     int    `json:"mode"`
	Locked   bool   `json:"locked"`
}

// sanitizeModelName 把 Google 官方返回的名称格式化成标准的 gemini-* 格式
func sanitizeModelName(fullName, name string) string {
	s := fullName
	if s == "" {
		s = name
	}
	s = strings.ToLower(strings.TrimSpace(s))
	// 替换空格为中划线：例如 "4 argon" -> "gemini-4-argon", "3.8 flash" -> "gemini-3.8-flash"
	s = strings.ReplaceAll(s, " ", "-")
	if !strings.HasPrefix(s, "gemini-") {
		s = "gemini-" + s
	}
	return s
}

// FetchOfficialModelsFromGoogle 向 gemini.google.com 的 otAQ7b RPC 实时请求官方最新模型列表
func FetchOfficialModelsFromGoogle() ([]OfficialModelItem, error) {
	// 挑一个启用的账号作为探针
	accts := accountList()
	var acct *CookieAccount
	for _, a := range accts {
		if a.Status == "enabled" && a.Cookie != "" {
			acct = &a
			break
		}
	}
	if acct == nil {
		return nil, fmt.Errorf("cookie 池中无可用账号，无法向 Google 探针官方模型")
	}

	cookie := acct.Cookie
	sapisid := extractSAPISID(cookie)
	if sapisid == "" {
		return nil, fmt.Errorf("账号 #%d 缺少 SAPISID", acct.ID)
	}

	proxyURL := proxyURLByID(acct.ProxyID)

	xsrf, err := getXSRF(cookie, proxyURL)
	if err != nil {
		return nil, fmt.Errorf("获取 XSRF 失败: %w", err)
	}

	freq, _ := json.Marshal([]interface{}{[]interface{}{[]interface{}{"otAQ7b", "[]", nil, "generic"}}})
	form := url.Values{}
	form.Set("f.req", string(freq))
	if xsrf != "" {
		form.Set("at", xsrf)
	}

	reqid := time.Now().UnixNano() % 1000000
	endpoint := fmt.Sprintf(
		"https://gemini.google.com/_/BardChatUi/data/batchexecute?rpcids=otAQ7b&bl=%s&hl=zh-TW&_reqid=%d&rt=c",
		currentBL(proxyURL), reqid,
	)

	headers := buildGeminiHeaders(cookie, sapisid, "")
	delete(headers, "x-goog-ext-525001261-jspb")

	status, _, body, err := uploadPost(endpoint, headers, []byte(form.Encode()), proxyURL)
	if err != nil {
		return nil, fmt.Errorf("请求 otAQ7b 失败: %w", err)
	}
	if status != 200 {
		return nil, fmt.Errorf("otAQ7b 返回 HTTP %d", status)
	}

	return parseOTAQ7bResponse(body)
}

// parseOTAQ7bResponse 解析 otAQ7b 原始响应并提取模型项
func parseOTAQ7bResponse(body []byte) ([]OfficialModelItem, error) {
	str := string(body)
	var targetInner string
	for _, line := range strings.Split(str, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `[["wrb.fr"`) {
			var frames [][]interface{}
			if err := json.Unmarshal([]byte(line), &frames); err == nil && len(frames) > 0 {
				if len(frames[0]) > 2 {
					if innerStr, ok := frames[0][2].(string); ok {
						targetInner = innerStr
						break
					}
				}
			}
		}
	}

	if targetInner == "" {
		return nil, fmt.Errorf("otAQ7b 响应中未找到模型数据帧 (raw: %d bytes)", len(body))
	}

	var innerData []interface{}
	if err := json.Unmarshal([]byte(targetInner), &innerData); err != nil {
		return nil, fmt.Errorf("解析 innerData 失败: %w", err)
	}

	if len(innerData) <= 15 {
		return nil, fmt.Errorf("innerData 长度不足 16，未包含模型清单")
	}

	modelsList, ok := innerData[15].([]interface{})
	if !ok {
		return nil, fmt.Errorf("innerData[15] 不是数组格式")
	}

	var items []OfficialModelItem
	for _, m := range modelsList {
		arr, ok := m.([]interface{})
		if !ok || len(arr) < 2 {
			continue
		}

		hexID, _ := arr[0].(string)
		name, _ := arr[1].(string)
		desc := ""
		if len(arr) > 2 {
			desc, _ = arr[2].(string)
		}
		mode := 1
		if len(arr) > 4 {
			if f, ok := arr[4].(float64); ok {
				mode = int(f)
			}
		}
		locked := false
		if len(arr) > 7 {
			locked, _ = arr[7].(bool)
		}
		fullName := ""
		if len(arr) > 11 {
			fullName, _ = arr[11].(string)
		}

		if hexID != "" && name != "" {
			items = append(items, OfficialModelItem{
				HexID:    hexID,
				Name:     name,
				FullName: fullName,
				Desc:     desc,
				Mode:     mode,
				Locked:   locked,
			})
		}
	}

	return items, nil
}

// SyncOfficialModels 从 Google 实时同步官方模型列表，并更新系统 Models 映射
func SyncOfficialModels() ([]string, error) {
	items, err := FetchOfficialModelsFromGoogle()
	if err != nil {
		return nil, err
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()

	// 保存媒体工具模型
	mediaTools := map[string]ModelConfig{
		"gemini-image":  Models["gemini-image"],
		"gemini-music":  Models["gemini-music"],
		"gemini-video":  Models["gemini-video"],
		"gemini-canvas": Models["gemini-canvas"],
	}

	newModels := make(map[string]ModelConfig)
	var updatedNames []string

	for _, item := range items {
		baseID := sanitizeModelName(item.FullName, item.Name)
		desc := item.Desc
		if desc == "" {
			desc = item.FullName
		}

		// 基础普通模型
		newModels[baseID] = ModelConfig{
			HexID:    item.HexID,
			Mode:     item.Mode,
			Desc:     desc,
			Thinking: false,
		}
		updatedNames = append(updatedNames, baseID)

		// 扩展思考版
		thinkingID := baseID + "-thinking"
		newModels[thinkingID] = ModelConfig{
			HexID:    item.HexID,
			Mode:     item.Mode,
			Desc:     desc + " with extended thinking",
			Thinking: true,
		}
		updatedNames = append(updatedNames, thinkingID)
	}

	// 补充通用别名映射
	if v, ok := newModels["gemini-4-argon"]; ok {
		newModels["gemini-argon"] = v
		updatedNames = append(updatedNames, "gemini-argon")
	}
	if v, ok := newModels["gemini-4-argon-thinking"]; ok {
		newModels["gemini-argon-thinking"] = v
		updatedNames = append(updatedNames, "gemini-argon-thinking")
	}
	if v, ok := newModels["gemini-3.8-flash"]; ok {
		newModels["gemini-3.7-flash"] = v
		updatedNames = append(updatedNames, "gemini-3.7-flash")
	}
	if v, ok := newModels["gemini-3.8-flash-thinking"]; ok {
		newModels["gemini-3.7-flash-thinking"] = v
		updatedNames = append(updatedNames, "gemini-3.7-flash-thinking")
	}

	// 恢复媒体工具
	for k, v := range mediaTools {
		if v.HexID != "" {
			newModels[k] = v
			updatedNames = append(updatedNames, k)
		}
	}

	Models = newModels
	logf("[models] 官方模型清单已实时同步更新 (共 %d 个官方模型): %v", len(updatedNames), updatedNames)
	return updatedNames, nil
}

// startModelAutoSync 定期（每 1 小时）在后台探查 Google 是否更新了官方模型
func startModelAutoSync() {
	go func() {
		// 启动后等待 3 秒，首次自动同步一次
		time.Sleep(3 * time.Second)
		if _, err := SyncOfficialModels(); err != nil {
			logf("[models] 首次启动模型自动同步跳过: %v", err)
		}

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := SyncOfficialModels(); err != nil {
				logf("[models] 定期模型自动同步检测: %v", err)
			}
		}
	}()
}

// handleAdminModelsRefresh 后台一键刷新官方模型 API: POST /admin/api/models/refresh
func handleAdminModelsRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	names, err := SyncOfficialModels()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"models":  names,
		"count":   len(names),
		"message": fmt.Sprintf("已成功从 Google 实时同步官方最新模型清单 (共 %d 个)", len(names)),
	})
}
