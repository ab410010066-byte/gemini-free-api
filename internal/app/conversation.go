package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/google/uuid"
)

// convState 是一路 Gemini 原生会话的状态。整条会话绑定同一个 cookie / 出口，中途不换
// —— 换了 Gemini 就当成另一个会话，续接的 cid 失效。
type convState struct {
	cid, rid, rcid, tok26 string
	turn                  int    // 会话内轮次索引，填 inner[17]，逐轮 +1
	cookie                string // 会话绑定 cookie：登录号的 cookie，或匿名 /app 拿的 session
	sapisid               string // 登录态算 SAPISIDHASH 用；匿名为空
	xsrf                  string // 登录态 XSRF (at) 校验值
	isLogin               bool
	accountID             int64
	accountLabel          string
	proxyID               int64
	proxyURL              string
	updated               time.Time
}

const (
	// convTTL 跟导出 cookie ~2 小时寿命对齐，过期的会话状态没有复用价值。
	convTTL = 90 * time.Minute
	convMax = 4000
)

var (
	convMu    sync.Mutex
	convStore = map[string]*convState{}
)

// convDelete 作废一路会话（续接失败时用，避免客户端重试还撞同一路死会话）。
func convDelete(key string) {
	if key == "" {
		return
	}
	convMu.Lock()
	delete(convStore, key)
	convMu.Unlock()
}

// convGet 取一路会话，顺带清掉过期的。
func convGet(key string) *convState {
	if key == "" {
		return nil
	}
	convMu.Lock()
	defer convMu.Unlock()
	st, ok := convStore[key]
	if !ok {
		return nil
	}
	if time.Since(st.updated) > convTTL {
		delete(convStore, key)
		return nil
	}
	return st
}

// convPut 存一路会话，超量就按最旧的淘汰一批。
func convPut(key string, st *convState) {
	if key == "" {
		return
	}
	convMu.Lock()
	defer convMu.Unlock()
	st.updated = time.Now()
	convStore[key] = st
	if len(convStore) > convMax {
		// 简单淘汰：删掉过期的；还超就删最旧的一小批。
		var oldestK string
		var oldestT time.Time
		first := true
		for k, v := range convStore {
			if time.Since(v.updated) > convTTL {
				delete(convStore, k)
				continue
			}
			if first || v.updated.Before(oldestT) {
				oldestK, oldestT, first = k, v.updated, false
			}
		}
		if len(convStore) > convMax && oldestK != "" {
			delete(convStore, oldestK)
		}
	}
}

// canonicalMessages 把消息列表压成 role+text 的稳定串，用来算会话指纹。
// 只取 role 和文本内容 —— 客户端每轮重发同样的历史，压出来的串一致才能识别续接。
func canonicalMessages(messages []map[string]interface{}) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(getStr(m, "role"))
		b.WriteByte(0x1f)
		b.WriteString(contentToString(m["content"]))
		b.WriteByte(0x1e)
	}
	return b.String()
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// convParentKey 是"这轮之前的历史"的指纹：除最后一条消息外的全部。
// 命中 store 说明这是某路已知会话的延续，只要发最后一条新消息即可。
func convParentKey(messages []map[string]interface{}) string {
	if len(messages) < 2 {
		return ""
	}
	return hashStr(canonicalMessages(messages[:len(messages)-1]))
}

// convChildKey 是"这轮之后的历史"的指纹：历史 + 本轮模型回复。
// 下一轮请求的 parentKey 会正好等于它（客户端把我们的回复原样带回来），从而续上。
func convChildKey(messages []map[string]interface{}, responseText string) string {
	full := append(append([]map[string]interface{}{}, messages...),
		map[string]interface{}{"role": "assistant", "content": responseText})
	return hashStr(canonicalMessages(full))
}

var rcidRe = regexp.MustCompile(`"(rc_[A-Za-z0-9_-]{6,})"`)

