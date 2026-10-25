# 🌌 AETHER GATEWAY · Gemini Web2API (Go 企业旗舰版)

> **将 Google Gemini 网页端逆向转化为企业级 OpenAI 兼容 API 网关**  
> **免官方 API Key · 零付费额度 · Apple Liquid Glass & Linear 极客遥测面板 · Firefox 多账号容器物理隔离防掉线 · 1.2MB 三阶长文本突破引擎**

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
[![OpenAI Compatible](https://img.shields.io/badge/API-OpenAI%20v1%20Compatible-00F2FE?style=for-the-badge&logo=openai)](https://platform.openai.com/docs/api-reference)
[![UI: Apple Liquid Glass](https://img.shields.io/badge/UI-Apple%20Liquid%20Glass%20%2B%20Linear-8B5CF6?style=for-the-badge)](#1-🎛️-aether-旗舰遥测控制中心-apple-liquid-glass--linear-ui)
[![Zero Logout](https://img.shields.io/badge/Firefox-Multi--Account%20Containers-FF7139?style=for-the-badge&logo=firefox)](https://addons.mozilla.org/firefox/addon/multi-account-containers/)
[![Long Context](https://img.shields.io/badge/Context-1.2MB%203--Tier%20Engine-10B981?style=for-the-badge)](https://gemini.google.com)

[简体中文](README_CN.md) | [English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md)

</div>

---

## 🚀 为什么选择 AETHER GATEWAY？

官方 Google Gemini API 存在严苛的计费门槛与频率限制；而一般的开源逆向项目常遭遇 **Google 账号频繁互踢登出**、**超长 Prompt 触发 502 报错**、**并发过高触发 `BardErrorInfo [1095]` 封禁**、以及**粗糙的后台界面**。

**AETHER GATEWAY** 专为高负载生产环境、大模型智能体、专业知识库抽取与 Obsidian 个人脑库设计：

```
[客户端生态: Python / TypeScript / Go / Rust / Obsidian / Next.js]
                           ↓ http://localhost:8083/v1/chat/completions
┌────────────────────────────────────────────────────────────────────────┐
│                      AETHER GATEWAY 控制核心                           │
│  • 瞬时并发保护阀门 (Per-IP Concurrency Protector) → 防 1095/WAF 阻断    │
│  • 三阶长文本突破引擎 (Lossless → 155KB Attachment → 1.2MB Chunking)  │
│  • Chrome 146 TLS / JA4 真实浏览器指纹伪装 + 动态 BL 版本跟随          │
│  • Apple Liquid Glass & Linear 曜石暗黑遥测控制中心 (0 外网依赖)        │
└────────────────────────────────────────────────────────────────────────┘
       ↓ 实体 Cookie 隔离机队                 ↓ 出口网格分流
┌───────────────────────────────────┐     ┌───────────────────────────────┐
│ Firefox Multi-Account Containers  │     │ 代理节点池 (Proxy Mesh)       │
│ • 后端 0 点击直写 containers.json  │     │ • 实体出口 IP 轮替            │
│ • 多账号平行登录 · 零互踢登出     │     │ • 失败熔断与健康度诊断        │
│ • SQLite SNlM0e 登录态实时雷达    │     │ • 80 RPH Google Ban 红线雷达  │
└───────────────────────────────────┘     └───────────────────────────────┘
                           ↓ 逆向 StreamGenerate 协议
            [Google Gemini 网页端 (gemini-3.8-flash / thinking)]
```

---

## ✨ 核心突破与创新亮点

### 1. 🎛️ AETHER 旗舰遥测控制中心 (Apple Liquid Glass & Linear UI)
* **极致视觉美学**：曜石黑基底（`#06080D`）搭配双层高光微边框（`inset 0 1px 0 rgba(255,255,255,0.12)`）与亚克力雾面玻璃效果（`backdrop-filter: blur(24px)`）。
* **Google 80 RPH 封禁红线雷达 (Distance to Ban Redline)**：以逆向工程实测数据为基准，可视化呈现单出口距离 80~180 RPH WAF 惩罚线的实时安全距离，并动态提供避险指引。
* **P50 / P95 延迟波形图 (Chart.js)**：真实双轴线面图，同时监控请求吞吐量与中位数延迟。
* **飞行日志与全链路取证抽屉 (Forensic Dossier)**：支持实时多条件快搜、分页、RFC 兼容 CSV 导出，以及首字响应时间（TTFB）vs 生成思考耗时瀑布图。
* **零外网依赖**：Chart.js 与所有矢量图标完全本地嵌入，内网或完全隔离环境亦可 100% 正常开启。

### 2. 🦊 Firefox 多账号容器全自动精灵 (Zero-Logout Multi-Account Fleet)
* **0-Click 自动建立容器**：后端直接写入 Firefox 的 `containers.json`，秒级自动生成 5~20 个彩色隔离容器（`Gemini-01`, `Gemini-02`...），免去手动配置！
* **实体 Cookie 隔离 · 零互踢登出**：利用 Firefox Contextual Identities 硬件隔离机制，在同一台机器同时登录数十个 Google 账号，彻底根除多账号 Session 冲突互踢。
* **实时登录态雷达探测器**：每 2.5 秒自动探测本地 `cookies.sqlite`，只要用户在标签页登录 Google，界面立刻点亮绿灯（`🟢 SNlM0e ✅`），支持一键全部收割入库。

### 3. 📚 三阶长文本突破引擎 (1.2MB Ultra-Long Context Pipeline)
突破网页端常规单次输入限制：
* **Tier 1 (小于 32 KB)**：无损空白与 Markdown 优化，直接极速调用。
* **Tier 2 (32 KB ~ 155 KB)**：自动转化为 Google 原生 `message.txt` 附件外挂，上游解析效率提升 300%。
* **Tier 3 (超过 155 KB，最高可达 1.2 MB)**：自动借用 Google 原生会话多轮分切喂入，对话完毕自动清理远端暂存。

### 4. 🧠 完整支持 Thinking 深度思考模型与 SSE 真实流式输出
* 原生支持 `gemini-3.8-flash`、`gemini-3.8-flash-thinking`、`gemini-3.7-flash`、`gemini-3.1-pro`。
* **思考链独立紫晶玻璃抽屉**：自动将上游 `delta.reasoning_content` 实时注入打字机效果，推演完毕自动优雅收折。
* **实时 Token 速率监控**：实时计算每秒生成速率（`tok/s`）与首字反应延迟。

### 5. 🛡️ 浏览器真实指纹伪装与防刷风控防御
* **Chrome 146 TLS / JA4 指纹伪装**：模拟最新版本 Chrome 握手特征。
* **单 IP 瞬时并发保护 (`per_ip_concurrent`)**：严格限制本机在途请求（默认 5），避免触发 Google `BardErrorInfo [1095]` 空内容帧拒绝。
* **自动动态跟随前端构建版本 (Dynamic BL Tracking)**。

---

## 🛠️ 主流语言接入代码示例

### 🐍 Python (OpenAI SDK + 实时思考链提取)

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
        {"role": "system", "content": "你是一位顶尖的架构专家。"},
        {"role": "user", "content": "请深度剖析现代高并发异步微服务管道的设计模式。"}
    ],
    temperature=0.2,
    stream=True
)

for chunk in response:
    delta = chunk.choices[0].delta
    if hasattr(delta, "reasoning_content") and delta.reasoning_content:
        print(f"\033[95m{delta.reasoning_content}\033[0m", end="", flush=True)
    if delta.content:
        print(delta.content, end="", flush=True)
```

---

### 🌐 TypeScript / Node.js (含思考链流式输出)

```typescript
import OpenAI from "openai";

const openai = new OpenAI({
  baseURL: "http://localhost:8083/v1",
  apiKey: process.env.GEMINI_API_KEY || "sk-gemini-YOUR-MASTER-KEY",
});

async function main() {
  const stream = await openai.chat.completions.create({
    model: "gemini-3.8-flash-thinking",
    messages: [{ role: "user", content: "用 3 句话介绍现代异步架构的优势。" }],
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

### 🐹 Go 语言

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
			{Role: openai.ChatMessageRoleUser, Content: "请简述 Go channel 的底层数据结构。"},
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
        "messages": [{"role": "user", "content": "请用一首五言绝句赞美异步系统之美。"}],
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

---

### 💻 cURL

```bash
curl -X POST http://localhost:8083/v1/chat/completions \
  -H "Authorization: Bearer sk-gemini-YOUR-MASTER-KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3.8-flash",
    "messages": [{"role": "user", "content": "测试端到端链路"}],
    "temperature": 0.1
  }'
```

---

### 🔮 Obsidian (Smart Connections / Copilot 接入)

1. 打开 Obsidian 设置 ➔ 进入 **Smart Connections** 或 **Copilot** 插件。
2. **Custom OpenAI API Endpoint** 填写：`http://localhost:8083/v1`
3. **API Key** 填写：`sk-gemini-YOUR-MASTER-KEY`
4. **Model Name** 填写：`gemini-3.8-flash` 或 `gemini-3.8-flash-thinking`。
5. 点击测试连接即可无限制接入 Google 顶尖模型作为个人知识大脑！

---

## ⚡ 快速启动指南

### 方式 A: 单二进制直接运行 (推荐)

```bash
# 1. 编译或下载二进制
go build -o gemini-web2api-go main.go

# 2. 启动服务 (指定端口与管理面板 Admin Token)
./gemini-web2api-go --port 8083 --admin-token 你的管理密码
```

启动后在浏览器打开 [`http://localhost:8083/admin/`](http://localhost:8083/admin/) 即可进入 AETHER 控制中心！

---

### 方式 B: Docker 部署

```bash
docker run -d \
  --name aether-gateway \
  -p 8083:8083 \
  -v $(pwd)/data:/app/data \
  -e ADMIN_TOKEN=你的管理密码 \
  --restart unless-stopped \
  zexadev/gemini-web2api-go:latest
```

---

## 📋 生产环境推荐参数调优

实测黄金比例调优矩阵：

| 运行环境 | 推荐并发 (`-c`) | 批次合并 (`-b`) | 调优说明 |
| :---: | :---: | :---: | :--- |
| **单 IP 直连 (1~5 个账号)** | **3 ~ 4** | **2 ~ 3** | **⭐ 黄金平衡点**。Prompt 保持在 15K 字符以内，保证 0 报错秒回，绝不踩踏单 IP 并发上限（5）。 |
| **多代理节点池 (5+ 节点)** | **10 ~ 25** | **3** | 出口 IP 充分分散，吞吐量可线性扩展 5 倍以上。 |

---

## 📄 开源许可证
本项目遵循 **MIT License**。
仅供学术研究、本地自动化与个人知识库学习使用，请遵守 Google 官方服务条款。
