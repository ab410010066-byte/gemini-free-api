# Gemini Free API（Go 语言开源版）

> 免费、支持私有化部署的 Google Gemini 转 OpenAI `/v1/chat/completions` 反向代理网关。无需申请 Google 官方付费 API Key 或绑定信用卡。原生支持 Gemini 3.8 Flash、深度思考链流式输出、多模态输入、最高达 50,000 Token 之超长文本文件管道，并具备多账号 Cookie 池与零登出隔离机制。

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [简体中文](README_CN.md)

---

## 这是什么项目？

通俗来说：**这个项目让你能够将 Google 网页版 Gemini，当成完全免费且不限量的 OpenAI API 接口来使用。**

通常情况下，要想在各种客户端软件（如 NextChat、LobeChat、Cherry Studio、Dify、Obsidian）或自己编写的 Python 脚本中使用 AI，往往需要购买官方 API Key 并绑定信用卡按量付费。而 Google 官方的网页版 Gemini（gemini.google.com）虽然免费开放，却不提供任何可供第三方程序调用的开放接口。

**Gemini Free API 就是连接这两者的桥梁：**
1. 它在你的本地电脑或服务器后台运行，提供一个标准的 OpenAI API 接口（`http://localhost:8083/v1`）。
2. 当你的软件（Python、NextChat、Cherry Studio、Obsidian 等）向它提问时，它在后台自动将请求转发给 Google 网页版。
3. 它将网页版的回复实时包装成标准的 OpenAI 流式数据格式返回给你的客户端。

整个过程**不需要任何 Google 官方 API Key、不需要绑定信用卡、完全免费**，并且完整支持模型思考链（Thinking）与图片/文档多模态分析。

```
[你常用的软件] (NextChat / Obsidian / Python / One-API)
        |
        v 发送标准 OpenAI 格式请求 (http://localhost:8083/v1)
[Gemini Free API 网关] (运行在你的本地电脑或服务器)
        |
        v 后台安全代理
[Google Gemini 网页端] (完全免费的 Google 账号会话)
        |
        v 实时流式返回
[你的软件无缝显示 AI 答案]
```

---

## 1. 核心特性与能力

* **完全免费且免 API Key**：告别 Google Cloud 昂贵的计费阶梯，通过网页端反向代理直接获取 Gemini 核心能力。
* **完全兼容 OpenAI API**：无缝接入 One-API、New-API、NextChat、LobeChat、Cherry Studio、Dify 及 Obsidian 等各类主流客户端。
* **深度思考链流式传输**：完整支持 `gemini-3.8-flash-thinking` 与 `gemini-3.1-pro`，实时流式输出 `delta.reasoning_content` 思维过程。
* **50,000 Token 超长上下文管线**：长提示词自动转为 Google 原生文件附件上传，100% 无损、零截断。
* **多账号池与零被踢登出**：基于 Firefox Multi-Account Containers 物理容器隔离，彻底防止会话互相踢下线。
* **严格单账号并发锁**：单账号在途排他锁机制，确保单个 Google 会话绝不同时承受两路并发连接。
* **反爬虫浏览器指纹伪装**：内置 Chrome 146 TLS 与 JA4 握手指纹伪装（基于 `bogdanfinn/tls-client`）。
* **内置单二进制管理面板**：内置账号管理、实时延迟瀑布图与用量监控，零外部 CDN 依赖。

---

## 2. 上下文窗口与 Token 容量规格

| 处理阶段 | 输入字节门槛 | 估计输入 Token 数 | 核心执行机制 | 下游响应特性 |
| :--- | :--- | :--- | :--- | :--- |
| **第一阶：普通提示词** | < 30,000 字节 | ~1 至 8,000 Token | 内联提示词执行，具备多余空白压缩 | 立即响应，极低延迟 |
| **第二阶：文件附件管线** | 30,000 至 155,000 字节 | ~8,000 至 50,000 Token | 自动转化为 Google 原生 `message.txt` 文件附件挂载 | 零截断、100% 无损完整吸收 |
| **第三阶：超限安全熔断** | > 155,000 字节 | > 50,000 Token | 规范返回 HTTP 400（`context_length_exceeded`） | 拒发高危机器人连续探针，保护账号安全 |
| **生成输出上限** | 单次请求限制 | 最大 8,192 输出 Token | OpenAI SSE 流式推送，思维链独立分离 | 完整支持 `reasoning_content` |

---

## 3. 系统架构图

