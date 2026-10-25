# Gemini Free API（Go 語言開源版）

> 免費、支援私有化部署的 Google Gemini 轉 OpenAI `/v1/chat/completions` 反向代理閘道器。無需申請 Google 官方付費 API Key 或綁定信用卡。原生支援 Gemini 3.8 Flash、深度思考鏈串流、多模態輸入、最高達 50,000 Token 之超長文本文件管道，並具備多帳號 Cookie 池與零登出隔離機制。

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [簡體中文](README_CN.md)

---

## 這是什麼專案？

簡單來說：**這個專案讓你可以把 Google 網頁版 Gemini，當成完全免費且不限次數的 OpenAI API 來使用。**

一般情況下，如果要讓寫程式、AI 聊天工具（如 NextChat、LobeChat、Cherry Studio）或筆記軟體（如 Obsidian）使用 AI，通常需要向官方購買 API Key 並綁定信用卡計費。而 Google 官方的網頁版 Gemini（gemini.google.com）雖然可以免費使用，卻沒有提供任何 API 接口供外部軟體串接。

**Gemini Free API 就是這兩者之間的橋樑：**
1. 它在你的本機或伺服器背景運行，模擬出一個標準的 OpenAI API 接口（`http://localhost:8083/v1`）。
2. 當任何軟體（Python 程式、NextChat、Cherry Studio、Obsidian 等）向它發送問題時，它會在後台自動向 Google 網頁版發送請求。
3. 它把網頁版的回覆即時包裝成標準的 OpenAI 串流回傳給你的軟體。

整個過程**不需要任何 Google API Key、不需要綁定信用卡、完全免費**，並且支援模型思考過程（Thinking）與多模態圖片/文件分析。

```
[你平常使用的軟體] (NextChat / Obsidian / Python / One-API)
        |
        v 發送標準 OpenAI 請求 (http://localhost:8083/v1)
[Gemini Free API 閘道器] (在你的電腦或伺服器本機運行)
        |
        v 背景安全代理
[Google Gemini 網頁端] (完全免費的 Google 帳號會話)
        |
        v 即時串流回傳
[你的軟體順暢顯示 AI 回答]
```

---

## 1. 核心特點與能力

* **完全免費且免 API Key**：告別 Google Cloud 昂貴的計費階梯，透過網頁端反向代理直接取得 Gemini 核心能力。
* **完全相容 OpenAI API**：無縫接入 One-API、New-API、NextChat、LobeChat、Cherry Studio、Dify 及 Obsidian 等各類主流客戶端。
* **深度思考鏈串流**：完整支援 `gemini-3.8-flash-thinking` 與 `gemini-3.1-pro`，即時串流輸出 `delta.reasoning_content` 思維過程。
* **50,000 Token 超長文本管線**：長提示詞自動轉為 Google 原生文件附件上傳，100% 無損、零截斷。
* **多帳號池與零被踢登出**：基於 Firefox Multi-Account Containers 物理容器隔離，徹底防止會話互相踢下線。
* **嚴格單帳號並發鎖**：單帳號在途排他鎖機制，確保單一 Google 會話絕不同時承受兩路並發連線。
* **反爬蟲瀏覽器指紋偽裝**：內建 Chrome 146 TLS 與 JA4 握手指紋偽裝（基於 `bogdanfinn/tls-client`）。
* **內建單一二進位管理面板**：內建帳號管理、即時延遲瀑布圖與用量監控，零外部 CDN 依賴。

---

## 2. 上下文視窗與 Token 容量規格

| 處理階段 | 輸入字節門檻 | 估計輸入 Token 數 | 核心執行機制 | 下游響應特性 |
| :--- | :--- | :--- | :--- | :--- |
| **第一階：普通提示詞** | < 30,000 字節 | ~1 至 8,000 Token | 內聯提示詞執行，具備多餘空白壓縮 | 立即響應，極低延遲 |
| **第二階：文件附件管線** | 30,000 至 155,000 字節 | ~8,000 至 50,000 Token | 自動轉化為 Google 原生 `message.txt` 文件附件掛載 | 零截斷、100% 無損完整吸收 |
| **第三階：超限安全熔斷** | > 155,000 字節 | > 50,000 Token | 規範回傳 HTTP 400（`context_length_exceeded`） | 拒發高危機器人連續探針，保護帳號安全 |
| **生成輸出上限** | 單次請求限制 | 最大 8,192 輸出 Token | OpenAI SSE 串流推送，思維鏈獨立分離 | 完整支援 `reasoning_content` |

---

## 3. 系統架構圖