// parseConvIDs 从 StreamGenerate 响应里取续接要用的四样：cid=[1][0]、rid=[1][1]、
// tok26=帧[26]、rcid=正文里的 rc_xxx（登录态续接第 3 位要用它，匿名为空）。
func parseConvIDs(raw string) (cid, rid, rcid, tok26 string) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[[") {
			continue
		}
		var arr []interface{}
		if json.Unmarshal([]byte(line), &arr) != nil {
			continue
		}
		for _, it := range arr {
			row, ok := it.([]interface{})
			if !ok || len(row) < 3 || row[0] != "wrb.fr" {
				continue
			}
			payload, ok := row[2].(string)
			if !ok {
				continue
			}
			var f []interface{}
			if json.Unmarshal([]byte(payload), &f) != nil {
				continue
			}
			if len(f) > 1 {
				if meta, ok := f[1].([]interface{}); ok && len(meta) >= 2 {
					if s, ok := meta[0].(string); ok && s != "" {
						cid = s
					}
					if s, ok := meta[1].(string); ok && s != "" {
						rid = s
					}
				}
			}
			if len(f) > 26 {
				if s, ok := f[26].(string); ok && s != "" {
					tok26 = s
				}
			}
		}
	}
	if m := rcidRe.FindStringSubmatch(raw); m != nil {
		rcid = m[1]
	}
	return
}

// getAnonSession 匿名 GET /app 拿一份 session cookie（NID/COMPASS 等），给匿名多轮当会话
// 载体。续接请求必须每轮带上它，否则被服务端当新会话（实测不带 0/4 通、带 4/4 通）。
// 必须走跟正式请求同一出口。
func getAnonSession(proxyURL string) (string, error) {
	req := func() (*http.Response, io.ReadCloser, []string, error) {
		if proxyURL != "" {
			r, err := http.NewRequest("GET", "https://gemini.google.com/app", nil)
			if err != nil {
				return nil, nil, nil, err
			}
			applyChromeHeaders(r)
			resp, err := getStdlibClient(proxyURL).Do(r)
			if err != nil {
				return nil, nil, nil, err
			}
			return resp, resp.Body, resp.Header.Values("Set-Cookie"), nil
		}
		r, err := fhttp.NewRequest("GET", "https://gemini.google.com/app", nil)
		if err != nil {
			return nil, nil, nil, err
		}
		r.Header.Set("User-Agent", webUA)
		resp, err := getTLSClient().Do(r)
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, resp.Body, resp.Header.Values("Set-Cookie"), nil
	}
	_, body, setCookies, err := req()
	if err != nil {
		return "", err
	}
	if body != nil {
		io.Copy(io.Discard, body)
		body.Close()
	}
	var pairs []string
	seen := map[string]bool{}
	for _, sc := range setCookies {
		nv := strings.SplitN(sc, ";", 2)[0]
		name := strings.TrimSpace(strings.SplitN(nv, "=", 2)[0])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		pairs = append(pairs, strings.TrimSpace(nv))
	}
	if len(pairs) == 0 {
		return "", fmt.Errorf("匿名 /app 没返回 Set-Cookie")
	}
	return strings.Join(pairs, "; "), nil
}

// webUA 是匿名 GET /app 的 User-Agent（tls-client 路径要手动带）。
const webUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

// streamGenerateConv 发一路会话的一轮。turn==0 是首轮（建会话），>0 是续接（只发新消息）。
func streamGenerateConv(prompt string, mc ModelConfig, conv *convState,
	onDelta, onReasoning func(string)) (*StreamResult, error) {
	return streamGenerateConvWithLatest(prompt, "", mc, conv, onDelta, onReasoning)
}