```
[客户端生态系统]
(Python / TypeScript / Go / Rust / cURL / One-API / NextChat / Obsidian)
                       |
                       v  HTTP POST /v1/chat/completions
+-------------------------------------------------------------------------+
|                         Gemini Free API 网关核心                        |
|                                                                         |
|  [入口层与身份鉴权]                                                     |
|    - Bearer Token 签名校验与权限拦截                                    |
|    - OpenAI 数据规范验证与角色上下文重构                                |
|                                                                         |
|  [长上下文引擎（最高支持 50,000 Token）]                                |
|    - 无损空白压缩与重复结构剔除                                        |
|    - 原生 Context 文件附件挂载（<155 KB / ~50,000 Token）              |
|                                                                         |
|  [流量控制与调度锁]                                                     |
|    - 单 IP 出口槽位限流（默认 5 并发，80 RPH 安全边界）                 |
|    - 单账号在途请求保护锁（严格 1 对 1 会话隔离，彻底防止踢出登出）    |
|                                                                         |
|  [上游通讯引擎]                                                         |
|    - Chrome TLS / JA4 握手指纹伪装（bogdanfinn/tls-client）             |
|    - 动态跟随 Google 前端 bl 构建版本                                  |
|    - SSE 流式解析器与 reasoning_content 思考链分离提取                  |
+-------------------------------------------------------------------------+
             |                                           |
             v 本机直接连线                              v 出口代理节点池
+--------------------------------+          +--------------------------------+
| 本机网络网卡                   |          | 代理节点池                     |
| 单 IP（80 RPH 安全阈值）       |          | 多 IP 轮询负载均衡             |
+--------------------------------+          +--------------------------------+
             \                                           /
              \                                         /
               v                                       v
         [Google Gemini 网页端后端（StreamGenerate 协议）]
```

### 3.1 请求生命周期与并发锁机制

```mermaid
flowchart TD
    Req[客户端传入请求] --> Auth{验证 Bearer Token}
    Auth -- 失败 --> E401[HTTP 401 Unauthorized]
    Auth -- 成功 --> Ingest[格式化 OpenAI 历史消息]
    
    Ingest --> LengthCheck{Prompt 长度判定}
    LengthCheck -- "< 30 KB (~8k Token)" --> InflightSlot[获取 IP 出口并发槽位]
    LengthCheck -- "30 KB - 155 KB (~50k Token)" --> MakeDoc[打包为原生 Context 文件附件] --> InflightSlot
    LengthCheck -- "> 155 KB (>50k Token)" --> E400[HTTP 400 Context Length Exceeded]
    
    InflightSlot -- IP 槽位已满 --> E429IP[HTTP 429 触发安全限流]
    InflightSlot -- 槽位成功获取 --> AcctSelect[挑选当前在途数为 0 的空闲账号]
    
    AcctSelect -- 账号全数繁忙 --> WaitAcct{短暂队列等待（最多 6 秒）}
    WaitAcct -- 等待超时且开启 FallbackAnon --> AnonReq[降级走匿名通道发送]
    WaitAcct -- 等待超时且严格模式 --> E429Acct[HTTP 429 账号繁忙]
    WaitAcct -- 账号释放空闲 --> ExecReq[发起 StreamGenerate 请求]
    AcctSelect -- 顺利获取空闲账号 --> ExecReq
    
    ExecReq --> SSETrans[转译为 OpenAI SSE 数据流]
    SSETrans --> RelSlot[释放 IP 槽位与账号在途锁]
    RelSlot --> ClientEnd[客户端流式结束]
```

### 3.2 零登出账号隔离架构

```mermaid
flowchart LR
    subgraph Firefox 多容器环境
        C1[容器 1：账号甲]
        C2[容器 2：账号乙]
        C3[容器 3：账号丙]
    end

    subgraph 本地存储
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph 网关核心
        Poller[后台只读轮询器]
        Pool[(内存账号池)]
        InflightLock[单账号 1 对 1 排他锁]
    end

    C1 -->|浏览器合法后台保活| SQLite
    C2 -->|浏览器合法后台保活| SQLite
    C3 -->|浏览器合法后台保活| SQLite

    SQLite -->|每 10 分钟只读安全同步| Poller
    Poller -->|安全注入| Pool
    Pool --> InflightLock
    InflightLock -->|单一账号绝不同时承受两路请求| Upstream[Google Gemini 网页端]
```

---

## 4. 快速启动指南

### 4.1 单二进制文件运行（推荐）

```bash
# 1. 编译或下载二进制文件
go build -o gemini-free-api main.go

# 2. 启动服务（指定端口与管理口令）
./gemini-free-api --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

浏览器访问 `http://localhost:8083/admin/` 即可进入内置管理面板。

### 4.2 Docker 容器化部署

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

## 5. 主流第三方客户端接入配置

