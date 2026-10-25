package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestProbeAllAccountsModels(t *testing.T) {
	dbOnce = sync.Once{}
	db = nil
	dbw = nil
	cfg.DBPath = "../../data/gemini.db"
	initRuntimeConfig()

	accts := accountList()
	for _, a := range accts {
		if a.Status != "enabled" || a.Cookie == "" {
			continue
		}
		sapisid := extractSAPISID(a.Cookie)
		xsrf, err := getXSRF(a.Cookie, "")
		if err != nil {
			t.Logf("Account %s (#%d): getXSRF failed: %v", a.Label, a.ID, err)
			continue
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
			currentBL(""), reqid,
		)
		headers := buildGeminiHeaders(a.Cookie, sapisid, "")
		delete(headers, "x-goog-ext-525001261-jspb")

		status, _, body, err := uploadPost(endpoint, headers, []byte(form.Encode()), "")
		if err != nil || status != 200 {
			t.Logf("Account %s (#%d): otAQ7b status=%d, err=%v", a.Label, a.ID, status, err)
			continue
		}

		items, err := parseOTAQ7bResponse(body)
		t.Logf("=== Account %s (#%d) otAQ7b items (%d items, err=%v) ===", a.Label, a.ID, len(items), err)
		for _, it := range items {
			t.Logf("  Model: HexID=%s, Name=%q, FullName=%q, Mode=%d, Locked=%v, Desc=%q",
				it.HexID, it.Name, it.FullName, it.Mode, it.Locked, it.Desc)
		}
	}
}

func TestProbeArgonDirect(t *testing.T) {
	dbOnce = sync.Once{}
	db = nil
	dbw = nil
	cfg.DBPath = "../../data/gemini.db"
	initRuntimeConfig()

	mc := ModelConfig{
		HexID:    "95a4c0abcba24364",
		Mode:     2,
		Thinking: true,
	}

	msgs := []map[string]interface{}{
		{"role": "user", "content": "Hello Argon, introduce yourself in 1 sentence."},
	}

	text, _, res, err := callGeminiConv(msgs, mc, nil, nil, nil, nil)
	t.Logf("Argon callGeminiConv err: %v", err)
	t.Logf("Argon text: %s", text)
	if res != nil {
		t.Logf("Argon UpstreamModel: %s", res.UpstreamModel)
		t.Logf("Argon Reasoning len: %d", len(res.Reasoning))
		t.Logf("Argon Raw len: %d", len(res.Raw))

		matches := upstreamModelRe.FindAllStringSubmatch(res.Raw, -1)
		for idx, match := range matches {
			t.Logf("Argon Match #%d: full=%s, text=%s", idx, match[0], match[1])
		}
	}
}