```
[客戶端生態系統]
(Python / TypeScript / Go / Rust / cURL / One-API / NextChat / Obsidian)
                       |
                       v  HTTP POST /v1/chat/completions
+-------------------------------------------------------------------------+
|                         Gemini Free API 閘道核心                        |
|                                                                         |
|  [入口層與身分鑑權]                                                     |
|    - Bearer Token 簽名校驗與權限攔截                                    |
|    - OpenAI 數據規格驗證與角色上下文重構                                |
|                                                                         |
|  [長文本引擎（最高支援 50,000 Token）]                                  |
|    - 無損空白壓縮與重複結構剔除                                        |
|    - 原生 Context 文件附件掛載（<155 KB / ~50,000 Token）              |
|                                                                         |
|  [流量控制與調度鎖]                                                     |
|    - 單 IP 出口槽位限流（預設 5 並發，80 RPH 安全邊界）                 |
|    - 單帳號在途請求保護鎖（嚴格 1 對 1 會話隔離，徹底防止踢出登出）    |
|                                                                         |
|  [上游通訊引擎]                                                         |
|    - Chrome TLS / JA4 握手指紋偽裝（bogdanfinn/tls-client）             |
|    - 動態跟隨 Google 前端 bl 構建版本                                  |
|    - SSE 串流解析器與 reasoning_content 思考鏈分離提取                  |
+-------------------------------------------------------------------------+
             |                                           |
             v 本機直接連線                              v 出口代理節點池
+--------------------------------+          +--------------------------------+
| 本機網路網卡                   |          | 代理節點池                     |
| 單 IP（80 RPH 安全閾值）       |          | 多 IP 輪詢負載均衡             |
+--------------------------------+          +--------------------------------+
             \                                           /
              \                                         /
               v                                       v
         [Google Gemini 網頁端後端（StreamGenerate 協定）]
```

### 3.1 請求生命週期與並發鎖機制

```mermaid
flowchart TD
    Req[客戶端傳入請求] --> Auth{驗證 Bearer Token}
    Auth -- 失敗 --> E401[HTTP 401 Unauthorized]
    Auth -- 成功 --> Ingest[格式化 OpenAI 歷史訊息]
    
    Ingest --> LengthCheck{Prompt 長度判定}
    LengthCheck -- "< 30 KB (~8k Token)" --> InflightSlot[取得 IP 出口並發槽位]
    LengthCheck -- "30 KB - 155 KB (~50k Token)" --> MakeDoc[打包為原生 Context 文件附件] --> InflightSlot
    LengthCheck -- "> 155 KB (>50k Token)" --> E400[HTTP 400 Context Length Exceeded]
    
    InflightSlot -- IP 槽位已滿 --> E429IP[HTTP 429 觸發安全限流]
    InflightSlot -- 槽位成功取得 --> AcctSelect[挑選目前在途數為 0 的空閒帳號]
    
    AcctSelect -- 帳號全數繁忙 --> WaitAcct{短暫佇列等待（最多 6 秒）}
    WaitAcct -- 等待超時且開啟 FallbackAnon --> AnonReq[降級走匿名通道發送]
    WaitAcct -- 等待超時且嚴格模式 --> E429Acct[HTTP 429 帳號繁忙]
    WaitAcct -- 帳號釋放空閒 --> ExecReq[發起 StreamGenerate 請求]
    AcctSelect -- 順利取得空閒帳號 --> ExecReq
    
    ExecReq --> SSETrans[轉譯為 OpenAI SSE 串流]
    SSETrans --> RelSlot[釋放 IP 槽位與帳號在途鎖]
    RelSlot --> ClientEnd[客戶端串流結束]
```

### 3.2 零登出帳號隔離架構

```mermaid
flowchart LR
    subgraph Firefox 多容器環境
        C1[容器 1：帳號甲]
        C2[容器 2：帳號乙]
        C3[容器 3：帳號丙]
    end

    subgraph 本地儲存
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph 閘道器核心
        Poller[背景唯讀輪詢器]
        Pool[(記憶體帳號池)]
        InflightLock[單帳號 1 對 1 排他鎖]
    end

    C1 -->|瀏覽器合法背景保活| SQLite
    C2 -->|瀏覽器合法背景保活| SQLite
    C3 -->|瀏覽器合法背景保活| SQLite

    SQLite -->|每 10 分鐘唯讀安全同步| Poller
    Poller -->|安全注入| Pool
    Pool --> InflightLock
    InflightLock -->|單一帳號絕不同時承受兩路請求| Upstream[Google Gemini 網頁端]
```

---

## 4. 快速啟動指南

### 4.1 二進位執行檔直接啟動（推薦）

```bash
# 1. 編譯或下載二進位檔案
go build -o gemini-free-api main.go

# 2. 啟動服務（指定連接埠與管理金鑰）
./gemini-free-api --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

瀏覽器存取 `http://localhost:8083/admin/` 即可進入內建管理面板。

### 4.2 Docker 容器化部屬

```bash
docker run -d \
  --name gemini-free-api \
  -p 8083:8083 \
  -v $(pwd)/data:/app/data \
  -e ADMIN_TOKEN=YOUR-ADMIN-PASSWORD \
  --restart unless-stopped \
  zexadev/gemini-web2api-go:latest
```

---

## 5. 主流第三方客戶端接入設定

