# Gemini Free API (Go 言語オープンソース版)

> 無料でプライベート環境にデプロイ可能な Google Gemini to OpenAI `/v1/chat/completions` リバースプロキシゲートウェイ。Google 課金 API キーやクレジットカード登録は一切不要です。Gemini 3.8 Flash、思考プロセス (reasoning_content) のリアルタイムストリーミング、マルチモーダル入力、最大 50,000 Token の長文ドキュメントパイプライン、およびゼロログアウト分離による複数アカウントプールをネイティブサポートします。

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [簡體中文](README_CN.md)

---

## 1. コア機能と特徴

* **完全無料かつ API キー不要**: Google Cloud の課金体系を回避し、Web 版リバースプロキシを通じて Gemini の能力を直接活用可能。
* **OpenAI API 完全互換**: One-API、New-API、NextChat、LobeChat、Cherry Studio、Dify、Obsidian などの主要クライアントと即座に連携。
* **思考プロセスのリアルタイムストリーミング**: `gemini-3.8-flash-thinking` および `gemini-3.1-pro` の `delta.reasoning_content` を分離して配信。
* **50,000 Token 長文ドキュメントパイプライン**: 長文プロンプトを Google ネイティブのテキストファイル添付として自動送信し、100% 欠損なしで処理。
* **複数アカウントプールと強制ログアウト防止**: Firefox Multi-Account Containers による物理コンテナ分離でセッションの競合を根絶。
* **厳格なアカウント単位の並行排他制御**: 単一 Google アカウントへ同時に 2 つ以上の並行リクエストが流れるのを防ぐ排他ロック機構。
* **高度なブラウザ指紋偽装**: Chrome 146 TLS および JA4 ハンドシェイク署名偽装を内蔵 (`bogdanfinn/tls-client`)。
* **単一バイナリ内蔵管理画面**: 外部 CDN への依存ゼロで、レイテンシ分析、アカウント管理、使用状況確認が可能。

---

## 2. コンテキストウィンドウと Token 容量仕様

| 処理ステージ | 入力バイト閾値 | 推定入力 Token 数 | 実行メカニズム | 応答特性 |
| :--- | :--- | :--- | :--- | :--- |
| **第 1 階層：標準プロンプト** | < 30,000 バイト | 約 1 〜 8,000 Token | 余分な空白の最適化を伴うインラインプロンプト実行 | 即時応答、最小遅延 |
| **第 2 階層：ドキュメント添付** | 30,000 〜 155,000 バイト | 約 8,000 〜 50,000 Token | Google ネイティブの `message.txt` 添付ファイルとして自動マウント | 切り捨てゼロ、100% 完全取り込み |
| **第 3 階層：上限保護** | > 155,000 バイト | > 50,000 Token | HTTP 400 (`context_length_exceeded`) を即時返却 | ボット検知およびアカウント規制を未然に防止 |
| **生成出力制限** | 1 リクエストあたり | 最大 8,192 出力 Token | OpenAI SSE ストリーム配信、思考プロセスの分離出力 | `reasoning_content` 完全対応 |

---

## 3. システムアーキテクチャ

```
[クライアントエコシステム]
(Python / TypeScript / Go / Rust / cURL / One-API / NextChat / Obsidian)
                       |
                       v  HTTP POST /v1/chat/completions
+-------------------------------------------------------------------------+
|                       Gemini Free API ゲートウェイ                      |
|                                                                         |
|  [イングレスおよび認証層]                                               |
|    - Bearer Token 検証および権限インターセプト                          |
|    - OpenAI データ仕様検証とロール正規化                                |
|                                                                         |
|  [長文コンテキストエンジン (最大 50,000 Token)]                         |
|    - 可逆的空白文字圧縮および不要構造の除去                             |
|    - ネイティブ Context ファイル添付 (<155 KB / ~50,000 Token)          |
|                                                                         |
|  [トラフィック制御およびセッションガード]                               |
|    - IP 出口スロット制限 (デフォルト並行 5、80 RPH 安全境界)            |
|    - アカウント排他排他ガード (厳格な 1 対 1 分離で強制ログアウト防止)   |
|                                                                         |
|  [アップストリーム通信エンジン]                                         |
|    - Chrome TLS / JA4 ハンドシェイク偽装 (bogdanfinn/tls-client)        |
|    - Google Web フロントエンド bl ビルドバージョンの動的追従            |
|    - SSE ストリームパーサーおよび reasoning_content 思考プロセスの抽出  |
+-------------------------------------------------------------------------+
             |                                           |
             v ホスト直接接続                            v 出口プロキシメッシュ
+--------------------------------+          +--------------------------------+
| ホストネットワーク NIC         |          | プロキシノードプール           |
| 単一 IP (80 RPH 安全境界)      |          | マルチ IP ラウンドロビン       |
+--------------------------------+          +--------------------------------+
             \                                           /
              \                                         /
               v                                       v
         [Google Gemini Web バックエンド (StreamGenerate プロトコル)]
```