func streamGenerateConvWithLatest(prompt, latest string, mc ModelConfig, conv *convState,
	onDelta, onReasoning func(string)) (*StreamResult, error) {
	p, slotOK, slotErr := acquireSlot(conv.proxyID)
	if !slotOK {
		return &StreamResult{}, slotErr
	}
	defer releaseSlot(p.ID)
	proxyURL := p.URL

	// 首轮定出口；续接沿用会话原出口（acquireSlot 已优先，但拿不到原出口时只能换，
	// 换了续接大概率失败，靠调用方回退到全量重发兜底）。
	if conv.turn == 0 {
		conv.proxyID = p.ID
		conv.proxyURL = proxyURL
		// 匿名首轮：没有登录 cookie，就地拿一份 session cookie 当会话载体。
		if conv.cookie == "" && !conv.isLogin {
			c, err := getAnonSession(proxyURL)
			if err != nil {
				return &StreamResult{ProxyID: p.ID, ProxyName: p.Name}, fmt.Errorf("建匿名会话失败: %w", err)
			}
			conv.cookie = c
		}
	}

	// 登录态每轮要带 at（XSRF）；匿名不要。
	xsrf := ""
	if conv.isLogin && conv.cookie != "" {
		if tok, err := getXSRF(conv.cookie, proxyURL); err == nil {
			xsrf = tok
		} else {
			// 换号之前先给它一次机会：强制轮转一次再重取
			if freshAcct := accountByID(conv.accountID); freshAcct != nil {
				if _, _, rerr := rotateAccount(*freshAcct); rerr == nil {
					if refreshed := accountByID(conv.accountID); refreshed != nil {
						if tok2, err2 := getXSRF(refreshed.Cookie, proxyURL); err2 == nil {
							logf("[cookie] 账号 #%d 轮转后恢复可用", conv.accountID)
							conv.cookie = refreshed.Cookie
							conv.sapisid = extractSAPISID(refreshed.Cookie)
							xsrf = tok2
						}
					}
				}
			}
			if xsrf == "" {
				return &StreamResult{
					ProxyID: p.ID, ProxyName: p.Name,
					AccountID: conv.accountID, AccountLabel: conv.accountLabel,
				}, fmt.Errorf("取 XSRF 失败: %w", err)
			}
		}
		conv.xsrf = xsrf
	}

	// 超长处理三阶段：
	// 1. 无损压缩（compressPromptLossless，内含在 prepareContextFile 第一步）
	// 2. 若在 (budget, maxAttachmentIngestBytes] 且有登录 cookie → 走 message.txt 附件单轮发送
	// 3. 若 > maxAttachmentIngestBytes 或匿名超长 → 走原生会话多轮分块喂入（runChunkedOnSlot）
	var files []fileRef
	budget := rtCfg().MaxPromptBytes
	loginCookie := ""
	if conv.isLogin {
		loginCookie = conv.cookie
	}
	if pPrompt, f, used, ferr := prepareContextFile(prompt, latest, budget, loginCookie, proxyURL); ferr != nil {
		return &StreamResult{
			ProxyID: p.ID, ProxyName: p.Name,
			AccountID: conv.accountID, AccountLabel: conv.accountLabel,
		}, ferr
	} else if used {
		prompt = pPrompt
		files = append(files, f...)
	} else {
		prompt = pPrompt // 可能已被无损压缩瘦身到 budget 以内
	}

	if budget > 0 && len(prompt) > budget {
		if rtCfg().ChunkedMultiTurn && len(prompt) <= maxChunkedPromptBytes {
			res, cerr := runChunkedOnSlot(prompt, latest, mc, conv, p, xsrf, onDelta, onReasoning)
			if cerr == nil && rtCfg().AutoDeleteConversation && conv.cookie != "" && xsrf != "" && conv.cid != "" {
				go deleteConversation(conv.cid, conv.cookie, conv.sapisid, xsrf, p.URL)
			}
			return res, cerr
		}
		return &StreamResult{
			ProxyID: p.ID, ProxyName: p.Name,
			AccountID: conv.accountID, AccountLabel: conv.accountLabel,
		}, &PromptTooLongError{Bytes: len(prompt), Budget: budget, HasCookie: conv.isLogin}
	}

	return doSingleConvTurn(prompt, files, mc, conv, p, xsrf, onDelta, onReasoning)
}

// buildFinalChunkPrompt 构造多轮分块喂入最后一轮的提示词。
func buildFinalChunkPrompt(lastChunk, latest string, totalChunks, budget int, hasTools bool) string {
	var b strings.Builder
	fmt.Fprintf(&b,
		"[System instruction]: Below is the final Part %d of %d of the conversation context/document. "+
			"Now combine all %d parts from this session and answer the latest user request directly.\n\n"+
			"--- BEGIN PART %d/%d ---\n%s\n--- END PART %d/%d ---\n",
		totalChunks, totalChunks, totalChunks,
		totalChunks, totalChunks, lastChunk, totalChunks, totalChunks)
	latest = strings.TrimSpace(latest)
	if latest != "" && len(latest) <= latestInlineLimit(budget) && !strings.HasSuffix( strings.TrimSpace(lastChunk), latest) {
		fmt.Fprintf(&b, "\nLatest user request:\n%s\n", latest)
	}
	if hasTools {
		b.WriteString(toolAnchorSuffix)
	}
	return b.String()
}

