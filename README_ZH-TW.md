# Gemini Web2API 閘道器（Go 語言版本）

> 高效能、可私有化部署的反向代理閘道器，將 Google Gemini 網頁端轉換為完全相容 OpenAI `/v1/chat/completions` 的串流 API。無需申請 Google 官方付費 API Key，具備嚴格單帳號並發隔離保護機制以徹底杜絕會話被踢出登出，並內建高密度即時監控面板。

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [簡體中文](README_CN.md)

---

## 1. 系統架構圖

```
[客戶端應用程式]
(Python / TypeScript / Go / Rust / cURL / Obsidian)
                      |
                      v  HTTP POST /v1/chat/completions
+------------------------------------------------------------------------+
|                    Gemini Web2API 閘道核心引擎                         |
|                                                                        |
|  [入口層與身份驗證]                                                    |
|    - Bearer Token 簽名校驗與權限攔截                                   |
|    - OpenAI 資料規格驗證與角色上下文重構                               |
|                                                                        |
|  [長文本前置處理管線]                                                  |
|    - 無損空白壓縮與工具宣告瘦身                                       |
|    - 原生文件附件流水線（<155 KB 靜態掛載）                           |
|                                                                        |
|  [流量控制與調度器]                                                    |
|    - 單 IP 出口槽位限流（預設 5 並發，80 RPH 防封紅線）                |
|    - 單帳號在途請求保護鎖（嚴格 1 對 1 會話隔離，徹底防止踢出登出）   |
|                                                                        |
|  [上游通訊引擎]                                                        |
|    - Chrome TLS / JA4 握手指紋偽裝（bogdanfinn/tls-client）            |
|    - 動態跟隨 Google 前端 bl 構建版本                                 |
|    - SSE 串流解析器與 reasoning_content 思考鏈分離提取                 |
+------------------------------------------------------------------------+
            |                                           |
            v 本機直接連線                              v 出口代理節點池
+-------------------------------+           +-------------------------------+
| 本機網路網卡                  |           | 代理節點池                    |
| 單 IP（80 RPH 安全閾值）      |           | 多 IP 輪詢負載均衡            |
+-------------------------------+           +-------------------------------+
            \                                           /
             \                                         /
              v                                       v
         [Google Gemini 網頁端後端（StreamGenerate 協定）]
```

### 1.1 請求生命週期與調度流程

```mermaid
flowchart TD
    Req[客戶端傳入請求] --> Auth{驗證 Bearer Token}
    Auth -- 失敗 --> E401[返回 HTTP 401 Unauthorized]
    Auth -- 成功 --> Ingest[格式化 OpenAI 歷史訊息]
    
    Ingest --> LengthCheck{Prompt 字節長度檢查}
    LengthCheck -- "< 30 KB" --> InflightSlot[取得 IP 出口並發槽位]
    LengthCheck -- "30 KB - 155 KB" --> MakeDoc[打包為原生 Context 文件附件] --> InflightSlot
    LengthCheck -- "> 155 KB" --> E400[返回 HTTP 400 Context Length Exceeded]
    
    InflightSlot -- IP 槽位已滿 --> E429IP[返回 HTTP 429 觸發安全熔斷]
    InflightSlot -- 槽位成功取得 --> AcctSelect[挑選目前無在途請求的空閒帳號]
    
    AcctSelect -- 帳號全數繁忙 --> WaitAcct{短暫佇列等待（最多 6 秒）}
    WaitAcct -- 等待超時且開啟 FallbackAnon --> AnonReq[降級走匿名通道發送]
    WaitAcct -- 等待超時且嚴格模式 --> E429Acct[返回 HTTP 429 帳號繁忙]
    WaitAcct -- 帳號釋放空閒 --> ExecReq[發起 StreamGenerate 請求]
    AcctSelect -- 順利取得空閒帳號 --> ExecReq
    
    ExecReq --> SSETrans[轉譯區塊協定為 OpenAI SSE 串流]
    SSETrans --> RelSlot[釋放 IP 槽位與帳號在途保護鎖]
    RelSlot --> ClientEnd[客戶端串流結束]
```

### 1.2 帳號隔離與零登出同步架構

