package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// keyCookieWants 是判断一条 cookie 是否"复制全了"要看的关键项。
//
// 抓包实测浏览器每次发往 gemini.google.com 的有 30 个 cookie，其中 16 个是
// 每请求必带的登录态项。这里只挑最能反映"复制全没全"的几个来展示，不是说
// 别的不重要 —— 我们把整串原样转发，少一个就是少发一个。
var keyCookieWants = []string{"SID", "HSID", "SSID", "APISID", "SAPISID",
	"__Secure-1PSID", "__Secure-1PSIDTS", "__Secure-1PAPISID", "__Secure-1PSIDCC"}

// cookieAcctView 是账号的脱敏视图：**不含完整 cookie 值**，只回状态摘要。
// 凭证没必要从服务端再发回浏览器一次（跟旧 handleAdminCookie 同原则）。
func cookieAcctView(a CookieAccount) map[string]interface{} {
	names := cookieNames(a.Cookie)
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	var key []string
	for _, w := range keyCookieWants {
		if nameSet[w] {
			key = append(key, w)
		}
	}
	tail := ""
	if s := extractSAPISID(a.Cookie); len(s) >= 4 {
		tail = s[len(s)-4:]
	}
	// health 是给面板标红用的派生健康度（#18）。判据用 fail_count：markAccountResult
	// 成功会把它清零、失败才 +1，而且只有 401/403 才算失败（见 markCookieByStatus），
	// 所以 fail_count>0 就是「当前鉴权在失败」= 多半 cookie 过期了，直接标红提示重导。
	//   dead     鉴权失败中（红）—— 不管启用还是被自动停用，只要 fail_count>0
	//   disabled 用户手动停用且没有失败记录（灰）
	//   unknown  还没成功用过/检测过（待检测）
	//   ok       在用、没失败（绿）
	health := "ok"
	switch {
	case a.FailCount > 0:
		health = "dead"
	case a.Status != "enabled":
		health = "disabled"
	case a.LastOkAt == 0:
		health = "unknown"
	}

	var reqTotal, reqSuccess, reqTokens int64
	_ = getDB().QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN status=200 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(prompt_tokens + output_tokens), 0)
		FROM requests WHERE account_id=? OR (account_label=? AND account_label != '')`, a.ID, a.Label).Scan(&reqTotal, &reqSuccess, &reqTokens)

	return map[string]interface{}{
		"id":           a.ID,
		"label":        a.Label,
		"status":       a.Status,
		"health":       health,
		"note":         a.Note,
		"created_at":   a.CreatedAt,
		"last_used_at": a.LastUsedAt,
		"last_ok_at":   a.LastOkAt,
		"last_error":   a.LastError,
		"fail_count":   a.FailCount,
		"cookie_count": len(names),
		"key_cookies":  key,
		"sapisid_tail": tail,
		"has_1psidts":  nameSet["__Secure-1PSIDTS"],
		"proxy_id":     a.ProxyID,
		"proxy_name":   proxyNameByID(a.ProxyID),
		"quota":        getAccountQuotaCached(a.ID),
		"req_total":    reqTotal,
		"req_success":  reqSuccess,
		"req_tokens":   reqTokens,
	}
}

// handleAdminCookies — GET 列出池 / POST 新增一条。
func handleAdminCookies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		accts := accountList()
		items := make([]map[string]interface{}, 0, len(accts))
		for _, a := range accts {
			items = append(items, cookieAcctView(a))
		}
		total, enabled := accountCount()
		writeJSON(w, 200, map[string]interface{}{
			"items": items, "total": total, "enabled": enabled,
		})
	case http.MethodPost:
		var p struct {
			Label  string `json:"label"`
			Cookie string `json:"cookie"`
			Note   string `json:"note"`
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &p); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad json"})
			return
		}
		id, err := accountAdd(p.Label, p.Cookie, p.Note)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		logf("[cookies] 新增账号 #%d label=%q", id, strings.TrimSpace(p.Label))
		writeJSON(w, 200, map[string]interface{}{"id": id})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// handleAdminCookieItem — /admin/api/cookies/{id}[/toggle]
//
//	DELETE          删除
//	POST .../toggle 翻转 enabled/disabled
//	PATCH           改 label / note / status
func handleAdminCookieItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/admin/api/cookies/")
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || parts[0] == "" {
		writeJSON(w, 404, map[string]string{"error": "missing id"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad id"})
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch r.Method {
	case http.MethodDelete:
		if err := accountDelete(id); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		logf("[cookies] 删除账号 #%d", id)
		writeJSON(w, 200, map[string]bool{"ok": true})
	case http.MethodPost:
		if action == "rotate" {
			for _, a := range accountList() {
				if a.ID == id {
					iv, names, err := rotateAccount(a)
					if err != nil {
						writeJSON(w, 200, map[string]interface{}{"ok": false, "detail": err.Error()})
						return
					}
					detail := "保活成功"
					if len(names) > 0 {
						detail = "保活成功，刷新了 " + strings.Join(names, ", ")
					}
					writeJSON(w, 200, map[string]interface{}{
						"ok": true, "detail": detail, "refreshed": names, "next_sec": int(iv.Seconds())})
					return
				}
			}
			writeJSON(w, 404, map[string]string{"error": "account not found"})
			return
		}
		if action == "check" {
			for _, a := range accountList() {
				if a.ID == id {
					writeJSON(w, 200, checkAccountCookie(a))
					return
				}
			}
			writeJSON(w, 404, map[string]string{"error": "account not found"})
			return
		}
		if action == "quota" {
			for _, a := range accountList() {
				if a.ID == id {
					q, err := refreshAccountQuota(a)
					if err != nil {
						writeJSON(w, 200, map[string]interface{}{"ok": false, "error": err.Error(), "quota": q})
						return
					}
					writeJSON(w, 200, map[string]interface{}{"ok": true, "quota": q})
					return
				}
			}
			writeJSON(w, 404, map[string]string{"error": "account not found"})
			return
		}
		if action != "toggle" {
			writeJSON(w, 400, map[string]string{"error": "unknown action"})
			return
		}
		cur := ""
		for _, a := range accountList() {
			if a.ID == id {
				cur = a.Status
				break
			}
		}
		next := "enabled"
		if cur == "enabled" {
			next = "disabled"
		}
		if err := accountSetStatus(id, next); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"status": next})
	case http.MethodPatch:
		var p struct {
			Label   *string `json:"label"`
			Note    *string `json:"note"`
			Status  *string `json:"status"`
			ProxyID *int64  `json:"proxy_id"`
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &p); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad json"})
			return
		}
		if p.Status != nil {
			if err := accountSetStatus(id, *p.Status); err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
		}
		if p.ProxyID != nil {
			bindAccountProxy(id, *p.ProxyID)
		}
		if p.Label != nil || p.Note != nil {
			// 只改传了的字段：没传的沿用当前值。
			var curLabel, curNote string
			for _, a := range accountList() {
				if a.ID == id {
					curLabel, curNote = a.Label, a.Note
					break
				}
			}
			if p.Label != nil {
				curLabel = *p.Label
			}
			if p.Note != nil {
				curNote = *p.Note
			}
			if err := accountUpdateMeta(id, curLabel, curNote); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// handleAdminCookiesSync — POST /admin/api/cookies/sync 供浏览器扩展一键同步 Cookie
func handleAdminCookiesSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var p struct {
		Label  string `json:"label"`
		Cookie string `json:"cookie"`
		Note   string `json:"note"`
	}
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json: " + err.Error()})
		return
	}
	if strings.TrimSpace(p.Cookie) == "" {
		writeJSON(w, 400, map[string]string{"error": "cookie 不能为空"})
		return
	}

	id, updated, err := accountUpsert(p.Label, p.Cookie, p.Note)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	action := "created"
	if updated {
		action = "updated"
		logf("[cookies] 浏览器扩展同步：已更新现有账号 #%d (label=%q)", id, strings.TrimSpace(p.Label))
	} else {
		logf("[cookies] 浏览器扩展同步：成功新增账号 #%d (label=%q)", id, strings.TrimSpace(p.Label))
	}

	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"id":      id,
		"action":  action,
		"label":   p.Label,
		"message": fmt.Sprintf("账号 #%d %s 成功", id, map[bool]string{true: "更新", false: "添加"}[updated]),
	})
}

// handleAdminClearConversations — POST /admin/api/conversations/clear
func handleAdminClearConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	res, err := ClearAllAccountsConversations()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	total := 0
	for _, n := range res {
		total += n
	}
	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"total":   total,
		"details": res,
		"message": fmt.Sprintf("已成功清空全部账号的历史会话（共删除 %d 条）", total),
	})
}

// handleAdminQuotaRefreshAll — POST /admin/api/cookies/quota/refresh 批量刷新所有启用账号的用量
func handleAdminQuotaRefreshAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	accts := accountList()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	var mu sync.Mutex
	results := make([]AccountQuota, 0, len(accts))

	for _, a := range accts {
		if a.Status != "enabled" {
			continue
		}
		wg.Add(1)
		go func(acc CookieAccount) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			q, _ := refreshAccountQuota(acc)
			if q != nil {
				mu.Lock()
				results = append(results, *q)
				mu.Unlock()
			}
		}(a)
	}
	wg.Wait()
	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"count":   len(results),
		"results": results,
		"message": fmt.Sprintf("已成功刷新 %d 个账号的用量限制", len(results)),
	})
}

// handleAdminCookiesCheckAll — POST /admin/api/cookies/check-all 批量检测所有启用账号
func handleAdminCookiesCheckAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	accts := accountList()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	var mu sync.Mutex

	total := 0
	passed := 0
	failed := 0
	type checkRes struct {
		ID        int64  `json:"id"`
		Label     string `json:"label"`
		OK        bool   `json:"ok"`
		Detail    string `json:"detail"`
		TookMs    int64  `json:"took_ms"`
		ProxyName string `json:"proxy_name"`
	}
	results := make([]checkRes, 0, len(accts))

	for _, a := range accts {
		if a.Status != "enabled" {
			continue
		}
		total++
		wg.Add(1)
		go func(acc CookieAccount) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			chk := checkAccountCookie(acc)
			mu.Lock()
			if chk.OK {
				passed++
			} else {
				failed++
			}
			results = append(results, checkRes{
				ID:        acc.ID,
				Label:     acc.Label,
				OK:        chk.OK,
				Detail:    chk.Detail,
				TookMs:    chk.TookMs,
				ProxyName: chk.ProxyName,
			})
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"total":   total,
		"passed":  passed,
		"failed":  failed,
		"results": results,
		"message": fmt.Sprintf("检测完成：共 %d 个账号，%d 个正常，%d 个异常", total, passed, failed),
	})
}

// handleAdminCookiesClearAll — POST /admin/api/cookies/clear-all 一键清空 Cookie 池
func handleAdminCookiesClearAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	res, err := getDB().Exec(`DELETE FROM accounts`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	rowsAffected, _ := res.RowsAffected()
	logf("[cookies] 已一键清空 Cookie 池，共删除 %d 条账号记录", rowsAffected)
	writeJSON(w, 200, map[string]interface{}{
		"ok":      true,
		"deleted": rowsAffected,
		"message": fmt.Sprintf("已成功清空 Cookie 池（共删除 %d 个账号）", rowsAffected),
	})
}