| 客户端软件 | 自定义 API 端点（Base URL） | API Key | 推荐模型名称 |
| :--- | :--- | :--- | :--- |
| **One-API / New-API** | `http://localhost:8083` | `YOUR-ADMIN-PASSWORD` | 渠道类型选 `OpenAI`，填入 `gemini-3.8-flash-thinking` |
| **NextChat / ChatGPT-Next-Web** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` 或 `gemini-3.8-flash-thinking` |
| **LobeChat** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | 自定义 OpenAI 提供商 |
| **Cherry Studio** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI 兼容提供商 |
| **Dify** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI-API-compatible 模型提供商 |
| **Obsidian（Smart Connections）** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` |

---

## 6. 支持模型矩阵表

| 模型标识（Model ID） | 后端 Hex 代码 | 思考链支持（Thinking） | 多模态（图片/文档） | 输出 Token 上限 | 适用任务类型 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | 否 | 是 | 8,192 | 低延迟、高吞吐之快速问答 |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | 是（`reasoning_content`） | 是 | 8,192 | 复杂逻辑推理、深度推演 |
| `gemini-3.7-flash` | `fbb127bbb056c959` | 否 | 是 | 8,192 | 一般日常文本处理与总结 |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | 否 | 是 | 8,192 | 轻量任务、降级兜底执行 |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | 是 | 是 | 8,192 | 深度综合分析与专业领域抽取 |

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

## 8. 推荐生产环境配置

| 部署拓扑 | 账号数量 | 建议并发槽位 | 安全 RPM | 安全 RPH | 运维说明 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **本机直连 IP** | 1 - 5 个账号 | 3 - 5 个槽位 | 15 - 25 | 60 - 80 | 个人开发、知识库插件（Obsidian）之黄金推荐参数。 |
| **单一专用代理** | 1 - 3 个账号 | 3 - 4 个槽位 | 15 - 20 | 60 - 75 | 隔离本机 IP，适合中小型自动化任务。 |
| **分布式代理池** | 5 - 20 个账号 | 10 - 25 个槽位 | 60 - 120 | 300 - 800 | 高吞吐企业级网关，负载分散于多个出口节点。 |

---

## 9. 运行时配置参数表

所有参数均保存在 SQLite 数据库中，可直接在管理面板（`http://localhost:8083/admin/`）动态热更新，无需重启服务。

| 参数名称 | 数据类型 | 默认值 | 推荐安全范围 | 作用说明与稳定性考量 |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | 整数 | `5` | `1 - 10` | 单 IP 出口最大同时在途请求数。超过 10 易触发 Google 空帧拦截。 |
| `per_ip_rpm` | 整数 | `30` | `10 - 60` | 单 IP 每分钟最高请求数。 |
| `per_ip_rph` | 整数 | `80` | `40 - 150` | 单 IP 每小时最高请求数。严格低于 Google 官方 80-180 警戒线。 |
| `max_prompt_bytes` | 整数 | `120000` | `50000 - 150000` | 单次输入提示词之 UTF-8 字节上限。超过此门槛自动转入附件流水线。 |
| `context_compression` | 布尔值 | `true` | `true / false` | 无损剔除提示词中冗余空白字符与重复结构。 |
| `context_file_upload` | 布尔值 | `true` | `true / false` | 超长文本（30 KB - 155 KB / ~50,000 Token）自动转为原生文本文件附件发送。 |
| `fallback_anon` | 布尔值 | `false` | `true / false` | 当账号池全数繁忙时，允许无思考需求之请求临时走匿名通道。 |
| `impersonate` | 字符串 | `chrome_146` | TLS 预设集 | 伪装之浏览器 TLS 与 JA4 握手特征。 |
| `gemini_bl_auto` | 布尔值 | `true` | `true / false` | 每 6 小时自动检查并跟随 Google Web 最新前端构建版本。 |

---

## 10. 命令行参数说明

| 命令行参数 | 环境变量 | 默认值 | 参数功能说明 |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | 网关服务与管理面板监听之 HTTP 端口。 |
| `--admin-token` | `ADMIN_TOKEN` | 自动生成 | 管理面板登录与客户端 API 调用的主鉴权口令。 |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | SQLite 存储路径或 PostgreSQL / MySQL 连接字符串。 |
| `--cookie-file` | `COOKIE_FILE` | （可选） | 自动导入 Netscape 或 JSON 格式之 Cookie 文件路径。 |
| `--proxy` | `PROXY_URL` | （可选） | 静态上游代理 URL。 |

---

## 11. 开源协议

本项目基于 MIT 许可证开源。
仅供学术研究、本地知识库集成与个人开发测试使用。请严格遵守 Google 服务条款。