傳統反向代理在遇到並發或定時刷新時，常因主動向 Google 發送換票請求（`RotateCookies`）導致瀏覽器端 Token 判定為重放攻擊，進而被 Google 伺服器強制登出。本架構在底層徹底解決此問題：

```mermaid
flowchart LR
    subgraph Firefox 容器環境
        C1[容器 1：帳號甲]
        C2[容器 2：帳號乙]
        C3[容器 3：帳號丙]
    end

    subgraph 作業系統本機儲存
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph 閘道器守護進程
        Poller[背景唯讀輪詢器]
        Pool[(記憶體帳號池)]
        InflightLock[單帳號 1 對 1 在途排他鎖]
    end

    C1 -->|瀏覽器合法背景保活| SQLite
    C2 -->|瀏覽器合法背景保活| SQLite
    C3 -->|瀏覽器合法背景保活| SQLite

    SQLite -->|每 10 分鐘唯讀安全讀取| Poller
    Poller -->|寫入更新| Pool
    Pool --> InflightLock
    InflightLock -->|單一帳號同一秒絕不並發| Upstream[Google Gemini 網頁端]
```

* **禁止主動 RotateCookies**：後端全面停止向 Google 發起模擬刷新請求，防止 Token 搶奪與重放衝突。
* **嚴格單帳號排他保護**：單一 Google 帳號在同一時間只允許處理 1 條請求，後續請求平滑排隊或由其他空閒帳號承接，杜絕多執行緒推流造成的 Session 作廢。

---

## 2. 核心架構元件說明

| 元件名稱 | 核心職責 | 異常恢復與熔斷策略 |
| :--- | :--- | :--- |
| **入口引擎（Ingress Engine）** | 提供 OpenAI `/v1/chat/completions`、`/v1/models` 等標準介面。 | 回傳標準 JSON 錯誤定義（`invalid_request_error`、`rate_limit_exceeded`）。 |
| **帳號池管理器（Account Fleet）** | 以 SQLite 管理所有帳號憑證、用量額度與綁定代理。 | 連續 3 次鑑權失敗自動標記停用，防止無效帳號持續拖累效能。 |
| **單帳號並發鎖（In-Flight Guard）** | 保證單一帳號在途連線數不超過 1。 | 遇高並發請求時自動佇列排隊（最多 6 秒），或由其他空閒帳號分擔。 |
| **IP 槽位限流器（IP Slot Limiter）** | 精確控制各 IP 出口的即時並發、RPM 與 RPH。 | 逼近 Google WAF 風險閾值時自動拒絕或調度至其他代理節點。 |
| **長文本前置處理器** | 自動剔除冗餘格式標記，並將超長提示詞轉化為附件。 | 超過 155 KB 安全極限直接回傳 400，避免觸發上游封禁。 |
| **監控中心（Telemetry UI）** | 內建管理介面與 Chart.js 性能瀑布圖。 | 100% 離線編譯於單一二進位檔案，零外部網路與 CDN 依賴。 |

---

## 3. 支援模型矩陣表

| 模型標識（Model ID） | 後端 Hex 代碼 | 思考鏈支援（Thinking） | 多模態（圖片/文件） | 最大輸出 Token | 適用任務類型 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | 否 | 是 | 8,192 | 低延遲、高吞吐之快速問答 |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | 是（`reasoning_content`） | 是 | 8,192 | 複雜邏輯推理、深度推演 |
| `gemini-3.7-flash` | `fbb127bbb056c959` | 否 | 是 | 8,192 | 一般日常文本處理與總結 |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | 否 | 是 | 8,192 | 輕量任務、降級兜底執行 |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | 是 | 是 | 8,192 | 長篇論述、專業領域萃取 |

---

## 4. 運行時配置參數表

所有參數皆保存在 SQLite 資料庫中，可直接在管理面板（`http://localhost:8083/admin/`）動態熱更新，無需重新啟動服務。