// runChunkedOnSlot 在已持有出口 slot (p) 的前提下，将超长 prompt 切分成多轮依次喂入同一原生会话。
// 前 N-1 轮关闭扩展思考以加速摄取且不向客户端吐流；第 N 轮按原模型配置流式返回最终回答。
func runChunkedOnSlot(prompt, latest string, mc ModelConfig, conv *convState, p Proxy, xsrf string,
	onDelta, onReasoning func(string)) (*StreamResult, error) {
	budget := rtCfg().MaxPromptBytes
	chunkBudget := budget - 4096
	if chunkBudget <= 0 || chunkBudget > 110000 {
		chunkBudget = 110000
	}
	if budget > 0 && chunkBudget > budget*4/5 {
		chunkBudget = budget * 4 / 5
	}
	chunks := splitPromptIntoChunks(prompt, chunkBudget)
	if len(chunks) <= 1 {
		return doSingleConvTurn(prompt, nil, mc, conv, p, xsrf, onDelta, onReasoning)
	}

	logf("[chunked] 超长上下文 %d 字节，自动拆分为 %d 轮分批注入会话 (每轮 <= %d 字节)",
		len(prompt), len(chunks), chunkBudget)

	// 匿名会话若还没拿 session cookie，先拿一份以串联多轮
	if conv.turn == 0 && conv.cookie == "" && !conv.isLogin {
		c, err := getAnonSession(p.URL)
		if err != nil {
			return &StreamResult{ProxyID: p.ID, ProxyName: p.Name}, fmt.Errorf("分块多轮建匿名会话失败: %w", err)
		}
		conv.cookie = c
	}

	tStart := time.Now()
	// 前 N-1 轮只做上下文摄取，关闭 Thinking 避免每轮花十几秒思考 "OK"
	ingestMC := mc
	ingestMC.Thinking = false

	for i := 0; i < len(chunks)-1; i++ {
		if i > 0 {
			// 平滑步调防风控：分块之间休眠 800ms ~ 1200ms，避免突发高频触发 Google 防爬虫 302
			sleepMs := 800 + (time.Now().UnixNano()%400)/1e6
			time.Sleep(time.Duration(sleepMs) * time.Millisecond)

			// 记录实际发出的分块请求，确保不突破 RPH / RPM 安全红线
			if ok, reason := recordSlotHit(p.ID); !ok {
				return &StreamResult{ProxyID: p.ID, ProxyName: p.Name}, &RateLimitError{Reason: reason, ProxyID: p.ID}
			}
		}

		partPrompt := fmt.Sprintf(
			"[System instruction]: Below is Part %d of %d of the conversation context/document. "+
				"Read and hold this segment in memory for the final request. "+
				"Do NOT answer the user question or call any tools yet — reply ONLY with \"OK\" and wait for Part %d.\n\n"+
				"--- BEGIN PART %d/%d ---\n%s\n--- END PART %d/%d ---",
			i+1, len(chunks), i+2,
			i+1, len(chunks), chunks[i], i+1, len(chunks),
		)
		res, err := doSingleConvTurn(partPrompt, nil, ingestMC, conv, p, xsrf, nil, nil)
		if err != nil {
			return res, fmt.Errorf("分块喂入第 %d/%d 轮失败: %w", i+1, len(chunks), err)
		}
		logf("[chunked] 第 %d/%d 轮摄取完成 (%d 字节, cid=%s, turn=%d)",
			i+1, len(chunks), len(chunks[i]), conv.cid, conv.turn)
	}

	if len(chunks) > 1 {
		// 最后一轮正式提问前同样平滑缓冲，防止与前一轮摄取贴得太紧
		sleepMs := 800 + (time.Now().UnixNano()%400)/1e6
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
		if ok, reason := recordSlotHit(p.ID); !ok {
			return &StreamResult{ProxyID: p.ID, ProxyName: p.Name}, &RateLimitError{Reason: reason, ProxyID: p.ID}
		}
	}

	hasTools := strings.Contains(prompt, "```tool_call")
	finalPrompt := buildFinalChunkPrompt(chunks[len(chunks)-1], latest, len(chunks), budget, hasTools)
	res, err := doSingleConvTurn(finalPrompt, nil, mc, conv, p, xsrf, onDelta, onReasoning)
	if res != nil {
		res.TotalMs = time.Since(tStart).Milliseconds()
	}
	return res, err
}

