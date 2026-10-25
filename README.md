# Gemini Web2API Gateway (Go Edition)

> High-performance, self-hosted reverse proxy transforming Google Gemini Web into an OpenAI-compatible `/v1/chat/completions` streaming API. Requires no Google API key, enforces per-account concurrency isolation to prevent session invalidation, and features embedded real-time telemetry.

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [簡體中文](README_CN.md)

---

## 1. System Architecture

```
[Client Applications]
(Python / TypeScript / Go / Rust / cURL / Obsidian)
                      |
                      v  HTTP POST /v1/chat/completions
+------------------------------------------------------------------------+
|                      Gemini Web2API Gateway Core                       |
|                                                                        |
|  [Ingress & Authentication]                                            |
|    - Bearer Token Verification                                         |
|    - OpenAI Schema Validation & Role Normalization                     |
|                                                                        |
|  [Long Context Preprocessing]                                          |
|    - Lossless Whitespace & Tool Schema Compression                     |
|    - Native Context File Attachment (<155 KB)                          |
|                                                                        |
|  [Traffic Control & Concurrency Schedulers]                            |
|    - Per-IP Slot Limiter (Default 5 In-Flight, 80 RPH Google Ban Guard)|
|    - Per-Account In-Flight Guard (Strict 1-to-1 Concurrency Isolation) |
|                                                                        |
|  [Upstream Engine]                                                     |
|    - Chrome TLS / JA4 Handshake Impersonation (bogdanfinn/tls-client)   |
|    - Dynamic BL Version Resolution                                     |
|    - SSE Stream Parser & reasoning_content Extraction                  |
+------------------------------------------------------------------------+
            |                                           |
            v Direct Host Egress                        v Outbound Proxy Mesh
+-------------------------------+           +-------------------------------+
| Direct Network Interface      |           | Egress Proxy Pool             |
| Single IP (80 RPH Safe Limit) |           | Multi-IP Load Balancing       |
+-------------------------------+           +-------------------------------+
            \                                           /
             \                                         /
              v                                       v
         [Google Gemini Web Backend (StreamGenerate Protocol)]
```

### 1.1 Request Processing Pipeline

```mermaid
flowchart TD
    Req[Incoming Client Request] --> Auth{Validate Bearer Token}
    Auth -- Invalid --> E401[Return HTTP 401 Unauthorized]
    Auth -- Valid --> Ingest[Normalize OpenAI Messages]
    
    Ingest --> LengthCheck{Prompt Length}
    LengthCheck -- "< 30 KB" --> InflightSlot[Acquire IP Concurrency Slot]
    LengthCheck -- "30 KB - 155 KB" --> MakeDoc[Upload as Native Context File] --> InflightSlot
    LengthCheck -- "> 155 KB" --> E400[Return HTTP 400 Context Length Exceeded]
    
    InflightSlot -- All IP Slots Full --> E429IP[Return HTTP 429 Rate Limit Exceeded]
    InflightSlot -- Slot Granted --> AcctSelect[Select Account with Inflight == 0]
    
    AcctSelect -- All Accounts Busy --> WaitAcct{Wait up to 6s for Idle Account}
    WaitAcct -- Timeout & FallbackAnon --> AnonReq[Route via Anonymous Session]
    WaitAcct -- Timeout & Strict --> E429Acct[Return HTTP 429 Account Busy]
    WaitAcct -- Account Released --> ExecReq[Execute StreamGenerate]
    AcctSelect -- Account Granted --> ExecReq
    
    ExecReq --> SSETrans[Transform Chunked Protocol to SSE delta]
    SSETrans --> RelSlot[Release IP Slot & Account Inflight]
    RelSlot --> ClientEnd[Client Stream Complete]
```

### 1.2 Account Isolation and Zero-Logout Synchronization

Standard reverse proxies frequently encounter Google account disconnections due to concurrent session collisions and conflicting background cookie refreshes. Gemini Web2API resolves this at the architecture level:

