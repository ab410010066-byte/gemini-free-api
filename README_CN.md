# Gemini Web2API 网关（Go 语言版）

> 高性能、支持私有化部署的反向代理网关，将 Google Gemini 网页端转换为完全兼容 OpenAI `/v1/chat/completions` 的流式接口。无需 Google 官方付费 API Key，具备严格的单账号并发隔离保护机制以杜绝会话被踢登出，并内置高密度实时监控面板。

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [简体中文](README_CN.md)

---

## 1. 系统架构图

```
[客户端应用]
(Python / TypeScript / Go / Rust / cURL / Obsidian)
                      |
                      v  HTTP POST /v1/chat/completions
+------------------------------------------------------------------------+
|                     Gemini Web2API 网关核心引擎                        |
|                                                                        |
|  [入口层与身份鉴权]                                                    |
|    - Bearer Token 校验与权限拦截                                       |
|    - OpenAI 数据规范验证与角色上下文重构                               |
|                                                                        |
|  [长上下文预处理管线]                                                  |
|    - 无损空白压缩与工具声明瘦身                                       |
|    - 原生文件附件流水线（<155 KB 静态挂载）                           |
|                                                                        |
|  [流量控制与调度器]                                                    |
|    - 单 IP 出口槽位限流（默认 5 并发，80 RPH 防封红线）                |
|    - 单账号在途请求保护锁（严格 1 对 1 会话隔离，彻底防止踢出登出）   |
|                                                                        |
|  [上游通讯引擎]                                                        |
|    - Chrome TLS / JA4 握手指纹伪装（bogdanfinn/tls-client）            |
|    - 动态跟随 Google 前端 bl 构建版本                                 |
|    - SSE 流式解析器与 reasoning_content 思考链分离提取                 |
+------------------------------------------------------------------------+
            |                                           |
            v 本机直接连线                              v 出口代理节点池
+-------------------------------+           +-------------------------------+
| 本机网络网卡                  |           | 代理节点池                    |
| 单 IP（80 RPH 安全阈值）      |           | 多 IP 轮询负载均衡            |
+-------------------------------+           +-------------------------------+
            \                                           /
             \                                         /
              v                                       v
         [Google Gemini 网页端后端（StreamGenerate 协议）]
```

### 1.1 请求生命周期与调度流程

```mermaid
flowchart TD
    Req[客户端传入请求] --> Auth{验证 Bearer Token}
    Auth -- 失败 --> E401[返回 HTTP 401 Unauthorized]
    Auth -- 成功 --> Ingest[格式化 OpenAI 历史消息]
    
    Ingest --> LengthCheck{Prompt 字节长度检查}
    LengthCheck -- "< 30 KB" --> InflightSlot[获取 IP 出口并发槽位]
    LengthCheck -- "30 KB - 155 KB" --> MakeDoc[打包为原生 Context 文件附件] --> InflightSlot
    LengthCheck -- "> 155 KB" --> E400[返回 HTTP 400 Context Length Exceeded]
    
    InflightSlot -- IP 槽位已满 --> E429IP[返回 HTTP 429 触发安全熔断]
    InflightSlot -- 槽位成功获取 --> AcctSelect[挑选当前无在途请求的空闲账号]
    
    AcctSelect -- 账号全数繁忙 --> WaitAcct{短暂队列等待（最多 6 秒）}
    WaitAcct -- 等待超时且开启 FallbackAnon --> AnonReq[降级走匿名通道发送]
    WaitAcct -- 等待超时且严格模式 --> E429Acct[返回 HTTP 429 账号繁忙]
    WaitAcct -- 账号释放空闲 --> ExecReq[发起 StreamGenerate 请求]
    AcctSelect -- 顺利获取空闲账号 --> ExecReq
    
    ExecReq --> SSETrans[转译分块协议为 OpenAI SSE 数据流]
    SSETrans --> RelSlot[释放 IP 槽位与账号在途保护锁]
    RelSlot --> ClientEnd[客户端流式结束]
```

### 1.2 账号隔离与零登出同步架构

传统反向代理在遇到并发或定时刷新时，常因主动向 Google 发送换票请求（`RotateCookies`）导致浏览器端 Token 被判定为重放攻击，进而被 Google 服务器强制注销。本架构在底层彻底解决该隐患：

