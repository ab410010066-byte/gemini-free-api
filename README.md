# Gemini Free API

> Free, self-hosted reverse proxy converting Google Gemini Web into standard OpenAI-compatible API (`/v1/chat/completions`). Zero official Google API keys or credit cards required. Supports Gemini 3.8 Flash, native thinking reasoning stream, multimodal inputs, long context document ingestion up to 50,000 tokens, and multi-account cookie pools with zero-logout isolation.

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [简体中文](README_CN.md)

---

## What is Gemini Free API?

In plain terms: **This tool lets you use Google Gemini's free web interface just like a paid, standard OpenAI API — completely free of charge.**

Normally, connecting AI to your favorite tools (like NextChat, LobeChat, Cherry Studio, Dify, Obsidian, or custom Python scripts) requires an official API key from OpenAI or Google Cloud, which demands credit cards, billing accounts, and per-token fees. On the other hand, Google's web chat (gemini.google.com) is free to use, but lacks an open API for third-party software.

**Gemini Free API bridges that gap:**
1. It runs silently on your local computer or server, exposing a standard OpenAI-compatible API endpoint (`http://localhost:8083/v1`).
2. When your app sends a standard chat prompt, this gateway forwards it to Google Gemini Web in the background.
3. It converts the incoming response into standard OpenAI streaming format (`/v1/chat/completions`) and delivers it back to your app in real-time.

**Zero Google Cloud setup, zero credit cards, zero API fees.** You get access to Gemini 3.8 Flash, complex thinking reasoning streams, and vision/document inputs through any OpenAI-compatible client.

```
[Your Favorite Apps] (NextChat / Obsidian / Python / One-API)
         |
         v Standard OpenAI Request (http://localhost:8083/v1)
[Gemini Free API Gateway] (Runs on your machine / server)
         |
         v Automated Web Proxy
[Google Gemini Web] (Free Google Account Session)
         |
         v Smooth Streaming Response
[AI Answers Rendered in Your App]
```

---

## 1. Overview & Key Capabilities

* **100% Free & No API Key**: Replaces costly official Gemini API subscriptions with self-hosted web reverse proxying.
* **Full OpenAI Compatibility**: Seamless drop-in replacement for OpenAI SDK, One-API, New-API, NextChat, LobeChat, Cherry Studio, Dify, and Obsidian.
* **Deep Reasoning Thought Stream**: Real-time streaming for `gemini-3.8-flash-thinking` and `gemini-3.1-pro` with `delta.reasoning_content` reasoning output.
* **50,000 Tokens Long Context Engine**: Transparently converts long prompts into native Google document attachments, lossless and without truncation.
* **Multi-Account Fleet with Zero Logout**: Physical cookie isolation via Firefox Multi-Account Containers, avoiding session invalidation and account logouts.
* **Strict Concurrency Protection**: Per-account in-flight mutex guards ensure no two concurrent requests share the same Google session.
* **Anti-Bot Browser Fingerprinting**: Built on Chrome 146 TLS / JA4 fingerprint emulation (`bogdanfinn/tls-client`).
* **Self-Contained Web Dashboard**: Single binary embeds management UI, real-time latency waterfall charts, and quota tracking with zero external CDN dependencies.

---

## 2. Context Window & Token Capacity

| Processing Tier | Input Byte Threshold | Estimated Input Tokens | Execution Mechanism | Downstream Behavior |
| :--- | :--- | :--- | :--- | :--- |
| **Tier 1: Direct Prompt** | < 30,000 bytes | ~1 to 8,000 tokens | Inline prompt execution with whitespace optimization | Instant response, minimal latency |
| **Tier 2: Document Pipeline** | 30,000 to 155,000 bytes | ~8,000 to 50,000 tokens | Transparent upload as native `message.txt` document attachment | Zero truncation, 100% lossless ingestion |
| **Tier 3: Oversize Guard** | > 155,000 bytes | > 50,000 tokens | Immediate HTTP 400 rejection (`context_length_exceeded`) | Prevents account flagging from bot-like chunks |
| **Completion Output** | Per request limit | Max 8,192 output tokens | Streaming SSE delivery with reasoning separation | Full `reasoning_content` support |

---

## 3. System Architecture

