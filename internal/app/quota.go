package app

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// AccountQuota 包含一个 Google 账号的配额/用量限制信息
type AccountQuota struct {
	AccountID       int64   `json:"account_id"`
	Tier            string  `json:"tier"`              // "PRO", "Free", "Unknown"
	FiveHourlyPct   float64 `json:"five_hourly_pct"`   // 5小时已用百分比 0.0 ~ 100.0
	FiveHourlyReset string  `json:"five_hourly_reset"` // 5小时重置时间 (ISO8601 或格式化文本)
	WeeklyPct       float64 `json:"weekly_pct"`        // 每周已用百分比 0.0 ~ 100.0
	WeeklyReset     string  `json:"weekly_reset"`      // 每周重置时间
	FetchedAt       int64   `json:"fetched_at"`        // Unix timestamp
	Error           string  `json:"error,omitempty"`
}

var (
	quotaMu    sync.RWMutex
	quotaCache = map[int64]AccountQuota{}
)

var preloadScriptRe = regexp.MustCompile(`(?s)<script\s+type=["']application/json["']\s+data-bg3-relay-preload[^>]*>(.*?)</script>`)

type relayPreloadData struct {
	Events []struct {
		ID    string `json:"id"`
		Kind  string `json:"kind"`
		Chunk string `json:"chunk"`
	} `json:"events"`
}