| 參數名稱 | 資料型態 | 預設值 | 推薦安全範圍 | 作用說明與穩定性考量 |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | 整數 | `5` | `1 - 10` | 單 IP 出口最大同時在途請求數。超過 10 容易觸發 Google 空幀攔截。 |
| `per_ip_rpm` | 整數 | `30` | `10 - 60` | 單 IP 每分鐘最高請求數。 |
| `per_ip_rph` | 整數 | `80` | `40 - 150` | 單 IP 每小時最高請求數。低於 Google 官方 80-180 風險警戒線。 |
| `max_prompt_bytes` | 整數 | `120000` | `50000 - 150000` | 單次輸入提示詞之 UTF-8 字節上限。超過此門檻自動轉入附件流水線。 |
| `context_compression` | 布林值 | `true` | `true / false` | 無損剔除提示詞中多餘空白字元與重述結構。 |
| `context_file_upload` | 布林值 | `true` | `true / false` | 超長文本（30 KB - 155 KB）自動轉為原生文本文件附件發送。 |
| `fallback_anon` | 布林值 | `false` | `true / false` | 當帳號池全數繁忙時，允許無思考需求的請求暫時轉走匿名通道。 |
| `impersonate` | 字串 | `chrome_146` | TLS 預設集 | 偽裝之瀏覽器 TLS 與 JA4 握手特徵。 |
| `gemini_bl_auto` | 布林值 | `true` | `true / false` | 每 6 小時自動檢查並跟隨 Google Web 最新前端構建版本。 |

---

## 5. 生產環境部署與限流調校指南

經大規模壓力測試，不同網路拓撲之推薦配置如下：

| 部署拓撲 | 帳號數量 | 建議並發槽位 | 安全 RPM | 安全 RPH | 營運說明 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **本機直連 IP** | 1 - 5 個帳號 | 3 - 5 | 15 - 25 | 60 - 80 | 個人開發、知識庫外掛（Obsidian）之黃金推薦參數。 |
| **單一專用代理** | 1 - 3 個帳號 | 3 - 4 | 15 - 20 | 60 - 75 | 保護本機 IP，適合中小型自動化排程。 |
| **分散式代理池** | 5 - 20 個帳號 | 10 - 25 | 60 - 120 | 300 - 800 | 高吞吐企業級網關，負載分散於多個出口節點。 |

---

## 6. 上游異常狀態與自動恢復對照表

| 上游特徵簽名 | 客戶端狀態碼 | 根本原因 | 閘道器自動處理機制 |
| :--- | :--- | :--- | :--- |
| `no SNlM0e in page` | 401 Unauthorized | 帳號 Cookie 失效或已被登出 | 累加失敗計數，自動剔除並平滑切換至池中下一個可用帳號。 |
| `BardErrorInfo [1095]` | 429 Too Many Requests | IP 出口瞬間並發過高或觸發頻率限制 | 立即標記該 IP 槽位飽和，暫停排程或切換至替代代理節點。 |
| `302 -> /sorry/index` | 429 Too Many Requests | Google WAF 硬性封禁 IP（通常持續 80-120 分鐘） | 代理節點自動觸發熔斷機制，進入冷卻佇列。 |
| `PromptTooLongError` | 400 Bad Request | 提示詞超出 155 KB 附件安全上限 | 規範返回 `context_length_exceeded`，拒絕發送以防帳號遭標記。 |
| `xsrf token mismatch` | 502 Bad Gateway | 快取之 XSRF 權杖已過期 | 即刻清除記憶體快取，重新抓取最新權杖後自動重試。 |

---

## 7. 各主流程式語言整合範例

### 7.1 Python（官方 OpenAI SDK）

```python
import os
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8083/v1",
    api_key=os.environ.get("GEMINI_API_KEY", "sk-gemini-master-key")
)

stream = client.chat.completions.create(
    model="gemini-3.8-flash-thinking",
    messages=[
        {"role": "system", "content": "You are a systems engineering advisor."},
        {"role": "user", "content": "Explain the architectural trade-offs between stateless context replay and stateful server sessions."}
    ],
    temperature=0.2,
    stream=True
)

for chunk in stream:
    delta = chunk.choices[0].delta
    if hasattr(delta, "reasoning_content") and delta.reasoning_content:
        print(f"[Thinking] {delta.reasoning_content}", end="", flush=True)
    elif delta.content:
        print(delta.content, end="", flush=True)
print()
```

### 7.2 TypeScript / Node.js

