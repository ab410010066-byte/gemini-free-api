package app

import (
	"encoding/json"
	"testing"
)

func TestParseUsageHTML_Pro(t *testing.T) {
	sampleHTML := `"SNlM0e":"mock_token"<script type="application/json" data-bg3-relay-preload nonce="abc">{"events":[{"id":"usage:0","kind":"chunk","chunk":"[{\"data\":{\"viewer\":{\"rpcs\":{\"getUserStatus\":{\"featureGates\":[\"CAN_ENROLL_IN_AUREUS\"]},\"getUsageInfo\":{\"subscriptionTier\":\"TIER1\",\"pools\":[{\"poolType\":\"GXU_FIVE_HOURLY\",\"percentageUsed\":0.1134,\"gxu\":{\"windowRefreshesAt\":\"2026-10-18T16:51:56.304466Z\"}},{\"poolType\":\"GXU_WEEKLY\",\"percentageUsed\":0.1702,\"gxu\":{\"windowRefreshesAt\":\"2026-10-23T14:51:56.304608Z\"}}],\"advancedSettings\":null,\"useOverageAiCredits\":false},\"id\":\"ViewerRpcs__viewer_rpcs_v1\"}}}}]"}]}</script>`

	matches := preloadScriptRe.FindAllSubmatch([]byte(sampleHTML), -1)
	if len(matches) == 0 {
		t.Fatalf("no matches found")
	}
	var pData relayPreloadData
	if err := json.Unmarshal(matches[0][1], &pData); err != nil {
		t.Fatalf("pdata unmarshal failed: %v", err)
	}
	var items []relayChunkItem
	if err := json.Unmarshal([]byte(pData.Events[0].Chunk), &items); err != nil {
		t.Fatalf("chunk unmarshal failed: %v", err)
	}

	q, err := parseUsageHTML(101, []byte(sampleHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Tier != "PRO" {
		t.Errorf("expected Tier=PRO, got %s", q.Tier)
	}
	if q.FiveHourlyPct < 11.2 || q.FiveHourlyPct > 11.4 {
		t.Errorf("expected FiveHourlyPct ~ 11.3, got %f", q.FiveHourlyPct)
	}
	if q.WeeklyPct < 17.0 || q.WeeklyPct > 17.1 {
		t.Errorf("expected WeeklyPct ~ 17.0, got %f", q.WeeklyPct)
	}
	if q.FiveHourlyReset != "2026-10-18T16:51:56.304466Z" {
		t.Errorf("unexpected reset time: %s", q.FiveHourlyReset)
	}
}

func TestParseUsageHTML_Free(t *testing.T) {
	sampleHTML := `"SNlM0e":"mock_token"<script type="application/json" data-bg3-relay-preload nonce="abc">{"events":[{"id":"usage:0","kind":"chunk","chunk":"[{\"data\":{\"viewer\":null},\"errors\":[{\"message\":\"Internal error encountered.\",\"errorType\":\"DATA_FETCHING_EXCEPTION\",\"path\":[\"viewer\"],\"extensions\":{\"status\":{\"code\":13,\"message\":\"Internal error encountered.\"}}}],\"path\":[]}]"}]}</script>`
	q, err := parseUsageHTML(102, []byte(sampleHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Tier != "Free" {
		t.Errorf("expected Tier=Free, got %s", q.Tier)
	}
}