type relayChunkItem struct {
	Data *struct {
		Viewer *struct {
			Rpcs *struct {
				GetUsageInfo *struct {
					SubscriptionTier string `json:"subscriptionTier"`
					Pools            []struct {
						PoolType       string  `json:"poolType"`
						PercentageUsed float64 `json:"percentageUsed"`
						Gxu            *struct {
							WindowRefreshesAt string `json:"windowRefreshesAt"`
						} `json:"gxu"`
					} `json:"pools"`
				} `json:"getUsageInfo"`
			} `json:"rpcs"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// getAccountQuotaCached 返回缓存的配额信息；如果没有，尝试从 SQLite kv 恢复
func getAccountQuotaCached(accountID int64) *AccountQuota {
	quotaMu.RLock()
	q, ok := quotaCache[accountID]
	quotaMu.RUnlock()
	if ok {
		return &q
	}

	// 从 kv 恢复
	raw := kvGet("quota_acct_" + strconv.FormatInt(accountID, 10))
	if raw != "" {
		var sq AccountQuota
		if err := json.Unmarshal([]byte(raw), &sq); err == nil {
			quotaMu.Lock()
			quotaCache[accountID] = sq
			quotaMu.Unlock()
			return &sq
		}
	}
	return nil
}

// saveAccountQuotaToCache 写入缓存与持久化 kv
func saveAccountQuotaToCache(q AccountQuota) {
	quotaMu.Lock()
	quotaCache[q.AccountID] = q
	quotaMu.Unlock()

	data, err := json.Marshal(q)
	if err == nil {
		_ = kvSet("quota_acct_"+strconv.FormatInt(q.AccountID, 10), string(data))
	}
}

// fetchUsagePage 抓取 gemini.google.com/usage 的 HTML
func fetchUsagePage(cookie, proxyURL string) ([]byte, error) {
	const pageURL = "https://gemini.google.com/usage"
	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	}
	if cookie != "" {
		headers["Cookie"] = cookie
	}

	var body []byte
	if proxyURL != "" {
		req, err := http.NewRequest("GET", pageURL, nil)
		if err != nil {
			return nil, err
		}
		applyChromeHeaders(req)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := getStdlibClient(proxyURL).Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("fetch /usage: HTTP %d", resp.StatusCode)
		}
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	} else {
		req, err := fhttp.NewRequest("GET", pageURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := getTLSClient().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("fetch /usage: HTTP %d", resp.StatusCode)
		}
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

// parseUsageHTML 从 /usage HTML 提取配额数据
func parseUsageHTML(accountID int64, html []byte) (AccountQuota, error) {
	q := AccountQuota{
		AccountID: accountID,
		FetchedAt: time.Now().Unix(),
		Tier:      "Free",
	}

	str := string(html)
	if !strings.Contains(str, `"SNlM0e"`) {
		q.Error = "Cookie已失效（未检测到登录态 SNlM0e）"
		return q, fmt.Errorf("%s", q.Error)
	}

	matches := preloadScriptRe.FindAllSubmatch(html, -1)
	if len(matches) == 0 {
		// 页面能打开但没有 preload 脚本，通常是 Free 用户且未开启用量页面特性
		if !strings.Contains(str, `"SNlM0e"`) {
			q.Error = "未检测到登录态 SNlM0e"
			return q, fmt.Errorf("%s", q.Error)
		}
		q.Tier = "Free"
		return q, nil
	}

	var foundUsage bool
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		var pData relayPreloadData
		if err := json.Unmarshal(m[1], &pData); err != nil {
			continue
		}

		for _, ev := range pData.Events {
			if ev.Kind != "chunk" || ev.Chunk == "" {
				continue
			}

			var items []relayChunkItem
			if err := json.Unmarshal([]byte(ev.Chunk), &items); err != nil {
				continue
			}

			for _, item := range items {
				if item.Data != nil && item.Data.Viewer != nil && item.Data.Viewer.Rpcs != nil &&
					item.Data.Viewer.Rpcs.GetUsageInfo != nil {
					info := item.Data.Viewer.Rpcs.GetUsageInfo
					foundUsage = true
					if info.SubscriptionTier == "TIER1" || info.SubscriptionTier != "" {
						q.Tier = "PRO"
					}
					for _, pool := range info.Pools {
						pct := math.Round(pool.PercentageUsed*1000) / 10 // 一位小数百分比
						resetTime := ""
						if pool.Gxu != nil {
							resetTime = pool.Gxu.WindowRefreshesAt
						}
						switch pool.PoolType {
						case "GXU_FIVE_HOURLY":
							q.FiveHourlyPct = pct
							q.FiveHourlyReset = resetTime
						case "GXU_WEEKLY":
							q.WeeklyPct = pct
							q.WeeklyReset = resetTime
						}
					}
				}

				if len(item.Errors) > 0 {
					for _, er := range item.Errors {
						if strings.Contains(er.Message, "Internal error") {
							// Free 账号请求 Pro 配额时 Google 后端报 code 13
							q.Tier = "Free"
							foundUsage = true
						}
					}
				}
			}
		}
	}

	if !foundUsage {
		q.Tier = "Free"
	}
	return q, nil
}

// checkAccountPro 探测该账号是否解锁了 Pro 官方模型
func checkAccountPro(cookie, proxyURL string) bool {
	sapisid := extractSAPISID(cookie)
	if sapisid == "" {
		return false
	}
	xsrf, err := getXSRF(cookie, proxyURL)
	if err != nil {
		return false
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
	if err != nil || status != 200 {
		return false
	}

	items, err := parseOTAQ7bResponse(body)
	if err != nil {
		return false
	}
	for _, it := range items {
		if strings.EqualFold(it.Name, "Pro") || strings.Contains(strings.ToLower(it.FullName), "pro") {
			return !it.Locked
		}
	}
	return false
}

// refreshAccountQuota 实时拉取并更新指定账号的配额
func refreshAccountQuota(a CookieAccount) (*AccountQuota, error) {
	proxyURL := proxyURLByID(a.ProxyID)

	body, err := fetchUsagePage(a.Cookie, proxyURL)
	if err != nil {
		q := AccountQuota{
			AccountID: a.ID,
			FetchedAt: time.Now().Unix(),
			Tier:      "Unknown",
			Error:     err.Error(),
		}
		saveAccountQuotaToCache(q)
		return &q, err
	}

	q, err := parseUsageHTML(a.ID, body)
	if q.Tier != "PRO" && err == nil {
		if checkAccountPro(a.Cookie, proxyURL) {
			q.Tier = "PRO"
		}
	}
	saveAccountQuotaToCache(q)
	return &q, err
}