```mermaid
flowchart LR
    subgraph Firefox Environment
        C1[Container 1: Account A]
        C2[Container 2: Account B]
        C3[Container 3: Account C]
    end

    subgraph Host OS Storage
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph Gateway Process
        Poller[Background SQLite Reader]
        Pool[(In-Memory Account Pool)]
        InflightLock[Strict 1-to-1 Account Inflight Guard]
    end

    C1 -->|Legitimate Browser Refresh| SQLite
    C2 -->|Legitimate Browser Refresh| SQLite
    C3 -->|Legitimate Browser Refresh| SQLite

    SQLite -->|Read-Only Query Every 10m| Poller
    Poller -->|Safe Update| Pool
    Pool --> InflightLock
    InflightLock -->|Max 1 Request per Session| Upstream[Google Gemini Web]
```

* **No Active RotateCookies Calls**: The gateway never issues synthetic `POST accounts.google.com/RotateCookies` calls, preventing Google Token Replay Attack alerts.
* **Strict 1-to-1 Concurrency**: Only one request is permitted per Google account session at any instant. Concurrent client requests are automatically balanced across other idle accounts or queued cleanly.

---

## 2. Core Architectural Components

| Component | Responsibility | Failure Remediation |
| :--- | :--- | :--- |
| **Ingress Engine** | Serves OpenAI `/v1/chat/completions`, `/v1/models`, and Responses endpoints. | Standardized JSON error responses (`invalid_request_error`, `rate_limit_exceeded`). |
| **Account Fleet Manager** | Tracks account health, usage quotas, and proxies in SQLite. | Automatic circuit-breaker after 3 consecutive authentication failures. |
| **Account In-Flight Guard** | Restricts each Google account to at most 1 concurrent streaming connection. | Queues subsequent requests up to 6 seconds; safely balances traffic. |
| **IP Slot Limiter** | Enforces per-IP concurrency, RPM, and RPH thresholds. | Returns 429 when safe Google WAF rate boundaries are approached. |
| **Context Preprocessor** | Strips redundant whitespace and serializes large payloads into native document attachments. | Rejects payloads exceeding 155 KB with 400 instead of risking upstream bans. |
| **Telemetry Command Center** | Embedded administrative interface with Chart.js performance metrics. | Fully embedded in the single binary; zero CDN or external network dependencies. |

---

## 3. Supported Model Matrix

| Model Identifier | Backend Hex ID | Thinking Output | Vision / Docs | Max Tokens | Primary Use Case |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | No | Yes | 8,192 | Low latency, high throughput dialogue |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | Yes (`reasoning_content`) | Yes | 8,192 | Complex reasoning, chain-of-thought analysis |
| `gemini-3.7-flash` | `fbb127bbb056c959` | No | Yes | 8,192 | General text transformation and summarization |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | No | Yes | 8,192 | Lightweight tasks, fallback execution |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | Yes | Yes | 8,192 | Deep analytical synthesis and domain extraction |

---

## 4. Configuration and Runtime Parameters

All runtime parameters are stored in SQLite and can be modified live through the administration interface (`http://localhost:8083/admin/`) without restarting the service.

| Parameter Key | Data Type | Default | Safe Range | Description & Stability Impact |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | Integer | `5` | `1 - 10` | Maximum simultaneous requests per egress IP. Exceeding 10 triggers Google WAF blocks. |
| `per_ip_rpm` | Integer | `30` | `10 - 60` | Maximum requests per minute per egress IP. |
| `per_ip_rph` | Integer | `80` | `40 - 150` | Maximum requests per hour per egress IP. Lower bound of Google rate-limit threshold. |
| `max_prompt_bytes` | Integer | `120000` | `50000 - 150000` | Maximum input prompt size in UTF-8 bytes. Payloads beyond this trigger the attachment pipeline. |
| `context_compression` | Boolean | `true` | `true / false` | Lossless removal of redundant whitespace and repetitive prompt scaffolding. |
| `context_file_upload` | Boolean | `true` | `true / false` | Automatically attaches large prompts (30 KB - 155 KB) as native text documents. |
| `fallback_anon` | Boolean | `false` | `true / false` | When all accounts are busy, routes non-thinking queries anonymously to preserve availability. |
| `impersonate` | String | `chrome_146` | TLS Profiles | Browser TLS and JA4 signature emulation profile. |
| `gemini_bl_auto` | Boolean | `true` | `true / false` | Automatically tracks upstream Google Web build tags every 6 hours. |

