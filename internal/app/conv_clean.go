package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"
)

var cidRegex = regexp.MustCompile(`c_[a-f0-9]{16,}`)

// ClearAccountConversations 获取指定账号的所有网页历史会话并批量删除。
func ClearAccountConversations(a CookieAccount) (int, error) {
	if a.Cookie == "" {
		return 0, fmt.Errorf("账号 #%d 没有 cookie", a.ID)
	}
	proxyURL := ""
	if a.ProxyID > 0 {
		proxyURL = proxyURLByID(a.ProxyID)
	}
	xsrf, err := getXSRF(a.Cookie, proxyURL)
	if err != nil {
		return 0, fmt.Errorf("获取账号 #%d 的 XSRF 失败: %w", a.ID, err)
	}

	totalDeleted := 0
	// 每次取最多 100 条，循环直到为空
	for round := 0; round < 50; round++ {
		inner := `[100,null,[0,null,1]]`
		freq, _ := json.Marshal([]interface{}{[]interface{}{[]interface{}{"MaZiqc", inner, nil, "generic"}}})
		form := url.Values{}
		form.Set("f.req", string(freq))
		form.Set("at", xsrf)
		reqid := time.Now().UnixNano() % 1000000
		endpoint := fmt.Sprintf(
			"https://gemini.google.com/_/BardChatUi/data/batchexecute?rpcids=MaZiqc&bl=%s&hl=en&_reqid=%d&rt=c",
			currentBL(proxyURL), reqid)
		headers := buildGeminiHeaders(a.Cookie, extractSAPISID(a.Cookie), "")
		delete(headers, "x-goog-ext-525001261-jspb")

		status, _, body, err := uploadPost(endpoint, headers, []byte(form.Encode()), proxyURL)
		if err != nil || status != 200 {
			break
		}

		matches := cidRegex.FindAllString(string(body), -1)
		seen := map[string]bool{}
		var cids []string
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				cids = append(cids, m)
			}
		}

		if len(cids) == 0 {
			break
		}

		// 并发删除
		var wg sync.WaitGroup
		sem := make(chan struct{}, 15)
		roundDeleted := 0
		var mu sync.Mutex

		for _, cid := range cids {
			wg.Add(1)
			sem <- struct{}{}
			go func(c string) {
				defer wg.Done()
				defer func() { <-sem }()

				cInner, _ := json.Marshal([]interface{}{c})
				cFreq, _ := json.Marshal([]interface{}{[]interface{}{[]interface{}{"GzXR5e", string(cInner), nil, "generic"}}})
				cForm := url.Values{}
				cForm.Set("f.req", string(cFreq))
				cForm.Set("at", xsrf)
				cReqID := time.Now().UnixNano() % 1000000
				cEndpoint := fmt.Sprintf(
					"https://gemini.google.com/_/BardChatUi/data/batchexecute?rpcids=GzXR5e&bl=%s&hl=en&_reqid=%d&rt=c",
					currentBL(proxyURL), cReqID)

				cStatus, _, _, cErr := uploadPost(cEndpoint, headers, []byte(cForm.Encode()), proxyURL)
				if cErr == nil && cStatus == 200 {
					mu.Lock()
					roundDeleted++
					mu.Unlock()
				}
			}(cid)
		}
		wg.Wait()

		totalDeleted += roundDeleted
		if roundDeleted == 0 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	logf("[autodel] 账号 #%d (%s) 共清理删除 %d 个历史会话", a.ID, a.Label, totalDeleted)
	return totalDeleted, nil
}

// ClearAllAccountsConversations 清理池中全部 enabled 账号的网页历史会话。
func ClearAllAccountsConversations() (map[int64]int, error) {
	accts := accountList()
	results := make(map[int64]int)
	for _, a := range accts {
		if a.Status != "enabled" {
			continue
		}
		n, err := ClearAccountConversations(a)
		if err != nil {
			logf("[autodel] 清理账号 #%d 失败: %v", a.ID, err)
		}
		results[a.ID] = n
	}
	return results, nil
}
