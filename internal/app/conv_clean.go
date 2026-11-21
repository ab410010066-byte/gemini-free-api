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

// ConvCleanProgress 会话清理后台任务进度
type ConvCleanProgress struct {
	Running        bool             `json:"running"`
	TotalDeleted   int              `json:"total_deleted"`
	AccountsTotal  int              `json:"accounts_total"`
	AccountsDone   int              `json:"accounts_done"`
	CurrentAccount string           `json:"current_account"`
	StartTime      int64            `json:"start_time"`
	EndTime        int64            `json:"end_time"`
	Details        map[int64]int    `json:"details"`
	Errors         map[int64]string `json:"errors"`
	LastSummary    string           `json:"last_summary"`
}

type convCleanManager struct {
	mu       sync.RWMutex
	progress ConvCleanProgress
}

var globalConvCleaner = &convCleanManager{
	progress: ConvCleanProgress{
		Details: make(map[int64]int),
		Errors:  make(map[int64]string),
	},
}

// GetConvCleanProgress 获取当前清理任务进度快照
func GetConvCleanProgress() ConvCleanProgress {
	globalConvCleaner.mu.RLock()
	defer globalConvCleaner.mu.RUnlock()

	cp := globalConvCleaner.progress
	cp.Details = make(map[int64]int, len(globalConvCleaner.progress.Details))
	for k, v := range globalConvCleaner.progress.Details {
		cp.Details[k] = v
	}
	cp.Errors = make(map[int64]string, len(globalConvCleaner.progress.Errors))
	for k, v := range globalConvCleaner.progress.Errors {
		cp.Errors[k] = v
	}
	return cp
}

// StartBackgroundConvCleaning 启动后台会话清理任务（若已在执行则返回 false）
func StartBackgroundConvCleaning() (bool, ConvCleanProgress) {
	globalConvCleaner.mu.Lock()
	if globalConvCleaner.progress.Running {
		p := globalConvCleaner.progress
		globalConvCleaner.mu.Unlock()
		return false, p
	}

	accts := accountList()
	var enabled []CookieAccount
	for _, a := range accts {
		if a.Status == "enabled" {
			enabled = append(enabled, a)
		}
	}

	globalConvCleaner.progress = ConvCleanProgress{
		Running:        true,
		TotalDeleted:   0,
		AccountsTotal:  len(enabled),
		AccountsDone:   0,
		CurrentAccount: "准备启动",
		StartTime:      time.Now().Unix(),
		EndTime:        0,
		Details:        make(map[int64]int),
		Errors:         make(map[int64]string),
		LastSummary:    "清理任务运行中...",
	}
	p := globalConvCleaner.progress
	globalConvCleaner.mu.Unlock()

	go func(targetAccts []CookieAccount) {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 2) // 最多 2 个账号并发清理，平稳控制网络负载避免触发 Google WAF

		for _, a := range targetAccts {
			wg.Add(1)
			sem <- struct{}{}
			go func(acc CookieAccount) {
				defer wg.Done()
				defer func() { <-sem }()

				globalConvCleaner.mu.Lock()
				globalConvCleaner.progress.CurrentAccount = acc.Label
				globalConvCleaner.mu.Unlock()

				n, err := ClearAccountConversations(acc)

				globalConvCleaner.mu.Lock()
				if err != nil {
					globalConvCleaner.progress.Errors[acc.ID] = err.Error()
					logf("[autodel] 清理账号 #%d (%s) 失败: %v", acc.ID, acc.Label, err)
				} else {
					globalConvCleaner.progress.Details[acc.ID] = n
					globalConvCleaner.progress.TotalDeleted += n
				}
				globalConvCleaner.progress.AccountsDone++
				globalConvCleaner.mu.Unlock()
			}(a)
		}
		wg.Wait()

		globalConvCleaner.mu.Lock()
		globalConvCleaner.progress.Running = false
		globalConvCleaner.progress.EndTime = time.Now().Unix()
		globalConvCleaner.progress.CurrentAccount = ""
		summary := fmt.Sprintf("清理完成：共处理 %d 个账号，删除 %d 条会话",
			globalConvCleaner.progress.AccountsDone, globalConvCleaner.progress.TotalDeleted)
		globalConvCleaner.progress.LastSummary = summary
		globalConvCleaner.mu.Unlock()

		logf("[autodel] %s", summary)
	}(enabled)

	return true, p
}

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

// ClearAllAccountsConversations 同步清理池中全部 enabled 账号的网页历史会话（并发安全）。
func ClearAllAccountsConversations() (map[int64]int, error) {
	accts := accountList()
	results := make(map[int64]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 2)

	for _, a := range accts {
		if a.Status != "enabled" {
			continue
		}
		wg.Add(1)
		go func(acc CookieAccount) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			n, err := ClearAccountConversations(acc)
			mu.Lock()
			if err != nil {
				logf("[autodel] 清理账号 #%d 失败: %v", acc.ID, err)
			}
			results[acc.ID] = n
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	return results, nil
}
