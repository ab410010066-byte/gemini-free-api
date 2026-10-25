package app

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// 会话保活。同一条 POST /RotateCookies 有两种 payload，刷的不是同一族 cookie：
//
//  1. 哨兵 `[000,"-0000000000000000000"]`：无条件换发 `__Secure-1PSIDTS` /
//     `__Secure-3PSIDTS`。这是登录态真正的短命票（约 30 分钟），不刷就会被
//     当匿名。payload 必须是这串 JSPB（前导零合法、严格 JSON 非法），
//     json.Marshal 会变成 `[0,"…"]`，服务端不认。
//
//  2. 浏览器 iframe 那条：先 GET RotateCookiesPage 拿会话 id，再 POST
//     `[658,"<id>"]`。只刷新 SIDCC / `__Secure-1PSIDCC` / `__Secure-3PSIDCC`，
//     间隔由页面 init(...) 最后一个参数给出（实测 600 秒）。
//
// 以前只做了第 2 条，所以号大概半小时就死（issue #6）。Chrome 新版本还有
// DBSC 设备绑定（GET /RotateBoundCookies + 签名 JWT），那条我们复刻不了；
// 哨兵这条对 Firefox 导出、以及未绑定设备的会话有效。Chrome 导出的号如果
// 反复 401，换 Firefox 重新登录再导出。

const (
	rotatePageURL = "https://accounts.google.com/RotateCookiesPage" +
		"?og_pid=658&rot=3&origin=https%3A%2F%2Fgemini.google.com&exp_id=0"
	rotatePostURL = "https://accounts.google.com/RotateCookies"
	// og_pid 是产品标识，Gemini 固定 658；它既作为上面页面的 query，也回显在页面里。
	rotateProductID = 658
	// 服务端没给出间隔时的兜底值。
	defaultRotateInterval = 10 * time.Minute
	// 启动后尽快刷一次：导入时 cookie 可能已经快到期，干等 10 分钟会直接过期。
	firstRotateDelay = 15 * time.Second
	// 哨兵 payload。前导零是故意的，见文件头。
	rotate1PSIDTSBody = `[000,"-0000000000000000000"]`
	// 打太勤会 429。Gemini-API / notebooklm-py 都用 60 秒地板。
	min1PSIDTSInterval = 60 * time.Second
)

// 刷新 1PSIDTS 只带这一对。多带实测会 401。
var rotate1PSIDTSCookies = []string{"__Secure-1PSID", "__Secure-1PSIDTS"}

var (
	psidtsMu     sync.Mutex
	psidtsLastAt = map[int64]time.Time{}
)

// 页面里形如：init('4162200486104360679', 658.0, 0.0, 0.0, 600.0)
// 第一个参数是这个会话的标识，最后一个是下次轮转的间隔秒数。
var rotateInitRe = regexp.MustCompile(`init\('([^']{4,64})'\s*,\s*([0-9.]+)\s*,[^)]*?([0-9.]+)\s*\)`)

// rotateAccount 给一个账号做一次保活：使用本地 Firefox 容器安全同步取代向 Google 发送模拟伪造请求。
// 避免触发 Google 官方安全风控（HTTP 401 identity.hfcr），导致用户的 Google 账号被服务端强制踢出登出。
func rotateAccount(a CookieAccount) (time.Duration, []string, error) {
	if _, _, _, err := SyncAllFirefoxContainers(); err == nil {
		if fresh := accountByID(a.ID); fresh != nil && fresh.Cookie != a.Cookie {
			logf("[rotate] 账号 #%d 已从本地 Firefox 容器安全同步最新凭据", a.ID)
			return defaultRotateInterval, []string{"firefox_sync"}, nil
		}
	}
	return defaultRotateInterval, nil, nil
}

// tryRotate1PSIDTS: 已全面停用主动向 accounts.google.com/RotateCookies 发送模拟请求。
// 实测证实由后端主动抢换 1PSIDTS 会导致本地浏览器（Firefox）的旧 Token 被 Google 判定为 Token 重放攻击，
// 从而下发指令强制注销整组会话（导致用户账号被登出）。
// 所有凭证保活均统一由本地 Firefox 容器合法定时自动续约，网关仅通过本地 SQLite 静默安全同步。
func tryRotate1PSIDTS(id int64, cookie, proxyURL string) (string, []string, error) {
	return cookie, nil, nil
}