```
[Client Ecosystem]
(Python / TypeScript / Go / Rust / cURL / One-API / NextChat / Obsidian)
                       |
                       v  HTTP POST /v1/chat/completions
+-------------------------------------------------------------------------+
|                         Gemini Free API Gateway                         |
|                                                                         |
|  [Ingress & Authentication]                                             |
|    - Bearer Token Verification                                          |
|    - OpenAI Schema Validation & Role Normalization                      |
|                                                                         |
|  [Long Context Engine (Up to 50,000 Tokens)]                            |
|    - Lossless Whitespace Optimization                                   |
|    - Native Context File Attachment (<155 KB / ~50,000 Tokens)          |
|                                                                         |
|  [Traffic Control & Session Guards]                                     |
|    - Per-IP Slot Limiter (Default 5 In-Flight, 80 RPH Safe Bound)       |
|    - Per-Account In-Flight Lock (Strict 1-to-1 Concurrency Isolation)   |
|                                                                         |
|  [Upstream Engine]                                                      |
|    - Chrome TLS / JA4 Handshake Spoofing (bogdanfinn/tls-client)        |
|    - Dynamic BL Build Tracking                                          |
|    - SSE Stream Parser & reasoning_content Extraction                   |
+-------------------------------------------------------------------------+
             |                                           |
             v Direct Host Connection                    v Proxy Mesh Nodes
+--------------------------------+          +--------------------------------+
| Direct Network Interface       |          | Egress Proxy Pool              |
| Single IP (80 RPH Safe Limit)  |          | Multi-IP Load Balancing        |
+--------------------------------+          +--------------------------------+
             \                                           /
              \                                         /
               v                                       v
         [Google Gemini Web Backend (StreamGenerate Protocol)]
```

### 3.1 Request Lifecycle & Concurrency Guard

```mermaid
flowchart TD
    Req[Incoming Client Request] --> Auth{Validate Bearer Token}
    Auth -- Invalid --> E401[HTTP 401 Unauthorized]
    Auth -- Valid --> Ingest[Normalize OpenAI Messages]
    
    Ingest --> LengthCheck{Prompt Length}
    LengthCheck -- "< 30 KB (~8k tokens)" --> InflightSlot[Acquire IP Concurrency Slot]
    LengthCheck -- "30 KB - 155 KB (~50k tokens)" --> MakeDoc[Upload as Native Context Document] --> InflightSlot
    LengthCheck -- "> 155 KB (>50k tokens)" --> E400[HTTP 400 Context Length Exceeded]
    
    InflightSlot -- IP Slots Saturated --> E429IP[HTTP 429 Rate Limit Exceeded]
    InflightSlot -- Slot Granted --> AcctSelect[Select Account with Inflight == 0]
    
    AcctSelect -- All Accounts Busy --> WaitAcct{Wait up to 6s for Idle Account}
    WaitAcct -- Timeout & FallbackAnon --> AnonReq[Route via Anonymous Session]
    WaitAcct -- Timeout & Strict --> E429Acct[HTTP 429 Account Busy]
    WaitAcct -- Account Released --> ExecReq[Execute StreamGenerate]
    AcctSelect -- Account Granted --> ExecReq
    
    ExecReq --> SSETrans[Transform to OpenAI SSE Stream]
    SSETrans --> RelSlot[Release IP Slot & Account Inflight]
    RelSlot --> ClientEnd[Client Stream Complete]
```

### 3.2 Zero-Logout Account Fleet Model

```mermaid
flowchart LR
    subgraph Firefox Multi-Account Containers
        C1[Container 1: Account A]
        C2[Container 2: Account B]
        C3[Container 3: Account C]
    end

    subgraph Local Storage
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph Gateway Core
        Poller[Background SQLite Reader]
        Pool[(In-Memory Account Pool)]
        InflightLock[Strict 1-to-1 Account Lock]
    end

    C1 -->|Legitimate Browser Renewal| SQLite
    C2 -->|Legitimate Browser Renewal| SQLite
    C3 -->|Legitimate Browser Renewal| SQLite

    SQLite -->|Read-Only Sync Every 10m| Poller
    Poller -->|Safe Ingest| Pool
    Pool --> InflightLock
    InflightLock -->|Max 1 Request per Session| Upstream[Google Gemini Web]
```

---

## 4. Quickstart Guide

### 4.1 Single Binary (Recommended)

```bash
# 1. Compile or download binary
go build -o gemini-free-api main.go

# 2. Start server
./gemini-free-api --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

Open `http://localhost:8083/admin/` to access the built-in management dashboard.