```mermaid
flowchart LR
    subgraph Firefox 容器环境
        C1[容器 1：账号甲]
        C2[容器 2：账号乙]
        C3[容器 3：账号丙]
    end

    subgraph 操作系统本地存储
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph 网关守护进程
        Poller[后台只读轮询器]
        Pool[(内存账号池)]
        InflightLock[单账号 1 对 1 在途排他锁]
    end

    C1 -->|浏览器合法后台保活| SQLite
    C2 -->|浏览器合法后台保活| SQLite
    C3 -->|浏览器合法后台保活| SQLite

    SQLite -->|每 10 分钟只读安全读取| Poller
    Poller -->|写入更新| Pool
    Pool --> InflightLock
    InflightLock -->|单一账号同一秒绝不并发| Upstream[Google Gemini 网页端]
```

* **禁止主动 RotateCookies**：后端完全停用向 Google 主动发起模拟换票，彻底规避凭据重放风控。
* **严格单账号排他保护**：单个 Google 账号同一时刻严格只处理 1 个在途流式请求，后续请求排队或由其他空闲账号承接，杜绝并发推流导致的 Session 作废。

---

## 2. 核心架构组件说明

| 组件名称 | 核心职责 | 异常恢复与熔断策略 |
| :--- | :--- | :--- |
| **入口引擎（Ingress Engine）** | 提供 OpenAI `/v1/chat/completions`、`/v1/models` 等标准接口。 | 返回标准规范的 JSON 错误定义（`invalid_request_error`、`rate_limit_exceeded`）。 |
| **账号池管理器（Account Fleet）** | 基于 SQLite 管理所有账号凭据、用量额度与绑定代理。 | 连续 3 次鉴权失败自动标记停用，防止失效账号持续拖累集群。 |
| **单账号并发锁（In-Flight Guard）** | 保证单一账号在途连接数不超过 1。 | 遇到并发流量时平滑入队等待（最多 6 秒），或由其他空闲账号分担。 |
| **IP 槽位限流器（IP Slot Limiter）** | 精确控制各 IP 出口的即时并发、RPM 与 RPH。 | 逼近 Google WAF 阈值时自动限流或调度至备用代理节点。 |
| **长上下文预处理器** | 自动剔除冗余格式标记，并将超长提示词转化为附件。 | 超过 155 KB 安全上限直接规范报错 400，避免触发上游封禁。 |
| **监控面板（Telemetry UI）** | 内置管理界面与 Chart.js 性能瀑布图。 | 100% 离线编译于单个可执行文件，零外部网络与 CDN 依赖。 |

---

## 3. 支持模型矩阵表

| 模型标识（Model ID） | 后端 Hex 代码 | 思考链支持（Thinking） | 多模态（图片/文档） | 最大输出 Token | 适用任务类型 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | 否 | 是 | 8,192 | 低延迟、高吞吐之快速对话 |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | 是（`reasoning_content`） | 是 | 8,192 | 复杂逻辑推理、深度推演 |
| `gemini-3.7-flash` | `fbb127bbb056c959` | 否 | 是 | 8,192 | 一般日常文本处理与摘要 |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | 否 | 是 | 8,192 | 轻量任务、降级兜底执行 |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | 是 | 是 | 8,192 | 深度综合分析与专业领域抽取 |

---

## 4. 运行时配置参数表

所有参数均保存在 SQLite 数据库中，可直接在管理面板（`http://localhost:8083/admin/`）动态热更新，无需重启服务。

| 参数名称 | 数据类型 | 默认值 | 推荐安全范围 | 作用说明与稳定性考量 |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | 整数 | `5` | `1 - 10` | 单 IP 出口最大同时在途请求数。超过 10 易触发 Google 空帧拦截。 |
| `per_ip_rpm` | 整数 | `30` | `10 - 60` | 单 IP 每分钟最高请求数。 |
| `per_ip_rph` | 整数 | `80` | `40 - 150` | 单 IP 每小时最高请求数。严格低于 Google 官方 80-180 警戒线。 |
| `max_prompt_bytes` | 整数 | `120000` | `50000 - 150000` | 单次输入提示词之 UTF-8 字节上限。超过此门槛自动转入附件流水线。 |
| `context_compression` | 布尔值 | `true` | `true / false` | 无损剔除提示词中冗余空白字符与重复结构。 |
| `context_file_upload` | 布尔值 | `true` | `true / false` | 超长文本（30 KB - 155 KB）自动转为原生文本文件附件发送。 |
| `fallback_anon` | 布尔值 | `false` | `true / false` | 当账号池全数繁忙时，允许无思考需求的请求临时走匿名通道。 |
| `impersonate` | 字符串 | `chrome_146` | TLS 预设集 | 伪装之浏览器 TLS 与 JA4 握手特征。 |
| `gemini_bl_auto` | 布尔值 | `true` | `true / false` | 每 6 小时自动检查并跟随 Google Web 最新前端构建版本。 |

