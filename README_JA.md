# 🌌 AETHER GATEWAY · Gemini Web2API (Go 日本語旗艦版)

> **Google Gemini Web版を高品質なOpenAI互換APIゲートウェイに変換**  
> **Google公式APIキー不要 · 無料クォータ無制限 · Apple Liquid Glass ＆ Linear 遠隔測定ダッシュボード · Firefox Multi-Account Containers 物理隔離（ログアウトゼロ） · 1.2MB 3段階長文突破パイプライン**

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
[![OpenAI Compatible](https://img.shields.io/badge/API-OpenAI%20v1%20Compatible-00F2FE?style=for-the-badge&logo=openai)](https://platform.openai.com/docs/api-reference)
[![UI: Apple Liquid Glass](https://img.shields.io/badge/UI-Apple%20Liquid%20Glass%20%2B%20Linear-8B5CF6?style=for-the-badge)](#1-🎛️-aether-旗艦遠隔測定ダッシュボード-apple-liquid-glass--linear-ui)
[![Zero Logout](https://img.shields.io/badge/Firefox-Multi--Account%20Containers-FF7139?style=for-the-badge&logo=firefox)](https://addons.mozilla.org/firefox/addon/multi-account-containers/)
[![Long Context](https://img.shields.io/badge/Context-1.2MB%203--Tier%20Engine-10B981?style=for-the-badge)](https://gemini.google.com)

[日本語](README_JA.md) | [English](README.md) | [繁體中文](README_ZH-TW.md) | [简体中文](README_CN.md)

</div>

---

## 🚀 なぜ AETHER GATEWAY を選ぶのか？

公式 Google Gemini API には厳しい従量課金とレートリミットが課されています。従来のオープンソースプロキシでは、**複数アカウントが互いにログアウトする**、**長文プロンプトで 502 エラーが頻発する**、**同時リクエスト集中で `BardErrorInfo [1095]` BAN が発生する**といった課題がありました。

**AETHER GATEWAY** はこれらの課題を完全に解決し、安定した高スループット環境を提供します：

```
[クライアント: Python / TypeScript / Go / Rust / Obsidian / Next.js]
                           ↓ http://localhost:8083/v1/chat/completions
┌────────────────────────────────────────────────────────────────────────┐
│                        AETHER GATEWAY コア                             │
│  • 単一IP同時実行リミッター (1095空フレーム拒否を自動防止)             │
│  • 3段階長文突破パイプライン (可逆圧縮 → 155KB添付 → 1.2MB分割投入)    │
│  • Chrome 146 TLS / JA4 ブラウザフィンガープリント偽装 + BL自動同期    │
│  • Apple Liquid Glass ＆ Linear 漆黒ダークモードダッシュボード         │
└────────────────────────────────────────────────────────────────────────┘
       ↓ 物理 Cookie 隔離コンテナ               ↓ 出口プロキシメッシュ
┌───────────────────────────────────┐     ┌───────────────────────────────┐
│ Firefox Multi-Account Containers  │     │ プロキシノードプール          │
│ • バックエンドから containers.json 注入│     │ • 複数出口IPローテーション    │
│ • 複数アカウント同時ログイン隔離  │     │ • サーキットブレーカー機能    │
│ • SQLite SNlM0e リアルタイム検知  │     │ • 80 RPH Google BANレーダー   │
└───────────────────────────────────┘     └───────────────────────────────┘
                           ↓ 逆向 StreamGenerate プロトコル
            [Google Gemini Web (gemini-3.8-flash / thinking)]
```

---

## ✨ コア機能とブレークスルー

### 1. 🎛️ AETHER 旗艦遠隔測定ダッシュボード (Apple Liquid Glass & Linear UI)
* **究極のデザイン美学**：漆黒のカーボン（`#06080D`）にデュアルハイライトマイクロボーダー（`inset 0 1px 0 rgba(255,255,255,0.12)`）とアクリルすりガラス（`backdrop-filter: blur(24px)`）を採用。
* **Google 80 RPH BANレッドラインレーダー (Distance to Ban Redline)**：単一IPの80〜180 RPH WAF制限までの安全マージンをリアルタイムで可視化。
* **P50 / P95 レイテンシー波形グラフ (Chart.js)**：リクエスト量、中央値レイテンシー、95パーセンタイル値をデュアルY軸で追跡。
* **フライトログと全経路フォレンジックドロワー**：検索、ページネーション、CSV出力、TTFB（初文字応答時間）と生成思考時間の内訳を完全表示。
* **完全オフライン対応**：すべてのライブラリをバイナリ内部に組み込み、外部CDN依存はゼロ。

### 2. 🦊 Firefox 多重コンテナ自動構築＆ライブウィザード (ログアウトゼロ)
* **0クリック自動コンテナ注入**：Firefoxの `containers.json` に直接書き込み、分離コンテナを一括生成。手動操作は一切不要。
* **物理 Cookie 隔離**：Firefox Contextual Identities により、同一マシンで5〜50個のGoogleアカウントをセッション競合なく同時保持。
* **リアルタイム検知レーダー**：ローカルの `cookies.sqlite` を自動ポーリングし、ログイン完了を緑色ランプ（`🟢 SNlM0e ✅`）で即座に表示。

### 3. 📚 1.2MB 3段階長文突破パイプライン
* **Tier 1 (32KB未満)**：可逆最適化によるダイレクト送信。
* **Tier 2 (32KB〜155KB)**：Google公式の `message.txt` テキスト添付ファイルへ透過的に変換。
* **Tier 3 (155KB〜1.2MB)**：マルチターン会話として自動分割し、完了後にリモート履歴を自動削除。

### 4. 🧠 思考モデル（Thinking）とSSEストリーミング
* `gemini-3.8-flash-thinking`、`gemini-3.7-flash` に完全対応。
* `delta.reasoning_content` を美しいアコーディオンドロワーにリアルタイム描画。

---

## 🛠️ 主な言語での呼び出し例

### 🐍 Python (OpenAI SDK + 思考プロセス抽出)

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
        {"role": "system", "content": "あなたは優秀なシステムアーキテクトです。"},
        {"role": "user", "content": "非同期マイクロサービスパイプラインの設計パターンを解説してください。"}
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

### 🌐 TypeScript / Node.js (思考ストリーミング対応)

```typescript
import OpenAI from "openai";

const openai = new OpenAI({
  baseURL: "http://localhost:8083/v1",
  apiKey: process.env.GEMINI_API_KEY || "sk-gemini-YOUR-MASTER-KEY",
});

async function main() {
  const stream = await openai.chat.completions.create({
    model: "gemini-3.8-flash-thinking",
    messages: [{ role: "user", content: "現代の非同期アーキテクチャの利点を3行で説明してください。" }],
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
			{Role: openai.ChatMessageRoleUser, Content: "Go channel の内部構造を簡潔に説明してください。"},
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
        "messages": [{"role": "user", "content": "非同期パイプラインの美しさを短歌で表現してください。"}],
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
    "messages": [{"role": "user", "content": "エンドツーエンド接続確認"}],
    "temperature": 0.1
  }'
```

---

### 🔮 Obsidian (Smart Connections / Copilot)

1. Obsidian 設定 ➔ **Smart Connections** または **Copilot** を選択。
2. **Custom OpenAI API Endpoint**: `http://localhost:8083/v1`
3. **API Key**: `sk-gemini-YOUR-MASTER-KEY`
4. **Model Name**: `gemini-3.8-flash` または `gemini-3.8-flash-thinking`。

---

## ⚡ クイックスタート

```bash
# 1. ビルド
go build -o gemini-web2api-go main.go

# 2. 起動
./gemini-web2api-go --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```
ブラウザで [`http://localhost:8083/admin/`](http://localhost:8083/admin/) を開くと、AETHER 制御センターが表示されます。

---

## 📋 本番環境の推奨パラメータ

| 動作環境 | 推奨並行数 (`-c`) | バッチサイズ (`-b`) | 運用メモ |
| :---: | :---: | :---: | :--- |
| **単一ホストIP直結 (1〜5 アカウント)** | **3 〜 4** | **2 〜 3** | **⭐ 黄金比率**。プロンプトを15k文字以内に保ち、1095エラーを回避して瞬時応答を維持。 |
| **分散プロキシプール (5+ ノード)** | **10 〜 25** | **3** | 出口IPを分散し、スループットをリニアに拡大可能。 |

---

## 📄 ライセンス
MIT License