### 3.1 リクエストライフサイクルと並行排他制御

```mermaid
flowchart TD
    Req[クライアントリクエスト受信] --> Auth{Bearer Token 検証}
    Auth -- 失敗 --> E401[HTTP 401 Unauthorized 返却]
    Auth -- 成功 --> Ingest[OpenAI 履歴メッセージの正規化]
    
    Ingest --> LengthCheck{Prompt バイト長チェック}
    LengthCheck -- "< 30 KB (~8k Token)" --> InflightSlot[IP 出口並行スロット取得]
    LengthCheck -- "30 KB - 155 KB (~50k Token)" --> MakeDoc[ネイティブ添付ファイルに変換] --> InflightSlot
    LengthCheck -- "> 155 KB (>50k Token)" --> E400[HTTP 400 Context Length Exceeded 返却]
    
    InflightSlot -- IP スロット満杯 --> E429IP[HTTP 429 レート制限返却]
    InflightSlot -- スロット取得成功 --> AcctSelect[現在実行中のない空きアカウントを選択]
    
    AcctSelect -- 全アカウントビジー --> WaitAcct{短時間キュー待機 (最大 6 秒)}
    WaitAcct -- タイムアウトかつ FallbackAnon 有効 --> AnonReq[匿名ルートへフォールバック]
    WaitAcct -- タイムアウトかつ厳格モード --> E429Acct[HTTP 429 アカウントビジー返却]
    WaitAcct -- アカウント解放確認 --> ExecReq[StreamGenerate リクエスト実行]
    AcctSelect -- 空きアカウント取得成功 --> ExecReq
    
    ExecReq --> SSETrans[チャンクプロトコルを OpenAI SSE ストリームに変換]
    SSETrans --> RelSlot[IP スロットおよびアカウント排他ロックを解放]
    RelSlot --> ClientEnd[クライアントストリーミング完了]
```

### 3.2 ゼロログアウトアカウント分離モデル

```mermaid
flowchart LR
    subgraph Firefox マルチコンテナ環境
        C1[コンテナ 1: アカウント A]
        C2[コンテナ 2: アカウント B]
        C3[コンテナ 3: アカウント C]
    end

    subgraph OS ローカルストレージ
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph ゲートウェイコア
        Poller[バックグラウンド読取ポーラー]
        Pool[(メモリ内アカウントプール)]
        InflightLock[アカウント単位の厳格な 1 対 1 排他ロック]
    end

    C1 -->|ブラウザ正規のバックグラウンド更新| SQLite
    C2 -->|ブラウザ正規のバックグラウンド更新| SQLite
    C3 -->|ブラウザ正規のバックグラウンド更新| SQLite

    SQLite -->|10 分ごとに読み取り安全同期| Poller
    Poller -->|差分更新| Pool
    Pool --> InflightLock
    InflightLock -->|同一アカウントへ同時に 2 接続以上流さない| Upstream[Google Gemini Web]
```

---

## 4. クイックスタート

### 4.1 単一バイナリによる直接起動 (推奨)

```bash
# 1. バイナリのビルド
go build -o gemini-free-api main.go

# 2. サーバーの起動 (ポートおよび管理トークンを指定)
./gemini-free-api --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

ブラウザで `http://localhost:8083/admin/` にアクセスして内蔵管理ダッシュボードを開きます。

### 4.2 Docker コンテナによる実行

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

## 5. 主要サードパーティクライアント接続設定

| アプリケーション | カスタム API エンドポイント (Base URL) | API Key | 推奨モデル名 |
| :--- | :--- | :--- | :--- |
| **One-API / New-API** | `http://localhost:8083` | `YOUR-ADMIN-PASSWORD` | チャンネルタイプ `OpenAI`、`gemini-3.8-flash-thinking` を指定 |
| **NextChat / ChatGPT-Next-Web** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` または `gemini-3.8-flash-thinking` |
| **LobeChat** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | カスタム OpenAI プロバイダーとして設定 |
| **Cherry Studio** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI 互換プロバイダーとして設定 |
| **Dify** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | OpenAI-API 互換モデルプロバイダー |
| **Obsidian (Smart Connections)** | `http://localhost:8083/v1` | `YOUR-ADMIN-PASSWORD` | `gemini-3.8-flash` |