---

## 5. 生产环境部署与限流调优指南

经压力测试验证，不同网络拓扑之推荐配置如下：

| 部署拓扑 | 账号数量 | 建议并发槽位 | 安全 RPM | 安全 RPH | 运维说明 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **本机直连 IP** | 1 - 5 个账号 | 3 - 5 | 15 - 25 | 60 - 80 | 个人开发、知识库插件（Obsidian）之黄金推荐参数。 |
| **单一专用代理** | 1 - 3 个账号 | 3 - 4 | 15 - 20 | 60 - 75 | 隔离本机真实 IP，适合中小规模自动化任务。 |
| **分布式代理池** | 5 - 20 个账号 | 10 - 25 | 60 - 120 | 300 - 800 | 高吞吐集群部署，将请求平摊至多台代理机器。 |

---

## 6. 上游异常状态与自动恢复对照表

| 上游特征签名 | 客户端状态码 | 根本原因 | 网关自动处理机制 |
| :--- | :--- | :--- | :--- |
| `no SNlM0e in page` | 401 Unauthorized | 账号凭证失效或已被登出 | 累加失败计数，自动熔断并平滑切换至池中下一个可用账号。 |
| `BardErrorInfo [1095]` | 429 Too Many Requests | IP 出口瞬间并发过高或触发限流 | 立即标记该 IP 槽位饱和，暂停调度或切换至替代代理节点。 |
| `302 -> /sorry/index` | 429 Too Many Requests | Google WAF 硬性封禁 IP（通常持续 80-120 分钟） | 代理节点自动触发熔断机制，进入冷却队列。 |
| `PromptTooLongError` | 400 Bad Request | 提示词超出 155 KB 附件安全上限 | 规范返回 `context_length_exceeded`，拒绝发送以防账号被标。 |
| `xsrf token mismatch` | 502 Bad Gateway | 缓存之 XSRF 令牌已过期 | 即刻清除内存缓存，重新抓取最新令牌后自动重试。 |

---

## 7. 各主流编程语言调用示例

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

### 7.3 Go（标准库，零外部依赖）

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

1. 打开 Obsidian 设置面板，进入 **Smart Connections** 或 **Copilot**。
2. 配置 **Custom OpenAI API Endpoint**：`http://localhost:8083/v1`
3. 配置 **API Key**：`sk-gemini-master-key`（或启动时设置的 `--admin-token`）。
4. 配置 **Model Identifier**：`gemini-3.8-flash` 或 `gemini-3.8-flash-thinking`。

---

## 8. 快速部署方式

### 8.1 单二进制文件运行（推荐）

```bash
# 1. 编译二进制可执行文件
go build -o gemini-web2api-go main.go

# 2. 启动服务（指定端口与管理口令）
./gemini-web2api-go --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

浏览器访问 `http://localhost:8083/admin/` 即可进入管理面板。

### 8.2 Docker 容器部署

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

## 9. 命令行参数说明

| 命令行参数 | 环境变量 | 默认值 | 参数功能说明 |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | 网关服务与管理面板监听之 HTTP 端口。 |
| `--admin-token` | `ADMIN_TOKEN` | 自动生成 | 管理面板登录与客户端 API 调用的主鉴权口令。 |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | SQLite 存储路径或 PostgreSQL / MySQL 连接字符串。 |
| `--cookie-file` | `COOKIE_FILE` | （可选） | 自动导入 Netscape 或 JSON 格式之 Cookie 文件路径。 |
| `--proxy` | `PROXY_URL` | （可选） | 静态上游代理 URL。 |

---

## 10. 开源协议

本项目基于 MIT 许可证开源。
仅供学术研究、本地知识库集成与个人开发测试使用。请严格遵守 Google 服务条款。