// doSingleConvTurn 在已持有出口 p 和 xsrf 的条件下执行单轮会话请求并更新 conv 状态。
func doSingleConvTurn(prompt string, files []fileRef, mc ModelConfig, conv *convState,
	p Proxy, xsrf string, onDelta, onReasoning func(string)) (*StreamResult, error) {
	proxyURL := p.URL
	pickedOK := p.ID > 0

	inner := make([]interface{}, innerSlots)
	if len(files) > 0 {
		refs := make([]interface{}, 0, len(files))
		for _, f := range files {
			kind := f.Kind
			if kind == 0 {
				kind = 3
			}
			mime := f.Mime
			if mime == "" {
				mime = "text/plain"
			}
			refs = append(refs, []interface{}{
				[]interface{}{f.Ref, kind, nil, mime}, f.Name,
				nil, nil, nil, nil, nil, nil,
				[]interface{}{0},
			})
		}
		inner[0] = []interface{}{prompt, 0, nil, refs, nil, nil, 0}
	} else {
		inner[0] = []interface{}{prompt, 0, nil, nil, nil, nil, 0}
	}
	inner[1] = []interface{}{"en"}
	if conv.turn == 0 {
		inner[2] = []interface{}{"", "", "", nil, nil, nil, nil, nil, nil, ""}
	} else {
		// 续接：[cid, rid, rcid(登录)/""(匿名), null×6, tok26]
		inner[2] = []interface{}{conv.cid, conv.rid, conv.rcid,
			nil, nil, nil, nil, nil, nil, conv.tok26}
	}
	inner[6] = []interface{}{0}
	inner[7] = 1
	inner[10] = 1
	inner[11] = 0
	inner[17] = []interface{}{[]interface{}{conv.turn}}
	inner[18] = 0
	inner[27] = 1
	inner[30] = []interface{}{4}
	inner[41] = []interface{}{1}
	inner[53] = 0
	reqUUID := uuid.NewString()
	inner[59] = reqUUID
	inner[61] = []interface{}{}
	inner[68] = 1
	inner[79] = mc.Mode
	inner[80] = thinkingNormal
	inner[91] = 0
	inner[96] = 0
	if mc.Thinking {
		inner[80] = thinkingExtended
		inner[96] = 1
	}

	innerJSON, _ := json.Marshal(inner)
	outerJSON, _ := json.Marshal([]interface{}{nil, string(innerJSON)})

	buildBody := func(at string) string {
		form := url.Values{}
		form.Set("f.req", string(outerJSON))
		if at != "" {
			form.Set("at", at)
		}
		return form.Encode()
	}
	body := buildBody(xsrf)

	thinkVal := thinkingNormal
	if mc.Thinking {
		thinkVal = thinkingExtended
	}
	geminiHeaders := buildGeminiHeaders(conv.cookie, conv.sapisid, mc.HexID)
	geminiHeaders["x-goog-ext-525001261-jspb"] = buildModelHeader(mc.HexID, mc.Mode, thinkVal, uuid.NewString())
	geminiHeaders["x-goog-ext-525005358-jspb"] = fmt.Sprintf(`["%s",1]`, reqUUID)

	reqid := time.Now().UnixNano() % 1000000
	endpoint := fmt.Sprintf(
		"https://gemini.google.com/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate?bl=%s&hl=en&_reqid=%d&rt=c",
		currentBL(proxyURL), reqid)

	tracker := &deltaTracker{}
	rtracker := &deltaTracker{}
	var lineCB func(string)
	if onDelta != nil || onReasoning != nil {
		lineCB = func(line string) {
			if onReasoning != nil {
				if r := reasoningInLine(line); r != "" {
					if d := rtracker.Push(r); d != "" {
						onReasoning(d)
					}
				}
			}
			if onDelta == nil {
				return
			}
			for _, t := range textsInLine(line) {
				if d := tracker.Push(t); d != "" {
					onDelta(d)
				}
			}
		}
	}

	t0 := time.Now()
	var lastErr error
	attempts := rtCfg().RetryAttempts
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		statusCode, raw, ttfb, setCookie, err := doGeminiRequest(endpoint, body, geminiHeaders, proxyURL, lineCB)
		if len(setCookie) > 0 && conv.cookie != "" {
			if merged := mergeSetCookie(conv.cookie, setCookie); merged != conv.cookie {
				conv.cookie = merged
				if conv.accountID > 0 {
					updateAccountCookie(conv.accountID, merged)
				}
			}
		}
		if err != nil || statusCode != 200 || !hasContentFrame(string(raw)) {
			if err == nil && statusCode == 200 {
				lastErr = fmt.Errorf("续接无内容帧（会话可能已失效，raw %d 字节）", len(raw))
			} else if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("upstream HTTP %d: %s", statusCode, truncate(string(raw), 160))
			}
			if pickedOK {
				recordProxyResult(p.ID, false, lastErr.Error())
			}
			if tracker.emitted != "" || rtracker.emitted != "" {
				break
			}
			if attempt < rtCfg().RetryAttempts-1 {
				time.Sleep(time.Duration(rtCfg().RetryDelaySec) * time.Second)
			}
			continue
		}
		// 成功：回填续接状态。
		cid, rid, rcid, tok26 := parseConvIDs(string(raw))
		if cid != "" {
			conv.cid = cid
		}
		if rid != "" {
			conv.rid = rid
		}
		if rcid != "" {
			conv.rcid = rcid
		}
		conv.tok26 = tok26 // 每轮更新（可能为空）
		conv.turn++
		if pickedOK {
			recordProxyResult(p.ID, true, "")
		}
		if conv.accountID > 0 {
			markCookieByStatus(conv.accountID, 200, "")
		}
		return &StreamResult{
			Emitted:          tracker.emitted,
			EmittedReasoning: rtracker.emitted,
			Raw:              string(raw),
			Reasoning:        extractReasoning(string(raw)),
			UpstreamModel:    extractUpstreamModel(string(raw)),
			ProxyID:          p.ID,
			ProxyName:        p.Name,
			AccountID:        conv.accountID,
			AccountLabel:     conv.accountLabel,
			TTFBMs:           ttfb,
			TotalMs:          time.Since(t0).Milliseconds(),
		}, nil
	}
	return &StreamResult{
		ProxyID: p.ID, ProxyName: p.Name,
		AccountID: conv.accountID, AccountLabel: conv.accountLabel,
	}, lastErr
}