func allow1PSIDTSRotate(id int64) (bool, time.Duration) {
	psidtsMu.Lock()
	defer psidtsMu.Unlock()
	last := psidtsLastAt[id]
	if last.IsZero() {
		return true, 0
	}
	elapsed := time.Since(last)
	if elapsed >= min1PSIDTSInterval {
		return true, 0
	}
	return false, min1PSIDTSInterval - elapsed
}

func note1PSIDTSAttempt(id int64, err error) {
	// 成功、401/403、429 都记时间，避免紧接着再打。网络错误不记，允许立刻重试。
	if err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "HTTP 401") &&
			!strings.Contains(msg, "HTTP 403") &&
			!strings.Contains(msg, "HTTP 429") {
			return
		}
	}
	psidtsMu.Lock()
	psidtsLastAt[id] = time.Now()
	psidtsMu.Unlock()
}

// rotate1PSIDTS POST 哨兵 payload，把响应里的 Set-Cookie 合并回完整 cookie 串。
func rotate1PSIDTS(cookie, proxyURL string) (string, []string, error) {
	subset := cookieSubset(cookie, rotate1PSIDTSCookies)
	headers := map[string]string{
		"Accept":         "*/*",
		"Content-Type":   "application/json",
		"Origin":         "https://accounts.google.com",
		"Cookie":         subset,
		"Cache-Control":  "no-cache",
		"Pragma":         "no-cache",
		"Sec-Fetch-Dest": "empty",
		"Sec-Fetch-Mode": "cors",
		"Sec-Fetch-Site": "same-origin",
	}
	status, setCookie, respBody, err := rotatePost(rotatePostURL, headers, []byte(rotate1PSIDTSBody), proxyURL)
	if err != nil {
		return cookie, nil, err
	}
	if status == 401 || status == 403 {
		return cookie, nil, fmt.Errorf("RotateCookies 1PSIDTS 返回 HTTP %d（Chrome 导出的 cookie 可能受设备绑定限制，建议用 Firefox 重新导出）: %s",
			status, truncate(string(respBody), 120))
	}
	if status != 200 {
		return cookie, nil, fmt.Errorf("RotateCookies 1PSIDTS 返回 HTTP %d: %s", status, truncate(string(respBody), 120))
	}
	merged := mergeSetCookie(cookie, setCookie)
	return merged, setCookieNames(setCookie), nil
}

// rotateSIDCC 走浏览器 iframe 那条：GET 轮转页拿会话 id，再 POST [658, id]。
func rotateSIDCC(cookie, proxyURL string) (string, time.Duration, []string, error) {
	id, interval, pageSet, err := fetchRotateParams(cookie, proxyURL)
	if err != nil {
		return cookie, 0, nil, err
	}
	cookie = mergeSetCookie(cookie, pageSet)

	body := fmt.Sprintf(`[%d,"%s"]`, rotateProductID, id)
	headers := rotatePostHeaders()
	headers["Cookie"] = cookie
	status, setCookie, respBody, err := rotatePost(rotatePostURL, headers, []byte(body), proxyURL)
	if err != nil {
		return cookie, 0, nil, err
	}
	if status != 200 {
		return cookie, 0, nil, fmt.Errorf("RotateCookies 返回 HTTP %d: %s", status, truncate(string(respBody), 120))
	}
	cookie = mergeSetCookie(cookie, setCookie)
	names := setCookieNames(append(append([]string{}, pageSet...), setCookie...))
	return cookie, interval, names, nil
}