### 4.2 Docker Container

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

## 5. Third-Party Client Configurations

| Application | Custom API Endpoint | API Key | Model Identifier |
| :--- | :--- | :--- | :--- |
| **One-API / New-API** | `http://localhost:8083` | `YOUR-ADMIN-PASSWORD` | Select channel type `OpenAI`, add `gemini-3.8-flash-thinking` |
| **NextChat / ChatGPT-Next-Web** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` or `gemini-3.8-flash-thinking` |
| **LobeChat** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | Custom OpenAI model provider |
| **Cherry Studio** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI compatible provider |
| **Dify** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI-API-compatible model provider |
| **Obsidian (Smart Connections)** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` |

---

## 6. Supported Models Matrix

| Model Identifier | Backend Hex ID | Thinking Output | Vision / Multimodal | Output Token Limit | Primary Use Case |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | No | Yes | 8,192 | Low latency, high throughput dialogue |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | Yes (`reasoning_content`) | Yes | 8,192 | Complex reasoning, chain-of-thought analysis |
| `gemini-3.7-flash` | `fbb127bbb056c959` | No | Yes | 8,192 | General text transformation and summarization |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | No | Yes | 8,192 | Lightweight tasks, fallback execution |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | Yes | Yes | 8,192 | Deep analytical synthesis and domain extraction |

---

## 7. Programming Language Examples

### 7.1 Python (Official OpenAI SDK)

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

### 7.3 Go (Standard Library Only)

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

### 7.4 Rust (reqwest + tokio)

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

## 8. Recommended Production Settings

| Topology | Account Fleet Size | Recommended In-Flight Slots | Safe RPM | Safe RPH | Operational Guidance |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Direct Host IP** | 1 - 5 Accounts | 3 - 5 Slots | 15 - 25 | 60 - 80 | Standard baseline for personal workflows and knowledge base plugins. |
| **Dedicated Proxy** | 1 - 3 Accounts | 3 - 4 Slots | 15 - 20 | 60 - 75 | Hides host IP, suitable for small team automation pipelines. |
| **Proxy Mesh Cluster** | 5 - 20 Accounts | 10 - 25 Slots | 60 - 120 | 300 - 800 | High-throughput distributed deployment across multiple proxy nodes. |

---

## 9. Configuration Parameters

All parameters are stored in SQLite and can be modified live through the administration interface (`http://localhost:8083/admin/`) without restarting the server.

| Parameter Key | Data Type | Default | Safe Range | Functional Description |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | Integer | `5` | `1 - 10` | Maximum simultaneous requests per egress IP slot. Exceeding 10 triggers Google WAF blocks. |
| `per_ip_rpm` | Integer | `30` | `10 - 60` | Maximum requests per minute per egress IP slot. |
| `per_ip_rph` | Integer | `80` | `40 - 150` | Maximum requests per hour per egress IP slot. Enforces safe Google rate limits. |
| `max_prompt_bytes` | Integer | `120000` | `50000 - 150000` | Prompt byte size threshold before triggering the document attachment pipeline. |
| `context_compression` | Boolean | `true` | `true / false` | Lossless compression of repetitive whitespace and prompt structures. |
| `context_file_upload` | Boolean | `true` | `true / false` | Ingests prompts between 30 KB and 155 KB (~50,000 tokens) as native document attachments. |
| `fallback_anon` | Boolean | `false` | `true / false` | Allows non-thinking requests to route anonymously when all accounts are busy. |
| `impersonate` | String | `chrome_146` | TLS Profiles | Browser TLS and JA4 signature emulation profile. |
| `gemini_bl_auto` | Boolean | `true` | `true / false` | Automatically synchronizes upstream Google Web build tags every 6 hours. |

---

## 10. CLI Options

| CLI Flag | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | HTTP listening port for gateway and admin dashboard. |
| `--admin-token` | `ADMIN_TOKEN` | Generated | Master authentication key for administration API and client requests. |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | Storage path for SQLite or connection string for PostgreSQL / MySQL. |
| `--cookie-file` | `COOKIE_FILE` | (Optional) | Path to raw Netscape or JSON cookie export for auto-seeding accounts. |
| `--proxy` | `PROXY_URL` | (Optional) | Static upstream proxy fallback URL. |

---

## 11. License

Distributed under the MIT License.
For research, local application development, and personal knowledge systems. Ensure compliance with Google Terms of Service.