// callGeminiConv 是多轮开启时的入口：按历史前缀识别续接。
//   - 命中已知会话 → 只发最后一条新消息（user 或 tool 结果），历史留服务端（绕开字节墙）。
//   - 没命中 → 新建会话，首轮发全量拼接的历史（含 tools 指令，把当前上下文交给服务端）。
//
// 支持 tools（续接里也解析 ```tool_call``` 围栏）；图片走原单轮路径（callGemini）。
func callGeminiConv(messages []map[string]interface{}, mc ModelConfig,
	tools []map[string]interface{}, toolChoice interface{},
	onDelta, onReasoning func(string)) (string, []ToolCall, *StreamResult, error) {
	parentKey := convParentKey(messages)
	conv := convGet(parentKey)
	fresh := conv == nil

	if fresh {
		budget := rtCfg().MaxPromptBytes
		triedAccounts := map[int64]bool{}
		_, enabledAccts := accountCount()
		maxTries := maxCookieTries
		if enabledAccts > maxTries {
			maxTries = enabledAccts
		}
		if maxTries < 1 {
			maxTries = 1
		}

		var lastErr error
		var lastRes *StreamResult

		for attempt := 0; attempt < maxTries; attempt++ {
			conv = &convState{}
			prompt, latest := messagesToPrompt(messages, tools, toolChoice)
			needsCookie := budget > 0 && len(prompt) > budget

			if !anonFirstEligible(mc, needsCookie) {
				if a, ok := pickCookieAccountExcept(triedAccounts); ok {
					triedAccounts[a.ID] = true
					conv.cookie = a.Cookie
					conv.sapisid = extractSAPISID(a.Cookie)
					conv.isLogin = true
					conv.accountID = a.ID
					conv.accountLabel = accountDisplayName(a)
					conv.proxyID = a.ProxyID
				} else if len(triedAccounts) > 0 && !rtCfg().FallbackAnon {
					// 试过的号都不可用且不允许降级匿名
					break
				}
			}

			if attempt == 0 {
				logf("[conv] 新会话，首轮发 %d 字节（tools=%d, 账号=%s#%d）", len(prompt), len(tools), conv.accountLabel, conv.accountID)
			} else {
				logf("[conv] 新会话换号重试 (%d/%d)，切到账号=%s#%d", attempt+1, maxTries, conv.accountLabel, conv.accountID)
			}

			res, err := streamGenerateConvWithLatest(prompt, latest, mc, conv, onDelta, onReasoning)
			lastRes = res
			if err == nil {
				text := extractResponseText(res.Raw)
				var toolCalls []ToolCall
				if len(tools) > 0 {
					text, toolCalls = parseToolCalls(text)
				}
				if text != "" || len(toolCalls) > 0 {
					convPut(convChildKey(messages, text), conv)
					if rtCfg().AutoDeleteConversation && conv.cookie != "" && conv.cid != "" && conv.xsrf != "" {
						go deleteConversation(conv.cid, conv.cookie, conv.sapisid, conv.xsrf, conv.proxyURL)
					}
					return text, toolCalls, res, nil
				}
				err = fmt.Errorf("upstream returned no content frame (raw %d bytes)", len(res.Raw))
			}

			lastErr = err

			// 检查是否已经向客户端推送过内容（SSE 数据流），如果已经推送过就不能换号重试了（避免输出重复）
			if res != nil && (res.Emitted != "" || res.EmittedReasoning != "") {
				return "", nil, res, err
			}

			// 如果是 IP 出口槽位已满（并发/RPM/RPH 限流），换 Cookie 账号依然走同一个 IP 出口，徒劳重试无意义，直接返回
			var rlErr *RateLimitError
			if errors.As(err, &rlErr) {
				return "", nil, res, err
			}

			// 如果当前账号鉴权报错（比如 XSRF 失败、Cookie 失效、401/403 等）才记死，避免模型或上游抖动误杀好号
			if conv.accountID > 0 {
				isAuthErr := strings.Contains(err.Error(), "SNlM0e") ||
					strings.Contains(err.Error(), "401") ||
					strings.Contains(err.Error(), "403") ||
					strings.Contains(err.Error(), "XSRF")
				if isAuthErr {
					markCookieByStatus(conv.accountID, 401, err.Error())
				}
				logf("[cookie] 账号 #%d 失败 (%v)，换池子中下一个可用账号", conv.accountID, err)
			}

			// 如果不是登录号（即走匿名），或者池子里没有更多可用账号了，就不用继续循环
			if !conv.isLogin || len(triedAccounts) >= enabledAccts {
				break
			}
		}

		return "", nil, lastRes, lastErr
	}

	// 续接命中已有会话
	prompt := formatNewTurn(messages[len(messages)-1])
	latest := latestUserMessage(messages)
	logf("[conv] 续接命中 turn=%d，只发 %d 字节（历史留服务端）", conv.turn, len(prompt))

	res, err := streamGenerateConvWithLatest(prompt, latest, mc, conv, onDelta, onReasoning)
	if err != nil {
		// 续接失败：这路会话作废
		convDelete(parentKey)
		// 如果还没向客户端输出过任何内容，自动降级为新会话（全量重发并允许换号）
		if res == nil || (res.Emitted == "" && res.EmittedReasoning == "") {
			logf("[conv] 续接失败 (%v)，未输出内容，自动无感降级为新会话全量重试", err)
			return callGeminiConv(messages, mc, tools, toolChoice, onDelta, onReasoning)
		}
		return "", nil, res, err
	}

	text := extractResponseText(res.Raw)
	var toolCalls []ToolCall
	if len(tools) > 0 {
		text, toolCalls = parseToolCalls(text)
	}
	if text == "" && len(toolCalls) == 0 {
		convDelete(parentKey)
		if res == nil || (res.Emitted == "" && res.EmittedReasoning == "") {
			logf("[conv] 续接无内容帧，自动无感降级为新会话全量重试")
			return callGeminiConv(messages, mc, tools, toolChoice, onDelta, onReasoning)
		}
		return "", nil, res, fmt.Errorf("upstream returned no content frame (raw %d bytes)", len(res.Raw))
	}
	convPut(convChildKey(messages, text), conv)
	if rtCfg().AutoDeleteConversation && conv.cookie != "" && conv.cid != "" && conv.xsrf != "" {
		go deleteConversation(conv.cid, conv.cookie, conv.sapisid, conv.xsrf, conv.proxyURL)
	}
	return text, toolCalls, res, nil
}

// formatNewTurn 把续接要发的那条新消息格式化，跟 messagesToPrompt 里各角色的写法对齐，
// 好让服务端历史里的模型认得出这是工具结果还是用户新话。
func formatNewTurn(msg map[string]interface{}) string {
	content := contentToString(msg["content"])
	if getStr(msg, "role") == "tool" {
		return fmt.Sprintf("[Tool result for %s]: %s", getStr(msg, "name"), content)
	}
	return content
}