---

## 6. 対応モデル一覧表

| モデル識別子 (Model ID) | バックエンド Hex コード | 思考プロセス対応 (Thinking) | マルチモーダル (画像/ファイル) | 出力 Token 上限 | 推奨されるユースケース |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | なし | あり | 8,192 | 低レイテンシ、高スループットな応答 |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | あり (`reasoning_content`) | あり | 8,192 | 高度な論理推論、推論プロセスの可視化 |
| `gemini-3.7-flash` | `fbb127bbb056c959` | なし | あり | 8,192 | 日常的なテキスト変換・要約処理 |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | なし | あり | 8,192 | 軽量タスク、フォールバック実行 |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | あり | あり | 8,192 | 深層分析、専門分野のデータ抽出 |

---

## 7. プログラミング言語別呼び出し例

### 7.1 Python (公式 OpenAI SDK)

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

### 7.3 Go (標準ライブラリのみ、外部依存関係なし)

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

## 8. 推奨本番環境設定

| ネットワーク構成 | 有効アカウント数 | 推奨並行スロット | 安全 RPM | 安全 RPH | 運用上の指針 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **ホスト直接接続 IP** | 1 - 5 アカウント | 3 - 5 スロット | 15 - 25 | 60 - 80 | 個人利用や Obsidian ナレッジベース連携における標準設定。 |
| **専用プロキシ** | 1 - 3 アカウント | 3 - 4 スロット | 15 - 20 | 60 - 75 | ホスト IP の露出を防ぎ、小規模チームの自動化に適しています。 |
| **分散プロキシプール** | 5 - 20 アカウント | 10 - 25 スロット | 60 - 120 | 300 - 800 | 負荷を複数のプロキシノードに分散させる高スループット構成。 |

---

## 9. 実行時設定パラメータ一覧

全パラメータは SQLite データベースに永続化され、管理ダッシュボード (`http://localhost:8083/admin/`) からサービスを再起動することなく即座に動的更新可能です。

| パラメータ名 | データ型 | デフォルト値 | 推奨安全範囲 | 機能説明および安定性への影響 |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | 整数 | `5` | `1 - 10` | 1 つの出口 IP あたりの最大同時実行数。10 を超えると Google の拒絶対象となります。 |
| `per_ip_rpm` | 整数 | `30` | `10 - 60` | 1 つの出口 IP あたりの 1 分間最大リクエスト数。 |
| `per_ip_rph` | 整数 | `80` | `40 - 150` | 1 つの出口 IP あたりの 1 時間最大リクエスト数。Google WAF 規制線未満に保持。 |
| `max_prompt_bytes` | 整数 | `120000` | `50000 - 150000` | 入力プロンプトの UTF-8 バイト上限。この値を超えると添付ファイル化パイプラインへ遷移。 |
| `context_compression` | 真偽値 | `true` | `true / false` | プロンプト内の重複空白文字や無駄なフォーマットを可逆的に軽量化。 |
| `context_file_upload` | 真偽値 | `true` | `true / false` | 30 KB - 155 KB (約 50,000 Token) の長文プロンプトを自動的にテキストファイル添付として送信。 |
| `fallback_anon` | 真偽値 | `false` | `true / false` | アカウントが全てビジーの際、思考を伴わないリクエストを一時的に匿名ルートへ迂回。 |
| `impersonate` | 文字列 | `chrome_146` | TLS プロファイル | 偽装するブラウザの TLS および JA4 署名プロファイル。 |
| `gemini_bl_auto` | 真偽値 | `true` | `true / false` | 6 時間ごとに Google Web の最新 bl バージョンを自動取得して適用。 |

---

## 10. コマンドライン引数一覧

| コマンドライン引数 | 環境変数 | デフォルト値 | 説明 |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | ゲートウェイおよび管理画面がリッスンする HTTP ポート。 |
| `--admin-token` | `ADMIN_TOKEN` | 自動生成 | 管理画面ログインおよびクライアント呼び出し用のマスターキー。 |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | SQLite データベースファイルのパス、または PostgreSQL / MySQL 接続文字列。 |
| `--cookie-file` | `COOKIE_FILE` | (任意) | 初期投入用の Netscape または JSON 形式の Cookie ファイルパス。 |
| `--proxy` | `PROXY_URL` | (任意) | 静的アップストリームプロキシ URL。 |

---

## 11. ライセンス

MIT ライセンスの下で配布されています。
研究、ローカル開発、パーソナルナレッジベース構築を目的としています。Google の利用規約を遵守してご利用ください。
