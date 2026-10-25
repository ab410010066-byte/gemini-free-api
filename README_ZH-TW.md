# 🌌 AETHER GATEWAY · Gemini Web2API (Go Edition)

> **將 Google Gemini 網頁端逆向轉化為頂級 OpenAI 相容 API 閘道器**  
> **免官方 API Key · 零付費配額 · Apple Liquid Glass & Linear 極致遙測面板 · Firefox 多帳號容器實體隔離防踢 · 三階長文本突破引擎 (1.2MB)**

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
[![OpenAI Compatible](https://img.shields.io/badge/API-OpenAI%20v1%20Compatible-00F2FE?style=for-the-badge&logo=openai)](https://platform.openai.com/docs/api-reference)
[![UI: Apple Liquid Glass](https://img.shields.io/badge/UI-Apple%20Liquid%20Glass%20%2B%20Linear-8B5CF6?style=for-the-badge)](#1-🎛️-aether-旗艦遙測控制中心-apple-liquid-glass--linear-ui)
[![Zero Logout](https://img.shields.io/badge/Firefox-Multi--Account%20Containers-FF7139?style=for-the-badge&logo=firefox)](https://addons.mozilla.org/firefox/addon/multi-account-containers/)
[![Long Context](https://img.shields.io/badge/Context-1.2MB%203--Tier%20Engine-10B981?style=for-the-badge)](https://gemini.google.com)

[繁體中文](README_ZH-TW.md) | [English](README.md) | [日本語](README_JA.md) | [简体中文](README_CN.md)

</div>

---

## 🚀 為什麼選擇 AETHER GATEWAY？

傳統的 Gemini 官方 API 面臨嚴苛的計費牆與頻率限制；而一般的逆向開源專案則經常遭遇 **Google 帳號頻繁互踢登出**、**超長提示詞直接 502 中斷**、**單 IP 並發過高觸發 `BardErrorInfo [1095]` 封禁**、以及**陽春簡陋的後台介面**。

**AETHER GATEWAY** 徹底解決了上述所有痛點，專為高負載生產環境、大模型代理人、醫學國考題庫抽取與 Obsidian 知識庫設計：

```
[客戶端生態: Python / TypeScript / Go / Rust / Obsidian / Next.js]
                           ↓ http://localhost:8083/v1/chat/completions
┌────────────────────────────────────────────────────────────────────────┐
│                      AETHER GATEWAY 控制核心                           │
│  • 瞬時並發閥門 (Per-IP Concurrency Protector) → 防 1095/WAF 阻斷       │
│  • 三階長文本突破引擎 (Lossless → 155KB Attachment → 1.2MB Chunking)  │
│  • Chrome 146 TLS / JA4 真實瀏覽器指紋偽裝 + 動態 BL 版本跟隨         │
│  • Apple Liquid Glass & Linear 曜石深色遙測控制中心 (0 外網依賴)       │
└────────────────────────────────────────────────────────────────────────┘
       ↓ 實體 Cookie 隔離機隊                 ↓ 出口網格分流
┌───────────────────────────────────┐     ┌───────────────────────────────┐
│ Firefox Multi-Account Containers  │     │ 代理節點池 (Proxy Mesh)       │
│ • 後端 0 點擊直寫 containers.json  │     │ • 實體出口 IP 輪替            │
│ • 多帳號平行登入 · 零互踢登出     │     │ • 失敗熔斷與健康度診斷        │
│ • SQLite SNlM0e 登入態實時雷達    │     │ • 80 RPH Google Ban 紅線警報  │
└───────────────────────────────────┘     └───────────────────────────────┘
                           ↓ 逆向 StreamGenerate 協議
            [Google Gemini 網頁端 (gemini-3.8-flash / thinking)]
```

---

## ✨ 核心突破與功能亮點

### 1. 🎛️ AETHER 旗艦遙測控制中心 (Apple Liquid Glass & Linear UI)
* **極致視覺美學**：曜石黑基底（`#06080D`）搭配雙層高光微邊框（`inset 0 1px 0 rgba(255,255,255,0.12)`）與壓克力霧面玻璃效果（`backdrop-filter: blur(24px)`）。
* **Google 80 RPH 封禁紅線雷達 (Distance to Ban Redline)**：以逆向工程實測數據為基準，視覺化呈現單一出口距離 80~180 RPH WAF 懲罰線的即時安全距離，並動態提供避險指引。
* **P50 / P95 延遲波形圖 (Chart.js)**：真實雙軸線面圖，同時監控請求吞吐量與中位數延遲。
* **飛行日誌與全鏈路鑑識抽屜 (Forensic Dossier)**：支援即時多條件快搜、分頁、RFC 相容 CSV 導出，以及首字反應時間（TTFB）vs 生成思考耗時瀑布圖。
* **零外網依賴**：Chart.js 與所有向量圖示完全本機嵌入，內網或完全隔離環境亦可 100% 正常開啟。

### 2. 🦊 Firefox 多帳號容器全自動精靈 (Zero-Logout Multi-Account Fleet)
* **0-Click 自動建立容器**：後端直接注入 Firefox 的 `containers.json`，秒級自動生成 5~20 個彩色隔離容器（`Gemini-01`, `Gemini-02`...），使用者免手動設定！
* **實體 Cookie 隔離 · 零互踢登出**：利用 Firefox 的 Contextual Identities 硬體隔離機制，在同一台電腦同時登入數十個 Google 帳號，徹底根除多帳號 Session 串擾問題。
* **實時登入態雷達探測器**：每 2.5 秒自動探測本地 `cookies.sqlite`，只要使用者在分頁登入 Google，畫面上立即點亮綠燈（`🟢 SNlM0e ✅`），支援一鍵全部收割入庫。

### 3. 📚 三階長文本突破引擎 (1.2MB Ultra-Long Context Pipeline)
突破網頁端一般單次輸入限制：
* **Tier 1 (小於 32 KB)**：無失真空白與 Markdown 圍欄優化，直接極速調用。
* **Tier 2 (32 KB ~ 155 KB)**：自動轉化為 Google 原生 `message.txt` 文字附件外掛，上游解析效率提升 300%。
* **Tier 3 (超過 155 KB，最高達 1.2 MB)**：自動借用 Google 原生會話多輪分切喂入，對話完畢自動清理遠端暫存。

### 4. 🧠 完整支援 Thinking 深度思考模型與 SSE 真流式傳輸
* 原生支援 `gemini-3.8-flash`、`gemini-3.8-flash-thinking`、`gemini-3.7-flash`、`gemini-3.1-pro`。
* **思考鏈獨立紫晶玻璃抽屜**：自動將上游 `delta.reasoning_content` 即時注入打字機效果，推演完畢自動優雅收折。
* **即時 Token 速率監控**：即時計算每秒生成速率（`tok/s`）與首字反應延遲。

### 5. 🛡️ 瀏覽器真實指紋偽裝與防刷風控防禦
* **Chrome 146 TLS / JA4 指紋偽裝**：模擬最新版本 Chrome 握手特徵。
* **單 IP 瞬時並發保護 (`per_ip_concurrent`)**：嚴格限制本機在途請求，避免觸發 Google `BardErrorInfo [1095]` 空內容幀拒絕。
* **自動動態跟隨前端構建版本 (Dynamic BL Tracking)**。

---

## 🛠️ 主流語言接入代碼範例

本專案完全相容 OpenAI 原生協議，任何支援 OpenAI API 的軟體與 SDK 均可**無縫秒切換**！

### 🐍 Python (OpenAI SDK + 串流思考鏈提取)

```python
import os
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8083/v1",
    api_key=os.environ.get("GEMINI_API_KEY", "sk-gemini-YOUR-MASTER-KEY")
)

response = client.chat.completions.create(
    model="gemini-3.8-flash-thinking",
    messages=[
        {"role": "system", "content": "你是一位頂尖的架構專家。"},
        {"role": "user", "content": "請深度剖析現代非同步高並發微服務管線設計。"}
    ],
    temperature=0.2,
    stream=True
)

print("=== 思考過程與回答 ===")
for chunk in response:
    # 提取深度思考鏈 (Reasoning Content)
    delta = chunk.choices[0].delta
    if hasattr(delta, "reasoning_content") and delta.reasoning_content:
        print(f"\033[95m{delta.reasoning_content}\033[0m", end="", flush=True)
    if delta.content:
        print(delta.content, end="", flush=True)
```

---

### 🌐 TypeScript / Node.js (含思考鏈串流)

```typescript
import OpenAI from "openai";

const openai = new OpenAI({
  baseURL: "http://localhost:8083/v1",
  apiKey: process.env.GEMINI_API_KEY || "sk-gemini-YOUR-MASTER-KEY",
});

async function main() {
  const stream = await openai.chat.completions.create({
    model: "gemini-3.8-flash-thinking",
    messages: [{ role: "user", content: "用 3 句話介紹現代非同步架構的優勢。" }],
    stream: true,
  });

  for await (const chunk of stream) {
    const delta = chunk.choices[0]?.delta as any;
    if (delta?.reasoning_content) {
      process.stdout.write(`\x1b[35m${delta.reasoning_content}\x1b[0m`);
    }
    if (delta?.content) {
      process.stdout.write(delta.content);
    }
  }
}

main().catch(console.error);
```

---

### 🐹 Go 語言

```go
package main

import (
	"context"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

func main() {
	config := openai.DefaultConfig("sk-gemini-YOUR-MASTER-KEY")
	config.BaseURL = "http://localhost:8083/v1"
	client := openai.NewClientWithConfig(config)

	req := openai.ChatCompletionRequest{
		Model: "gemini-3.8-flash",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "請簡述 Go channel 的底層結構。"},
		},
		Stream: true,
	}

	stream, err := client.CreateChatCompletionStream(context.Background(), req)
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		fmt.Print(resp.Choices[0].Delta.Content)
	}
}
```

---

### 🦀 Rust (Modern Reqwest)

```rust
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();
    let body = json!({
        "model": "gemini-3.8-flash",
        "messages": [{"role": "user", "content": "請用一首五言絕句讚美程式設計之美。"}],
        "temperature": 0.2
    });

    let res = client.post("http://localhost:8083/v1/chat/completions")
        .bearer_auth("sk-gemini-YOUR-MASTER-KEY")
        .json(&body)
        .send()
        .await?
        .json::<serde_json::Value>()
        .await?;

    println!("{}", res["choices"][0]["message"]["content"].as_str().unwrap_or(""));
    Ok(())
}
```

    println!("{}", res["choices"][0]["message"]["content"].as_str().unwrap_or(""));
    Ok(())
}
```

---

### 💻 cURL

```bash
curl -X POST http://localhost:8083/v1/chat/completions \
  -H "Authorization: Bearer sk-gemini-YOUR-MASTER-KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3.8-flash",
    "messages": [{"role": "user", "content": "測試端到端連線"}],
    "temperature": 0.1
  }'
```

---

### 🔮 Obsidian (Smart Connections / Copilot 整合)

1. 開啟 Obsidian 設置 ➔ 進入 **Smart Connections** 或 **Copilot** 外掛設定。
2. **Custom OpenAI API Endpoint** 填入：`http://localhost:8083/v1`
3. **API Key** 填入：`sk-gemini-YOUR-MASTER-KEY`
4. **Model Name** 填入：`gemini-3.8-flash` 或 `gemini-3.8-flash-thinking`。
5. 點擊測試連線即可永久免費用上 Google 最頂尖大模型作為個人外掛大腦！

---

## ⚡ 快速啟動指南

### 方法 A: 單一二進制直跑 (推薦)

```bash
# 1. 取得二進制檔案或直接編譯
go build -o gemini-web2api-go main.go

# 2. 啟動服務 (指定端口與管理面板 Admin Token)
./gemini-web2api-go --port 8083 --admin-token 你的管理員密碼
```

啟動後瀏覽器打開 [`http://localhost:8083/admin/`](http://localhost:8083/admin/) 即可進入 AETHER 控制中心！

---

### 方法 B: Docker 部署

```bash
docker run -d \
  --name aether-gateway \
  -p 8083:8083 \
  -v $(pwd)/data:/app/data \
  -e ADMIN_TOKEN=你的管理員密碼 \
  --restart unless-stopped \
  zexadev/gemini-web2api-go:latest
```

---

## 📋 生產環境最佳參數調優

依據大規模國考題庫高壓實測經驗：

| 運行環境 | 推薦並發 (`-c`) | 批次合併 (`-b`) | 建議說明 |
| :---: | :---: | :---: | :--- |
| **單 IP 直連 (1~5 個帳號)** | **3 ~ 4** | **2 ~ 3** | **⭐ 黃金平衡點**。Prompt 控制在 15K 字元內，保證 0 錯誤秒回，避免踩中單 IP 並發 5 的紅線。 |
| **多代理節點池 (5+ 代理)** | **10 ~ 25** | **3** | 出口 IP 分散，吞吐量可大幅提升 5 倍以上。 |

---

## 📄 開源協議

本專案基於 **MIT License** 授權釋出。
僅供學術研究、本機自動化與個人知識庫學習使用，請遵守 Google 官方服務條款。