| 客戶端軟體 | 自訂 API 端點（Base URL） | API Key | 推薦模型名稱 |
| :--- | :--- | :--- | :--- |
| **One-API / New-API** | `http://localhost:8083` | `YOUR-ADMIN-PASSWORD` | 渠道類型選 `OpenAI`，填入 `gemini-3.8-flash-thinking` |
| **NextChat / ChatGPT-Next-Web** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` 或 `gemini-3.8-flash-thinking` |
| **LobeChat** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | 自訂 OpenAI 提供商 |
| **Cherry Studio** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI 相容提供商 |
| **Dify** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI-API-compatible 模型提供商 |
| **Obsidian（Smart Connections）** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` |

---

## 6. 支援模型矩陣表

| 模型標識（Model ID） | 後端 Hex 代碼 | 思考鏈支援（Thinking） | 多模態（圖片/文件） | 輸出 Token 上限 | 適用任務類型 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | 否 | 是 | 8,192 | 低延遲、高吞吐之快速問答 |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | 是（`reasoning_content`） | 是 | 8,192 | 複雜邏輯推理、深度推演 |
| `gemini-3.7-flash` | `fbb127bbb056c959` | 否 | 是 | 8,192 | 一般日常文本處理與總結 |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | 否 | 是 | 8,192 | 輕量任務、降級兜底執行 |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | 是 | 是 | 8,192 | 深度綜合分析與專業領域抽取 |

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
        {"role": "system", "content": "You are a senior software architect."},
        {"role": "user", "content": "Explain how single-turn stateless context prevents session invalidation."}
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
    messages: [{ role: "user", content: "Summarize the ACID properties of databases." }],
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
			{"role": "user", "content": "Write a concurrent worker pool in Go."},
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

---

## 8. 推薦生產環境配置

| 部署拓撲 | 帳號數量 | 建議並發槽位 | 安全 RPM | 安全 RPH | 營運說明 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **本機直連 IP** | 1 - 5 個帳號 | 3 - 5 個槽位 | 15 - 25 | 60 - 80 | 個人開發、知識庫外掛（Obsidian）之黃金推薦參數。 |
| **單一專用代理** | 1 - 3 個帳號 | 3 - 4 個槽位 | 15 - 20 | 60 - 75 | 保護本機 IP，適合中小型自動化排程。 |
| **分散式代理池** | 5 - 20 個帳號 | 10 - 25 個槽位 | 60 - 120 | 300 - 800 | 高吞吐企業級網關，負載分散於多個出口節點。 |

---

## 9. 運行時配置參數表

所有參數皆保存在 SQLite 資料庫中，可直接在管理面板（`http://localhost:8083/admin/`）動態熱更新，無需重新啟動服務。

| 參數名稱 | 資料型態 | 預設值 | 推薦安全範圍 | 作用說明與穩定性考量 |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | 整數 | `5` | `1 - 10` | 單 IP 出口最大同時在途請求數。超過 10 容易觸發 Google 空幀攔截。 |
| `per_ip_rpm` | 整數 | `30` | `10 - 60` | 單 IP 每分鐘最高請求數。 |
| `per_ip_rph` | 整數 | `80` | `40 - 150` | 單 IP 每小時最高請求數。嚴格低於 Google 官方 80-180 風險警戒線。 |
| `max_prompt_bytes` | 整數 | `120000` | `50000 - 150000` | 單次輸入提示詞之 UTF-8 字節上限。超過此門檻自動轉入附件流水線。 |
| `context_compression` | 布林值 | `true` | `true / false` | 無損剔除提示詞中多餘空白字元與重述結構。 |
| `context_file_upload` | 布林值 | `true` | `true / false` | 超長文本（30 KB - 155 KB / ~50,000 Token）自動轉為原生文本文件附件發送。 |
| `fallback_anon` | 布林值 | `false` | `true / false` | 當帳號池全數繁忙時，允許無思考需求的請求暫時轉走匿名通道。 |
| `impersonate` | 字串 | `chrome_146` | TLS 預設集 | 偽裝之瀏覽器 TLS 與 JA4 握手特徵。 |
| `gemini_bl_auto` | 布林值 | `true` | `true / false` | 每 6 小時自動檢查並跟隨 Google Web 最新前端構建版本。 |

---

## 10. 命令列參數清單

| 命令列參數 | 環境變數 | 預設值 | 參數說明 |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | 閘道服務與管理面板監聽之 HTTP 連接埠。 |
| `--admin-token` | `ADMIN_TOKEN` | 自動產生 | 管理面板登入與客戶端 API 呼叫之主金鑰。 |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | SQLite 儲存路徑或 PostgreSQL / MySQL 連線字串。 |
| `--cookie-file` | `COOKIE_FILE` | （選填） | 自動匯入 Netscape 或 JSON 格式之 Cookie 檔案路徑。 |
| `--proxy` | `PROXY_URL` | （選填） | 靜態上游代理 URL。 |

---

## 11. 開源授權

本專案基於 MIT 授權條款開源發布。
僅供學術研究、本機知識庫輔助與個人開發測試使用。請嚴格遵守 Google 服務條款。