---

## 5. Production Concurrency and Safety Guidelines

Empirical stress testing under production workloads yields the following parameter configurations:

| Network Topology | Active Accounts | Concurrency Slots | Safe RPM | Safe RPH | Operational Assessment |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Direct Host IP** | 1 - 5 Accounts | 3 - 5 | 15 - 25 | 60 - 80 | Recommended baseline for personal or small team usage. |
| **Single Dedicated Proxy** | 1 - 3 Accounts | 3 - 4 | 15 - 20 | 60 - 75 | Isolates host IP; robust long-term stability. |
| **Distributed Proxy Mesh** | 5 - 20 Accounts | 10 - 25 | 60 - 120 | 300 - 800 | High-volume deployment across distributed proxy nodes. |

---

## 6. Upstream Error Handling State Machine

| Upstream Condition | HTTP Status | Root Cause | Automatic Gateway Action |
| :--- | :--- | :--- | :--- |
| `no SNlM0e in page` | 401 Unauthorized | Account session expired or logged out | Increments `fail_count`; switches to next idle account in pool. |
| `BardErrorInfo [1095]` | 429 Too Many Requests | IP-level burst concurrency or rate limit exceeded | Marks IP slot saturated; rejects or diverts traffic to alternate proxy. |
| `302 -> /sorry/index` | 429 Too Many Requests | Google WAF IP block (typically 80-120 minute ban) | Triggers circuit breaker on proxy node; enters cooldown state. |
| `PromptTooLongError` | 400 Bad Request | Payload exceeds 155 KB attachment limit | Returns clear `context_length_exceeded` error; prevents bot spam. |
| `xsrf token mismatch` | 502 Bad Gateway | Cached XSRF token stale | Automatically clears token cache and retries request immediately. |

---

## 7. Programming Language Integration Examples

### 7.1 Python (OpenAI Official SDK)

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
        {"role": "user", "content": "Explain the trade-offs of single-turn stateless context versus stateful server sessions."}
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

### 7.6 Obsidian (Smart Connections & Copilot)

1. Open Obsidian Settings and select **Smart Connections** or **Copilot**.
2. Configure **Custom OpenAI API Endpoint**: `http://localhost:8083/v1`
3. Configure **API Key**: `sk-gemini-master-key` (or matching `--admin-token` / configured key).
4. Configure **Model Identifier**: `gemini-3.8-flash` or `gemini-3.8-flash-thinking`.

---

## 8. Deployment Options

### 8.1 Single Binary (Recommended)

```bash
# 1. Compile binary
go build -o gemini-web2api-go main.go

# 2. Launch gateway server
./gemini-web2api-go --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

Access the administration dashboard at `http://localhost:8083/admin/`.

### 8.2 Docker Container

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

## 9. CLI Flags and Options

| CLI Flag | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | HTTP listening port for gateway and admin UI. |
| `--admin-token` | `ADMIN_TOKEN` | Generated | Master authentication token for administration API and client requests. |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | Storage path for SQLite or connection string for PostgreSQL / MySQL. |
| `--cookie-file` | `COOKIE_FILE` | (Optional) | Path to raw Netscape or JSON cookie export for auto-seeding accounts. |
| `--proxy` | `PROXY_URL` | (Optional) | Static upstream proxy fallback URL. |

---

## 10. License

Distributed under the MIT License.
For research, local application development, and personal knowledge systems. Ensure compliance with Google Terms of Service.