func uniqueKeepOrder(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// setCookieNames 把 Set-Cookie 头里的名字抽出来去重，只用于日志。
func setCookieNames(headers []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, h := range headers {
		name := h
		if i := strings.Index(name, "="); i > 0 {
			name = name[:i]
		}
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// fetchRotateParams 抓 RotateCookiesPage，取会话标识、服务端指定的间隔，以及这一发
// 自己带回的 Set-Cookie。
func fetchRotateParams(cookie, proxyURL string) (string, time.Duration, []string, error) {
	status, setCookie, body, err := rotateGet(rotatePageURL, cookie, proxyURL)
	if err != nil {
		return "", 0, nil, err
	}
	if status != 200 {
		return "", 0, nil, fmt.Errorf("取轮转页返回 HTTP %d", status)
	}
	m := rotateInitRe.FindSubmatch(body)
	if m == nil {
		// 页面拿到了却没有 init(...)，最可能是 cookie 已失效跳到了登录页。
		return "", 0, nil, fmt.Errorf("轮转页里没有 init(...)（cookie 可能已失效）")
	}
	id := string(m[1])
	interval := defaultRotateInterval
	if sec, e := strconv.ParseFloat(string(m[3]), 64); e == nil && sec >= 60 && sec <= 3600 {
		interval = time.Duration(sec) * time.Second
	}
	return id, interval, setCookie, nil
}

// 下面两组 header 逐项抄自抓包（wireHeaders，不是 headers —— 后者不含 cookie）。
// 抓包里还有 sec-ch-ua-arch / -bitness / -form-factors / -full-version-list /
// -model / -platform-version / -wow64 和 x-browser-* / x-client-data /
// x-chrome-id-consistency-request，那些是 Chrome 自己贴的浏览器身份，我们贴了反而
// 会跟 TLS 指纹对不上，所以不贴。

// rotateGet 取轮转页。它在浏览器里是个 iframe 导航，所以 sec-fetch 那组跟普通
// XHR 完全不同（dest=iframe / mode=navigate / site=same-site），别套用默认值。
func rotateGet(url, cookie, proxyURL string) (int, []string, []byte, error) {
	return rotateDo("GET", url, map[string]string{
		"Cookie":                    cookie,
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Referer":                   "https://gemini.google.com/",
		"Sec-Fetch-Dest":            "iframe",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "same-site",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"Priority":                  "u=0, i",
	}, nil, proxyURL)
}

// rotatePostHeaders 是 POST /RotateCookies 那一发的完整头，调用方只补 Cookie。
func rotatePostHeaders() map[string]string {
	return map[string]string{
		"Accept":         "*/*",
		"Content-Type":   "application/json",
		"Origin":         "https://accounts.google.com",
		"Referer":        rotatePageURL,
		"Cache-Control":  "no-cache",
		"Pragma":         "no-cache",
		"Priority":       "u=1, i",
		"Sec-Fetch-Dest": "empty",
		"Sec-Fetch-Mode": "same-origin",
		"Sec-Fetch-Site": "same-origin",
	}
}

func rotatePost(url string, headers map[string]string, body []byte, proxyURL string) (
	int, []string, []byte, error) {
	return rotateDo("POST", url, headers, body, proxyURL)
}

// rotateDo 走跟正式请求同一个出口：保活从别的 IP 发，等于告诉上游这个会话在两处活动。
//
// 两条传输路径共用同一份 header。以前只有走代理那条调 applyChromeHeaders，直连那条
// 连 User-Agent 都不发 —— 同一个账号在上游看来会因为走没走代理而呈现两种客户端。
func rotateDo(method, url string, headers map[string]string, body []byte, proxyURL string) (
	int, []string, []byte, error) {
	var rdr io.Reader
	if body != nil {
		rdr = strings.NewReader(string(body))
	}
	merged := map[string]string{
		"User-Agent":         ChromeUA,
		"Accept-Language":    "en-US,en;q=0.9",
		"Sec-CH-UA":          `"Chromium";v="146", "Google Chrome";v="146", "Not?A_Brand";v="24"`,
		"Sec-CH-UA-Mobile":   "?0",
		"Sec-CH-UA-Platform": `"Windows"`,
	}
	for k, v := range headers {
		merged[k] = v
	}
	if proxyURL != "" {
		req, err := http.NewRequest(method, url, rdr)
		if err != nil {
			return 0, nil, nil, err
		}
		for k, v := range merged {
			req.Header.Set(k, v)
		}
		resp, err := getStdlibClient(proxyURL).Do(req)
		if err != nil {
			return 0, nil, nil, err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Values("Set-Cookie"), b, err
	}
	req, err := fhttp.NewRequest(method, url, rdr)
	if err != nil {
		return 0, nil, nil, err
	}
	for k, v := range merged {
		req.Header.Set(k, v)
	}
	resp, err := getTLSClient().Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Values("Set-Cookie"), b, err
}

// rotateAllAccounts 给池子里每个启用的账号做一次保活，返回下次该等多久。
//
// 失败**不计入健康度**：保活打的是 accounts.google.com，跟对话能不能用是两码事，
// 网络抖一下就把号标成坏的，会让它在挑号时沉底，反而伤可用性。
func rotateAllAccounts() time.Duration {
	next := defaultRotateInterval
	for _, a := range accountList() {
		if a.Status != "enabled" {
			continue
		}
		iv, _, err := rotateAccount(a)
		if err != nil {
			logf("[rotate] 账号 #%d 保活失败: %v", a.ID, err)
			continue
		}
		if iv > 0 {
			next = iv
		}
	}
	return next
}
