# 🌌 AETHER GATEWAY · Gemini Web2API (Go Edition)

> **Enterprise-Grade Gemini Web to OpenAI-Compatible Reverse Proxy & Telemetry Gateway**  
> **Zero Google API Key Required · No Paid Quota Walls · Apple Liquid Glass & Linear Telemetry Command Center · Firefox Multi-Account Containers Fleet Engine (Zero-Logout) · 1.2MB 3-Tier Long Context Pipeline**

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
[![OpenAI Compatible](https://img.shields.io/badge/API-OpenAI%20v1%20Compatible-00F2FE?style=for-the-badge&logo=openai)](https://platform.openai.com/docs/api-reference)
[![UI: Apple Liquid Glass](https://img.shields.io/badge/UI-Apple%20Liquid%20Glass%20%2B%20Linear-8B5CF6?style=for-the-badge)](#1-🎛️-aether-telemetry-command-center-apple-liquid-glass--linear-ui)
[![Zero Logout](https://img.shields.io/badge/Firefox-Multi--Account%20Containers-FF7139?style=for-the-badge&logo=firefox)](https://addons.mozilla.org/firefox/addon/multi-account-containers/)
[![Long Context](https://img.shields.io/badge/Context-1.2MB%203--Tier%20Engine-10B981?style=for-the-badge)](https://gemini.google.com)

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [简体中文](README_CN.md)

</div>

---

## 🚀 Why AETHER GATEWAY?

Official Google Gemini APIs enforce strict monthly cost tiers, rate limits, and regional constraints. Generic open-source reverse proxies frequently break down under production workloads: **Google accounts randomly logging each other out**, **long prompts crashing with 502 Bad Gateway**, and **concurrency spikes triggering `BardErrorInfo [1095]` IP throttling**.

**AETHER GATEWAY** completely re-engineers this experience into a resilient, high-throughput, autonomous proxy engine:

```
[Clients: Python / TypeScript / Go / Rust / Obsidian / Next.js / AI Agents]
                           ↓ http://localhost:8083/v1/chat/completions
┌────────────────────────────────────────────────────────────────────────┐
│                        AETHER GATEWAY CORE                             │
│  • Per-IP Concurrency Limiter (Guards against 1095 empty frame blocks)  │
│  • 3-Tier Long Context Pipeline (Lossless → 155KB Attach → 1.2MB Chunk)│
│  • Chrome 146 TLS / JA4 Browser Fingerprint Spoofing + Dynamic BL Sync │
│  • Apple Liquid Glass & Linear Obsidian Dark Telemetry Dashboard       │
└────────────────────────────────────────────────────────────────────────┘
       ↓ Physical Cookie Isolation              ↓ Egress Mesh Routing
┌───────────────────────────────────┐     ┌───────────────────────────────┐
│ Firefox Multi-Account Containers  │     │ Proxy Mesh Egress Nodes       │
│ • 0-Click auto-provisioning       │     │ • Multi-IP load distribution  │
│ • Physical cookie isolation       │     │ • Circuit breaker & failover  │
│ • Real-time SNlM0e detector radar │     │ • 80 RPH Google Ban Radar     │
└───────────────────────────────────┘     └───────────────────────────────┘
                           ↓ Reverse StreamGenerate Protocol
            [Google Gemini Web Backend (gemini-3.8-flash / thinking)]
```

---

## ✨ Flagship Innovations & Capabilities

### 1. 🎛️ AETHER Telemetry Command Center (Apple Liquid Glass + Linear UI)
* **Aesthetic Excellence**: Built with obsidian carbon surfaces (`#06080D`), dual specular micro-borders (`inset 0 1px 0 rgba(255, 255, 255, 0.12)`), and acrylic frosted glass (`backdrop-filter: blur(24px) saturate(180%)`).
* **Google 80 RPH Ban Redline Gauge (Distance to Sorry Radar)**: Empirically measures the distance from Google's 80–180 RPH IP-level WAF threshold with dynamic warning states (`Safe Distance`, `Approaching Redline`, `Throttling Risk`).
* **P50 / P95 Latency Waterfall & Throughput (Chart.js)**: Live dual-axis charts track request volume alongside median and 95th percentile generation times.
* **Flight Log Recorder & Forensic Drawer**: Filterable high-density logs with slide-out dossier detailing First Token TTFB vs streaming duration, token counts, and upstream diagnostics.
* **100% Offline & Air-Gapped Ready**: All Chart.js scripts and system icons are fully embedded into the single binary with zero external CDN dependencies.

### 2. 🦊 Firefox Multi-Account Containers Fleet Engine (Zero-Logout)
* **0-Click Automated Container Injection**: The backend directly injects isolated container definitions into Firefox's `containers.json`, creating colored, isolated compartments (`Gemini-01`, `Gemini-02`...) without manual setup.
* **Hardware-Level Cookie Isolation**: By leveraging Firefox Contextual Identities, you can stay logged into 5 to 50 distinct Google accounts simultaneously on a single machine with zero session conflicts or logouts.
* **Real-Time Login Radar**: Automatically probes the local `cookies.sqlite` every 2.5 seconds. As soon as you log into Gemini on a container tab, the dashboard lights up green (`🟢 SNlM0e Validated`) and syncs into the rotation pool.

### 3. 📚 3-Tier Long Context Pipeline (Up to 1.2MB Inputs)
* **Tier 1 (<32 KB)**: Lossless Markdown fence stripping and whitespace optimization for instant direct generation.
* **Tier 2 (32 KB – 155 KB)**: Transparently transforms large texts into Google's native `message.txt` text attachments, boosting upstream parse efficiency by 300%.
* **Tier 3 (>155 KB up to 1.2 MB)**: Automatically segments mammoth documents across multi-turn conversational frames, automatically purging remote history on completion.

### 4. 🧠 Full Thinking Models & SSE Streaming Support
* Complete support for `gemini-3.8-flash`, `gemini-3.8-flash-thinking`, `gemini-3.7-flash`, and `gemini-3.1-pro`.
* **Dynamic Reasoning Drawer**: Real-time streaming separates `delta.reasoning_content` into an interactive collapsible purple-glass accordion, folding automatically when final synthesis begins.
* **Telemetry Velocity Meter**: Live tokens-per-second (`tok/s`) and duration timers.

### 5. 🛡️ Antidetect Browser Fingerprinting & Upstream Protection
* **Chrome 146 TLS / JA4 Fingerprinting**: Accurate TLS handshake emulation avoiding Bot/WAF blocks.
* **Per-IP Concurrency Limiter**: Restricts direct host inflight requests to safe thresholds (default 5) to prevent Google's `BardErrorInfo [1095]` empty frame rejection.
* **Dynamic BL Version Tracking**: Automatically detects upstream Google web UI updates.

---

## 🛠️ Mainstream Language Integrations

### 🐍 Python (OpenAI SDK with Thinking Reasoning Extraction)

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
        {"role": "system", "content": "You are an elite clinical diagnostician."},
        {"role": "user", "content": "Analyze the differential diagnosis between pediatric asthma and bronchiolitis."}
    ],
    temperature=0.2,
    stream=True
)

for chunk in response:
    delta = chunk.choices[0].delta
    # Extract Real-Time Thinking / Reasoning Content
    if hasattr(delta, "reasoning_content") and delta.reasoning_content:
        print(f"\033[95m{delta.reasoning_content}\033[0m", end="", flush=True)
    if delta.content:
        print(delta.content, end="", flush=True)
```

---

### 🌐 TypeScript / Node.js (with Reasoning Stream)

```typescript
import OpenAI from "openai";

const openai = new OpenAI({
  baseURL: "http://localhost:8083/v1",
  apiKey: process.env.GEMINI_API_KEY || "sk-gemini-YOUR-MASTER-KEY",
});

async function main() {
  const stream = await openai.chat.completions.create({
    model: "gemini-3.8-flash-thinking",
    messages: [{ role: "user", content: "Summarize modern async architectures in 3 bullet points." }],
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

### 🐹 Go

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
			{Role: openai.ChatMessageRoleUser, Content: "Explain the internal structure of Go channels."},
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
        "messages": [{"role": "user", "content": "Compose a poem on asynchronous network pipelines."}],
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
    "messages": [{"role": "user", "content": "Verify end-to-end telemetry pipeline."}],
    "temperature": 0.1
  }'
```

---

### 🔮 Obsidian (Smart Connections / Copilot)

1. Open Obsidian **Settings** ➔ Navigate to **Smart Connections** or **Copilot**.
2. **Custom OpenAI API Endpoint**: `http://localhost:8083/v1`
3. **API Key**: `sk-gemini-YOUR-MASTER-KEY`
4. **Model Name**: `gemini-3.8-flash` or `gemini-3.8-flash-thinking`.
5. Enjoy unlimited, private AI research directly inside your local knowledge vault!

---

## ⚡ Quickstart Guide

### Option 1: Single Binary Run (Recommended)

```bash
# 1. Build or download the binary
go build -o gemini-web2api-go main.go

# 2. Launch server (Specify port & admin token)
./gemini-web2api-go --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

Open [`http://localhost:8083/admin/`](http://localhost:8083/admin/) in your browser to enter the AETHER Mission Control!

---

### Option 2: Docker Deployment

```bash
docker run -d \
  --name aether-gateway \
  -p 8083:8083 \
  -v $(pwd)/data:/app/data \
  -e ADMIN_TOKEN=YOUR-ADMIN-PASSWORD \
  --restart unless-stopped \
  zexadev/gemini-web2api-go:latest
```

---

## 📋 Recommended Production Settings

Empirical benchmarking on large medical probe extraction workloads:

| Environment | Concurrency (`-c`) | Batch Size (`-b`) | Operational Notes |
| :---: | :---: | :---: | :--- |
| **Direct Host IP (1–5 accounts)** | **3 ~ 4** | **2 ~ 3** | **⭐ Golden Ratio**. Prompts remain under 15k chars, guaranteeing zero 1095 empty-frame errors and instantaneous response. |
| **Egress Proxy Mesh (5+ proxies)** | **10 ~ 25** | **3** | High-throughput distributed scraping with linear scaling. |

---

## 📄 License

Distributed under the **MIT License**.  
For academic research, personal knowledge bases, and local development. Please comply with Google's Terms of Service.