```typescript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8083/v1",
  apiKey: process.env.GEMINI_API_KEY || "sk-gemini-master-key",
});

async function main() {
  const stream = await client.chat.completions.create({
    model: "gemini-3.8-flash-thinking",
    messages: [{ role: "user", content: "Summarize the ACID properties of relational databases." }],
    stream: true,
  });

  for await (const chunk of stream) {
    const delta = chunk.choices[0]?.delta as any;
    if (delta?.reasoning_content) {
      process.stdout.write(`\x1b[35m${delta.reasoning_content}\x1b[0m`);
    } else if (delta?.content) {
      process.stdout.write(delta.content);
    }
  }
}

main().catch(console.error);
```

### 7.3 Go（標準函式庫，零外部依賴）

```go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

func main() {
	payload := map[string]interface{}{
		"model": "gemini-3.8-flash",
		"messages": []map[string]string{
			{"role": "user", "content": "Output a verified Go concurrency worker pool implementation."},
		},
		"stream": false,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "http://localhost:8083/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-gemini-master-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	io.Copy(os.Stdout, resp.Body)
}
```

### 7.4 Rust（reqwest + tokio）

```rust
use reqwest::header::{HeaderMap, HeaderValue, AUTHORIZATION, CONTENT_TYPE};
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut headers = HeaderMap::new();
    headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
    headers.insert(AUTHORIZATION, HeaderValue::from_static("Bearer sk-gemini-master-key"));

    let client = reqwest::Client::new();
    let res = client
        .post("http://localhost:8083/v1/chat/completions")
        .headers(headers)
        .json(&json!({
            "model": "gemini-3.8-flash",
            "messages": [{"role": "user", "content": "Explain zero-copy deserialization in Rust."}],
            "stream": false
        }))
        .send()
        .await?;

    let text = res.text().await?;
    println!("{}", text);
    Ok(())
}
```

### 7.5 cURL

```bash
curl -X POST http://localhost:8083/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-gemini-master-key" \
  -d '{
    "model": "gemini-3.8-flash",
    "messages": [{"role": "user", "content": "Write a POSIX compliant shell script template."}],
    "stream": true
  }'
```

### 7.6 Obsidian（Smart Connections / Copilot）

1. 開啟 Obsidian 設定面板，進入 **Smart Connections** 或 **Copilot**。
2. 設定 **Custom OpenAI API Endpoint**：`http://localhost:8083/v1`
3. 設定 **API Key**：`sk-gemini-master-key`（或啟動時設定之 `--admin-token`）。
4. 設定 **Model Identifier**：`gemini-3.8-flash` 或 `gemini-3.8-flash-thinking`。

---

## 8. 快速啟動指南

### 8.1 二進位執行檔直接啟動（推薦）

```bash
# 1. 編譯二進位執行檔
go build -o gemini-web2api-go main.go

# 2. 啟動服務（指定連接埠與管理金鑰）
./gemini-web2api-go --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

瀏覽器存取 `http://localhost:8083/admin/` 即可進入管理面板。

### 8.2 Docker 容器化部屬

```bash
docker run -d \
  --name gemini-web2api \
  -p 8083:8083 \
  -v $(pwd)/data:/app/data \
  -e ADMIN_TOKEN=YOUR-ADMIN-PASSWORD \
  --restart unless-stopped \
  zexadev/gemini-web2api-go:latest
```

---

## 9. 命令列參數清單

| 命令列參數 | 環境變數 | 預設值 | 參數說明 |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | 閘道服務與管理面板監聽之 HTTP 連接埠。 |
| `--admin-token` | `ADMIN_TOKEN` | 自動產生 | 管理面板登入與客戶端 API 呼叫之主金鑰。 |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | SQLite 儲存路徑或 PostgreSQL / MySQL 連線字串。 |
| `--cookie-file` | `COOKIE_FILE` | （選填） | 自動匯入 Netscape 或 JSON 格式之 Cookie 檔案路徑。 |
| `--proxy` | `PROXY_URL` | （選填） | 靜態上游代理 URL。 |

---

## 10. 開源授權

本專案基於 MIT 授權條款開源發布。
僅供學術研究、本機知識庫輔助與個人開發測試使用。請嚴格遵守 Google 服務條款。
