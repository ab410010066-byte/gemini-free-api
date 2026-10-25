# Gemini Web2API ゲートウェイ (Go 言語版)

> 高性能でプライベート環境にデプロイ可能なリバースプロキシゲートウェイ。Google Gemini Web を OpenAI 完全互換の `/v1/chat/completions` ストリーミング API に変換します。Google 課金 API キーは不要であり、アカウント単位の厳格な並行分離排他制御により強制ログアウトを根本から防止し、高密度リアルタイム監視ダッシュボードを内蔵しています。

[English](README.md) | [繁體中文](README_ZH-TW.md) | [日本語](README_JA.md) | [簡體中文](README_CN.md)

---

## 1. システムアーキテクチャ

```
[クライアントアプリケーション]
(Python / TypeScript / Go / Rust / cURL / Obsidian)
                      |
                      v  HTTP POST /v1/chat/completions
+------------------------------------------------------------------------+
|                    Gemini Web2API ゲートウェイコア                     |
|                                                                        |
|  [イングレスおよび認証層]                                              |
|    - Bearer Token 検証および権限インターセプト                         |
|    - OpenAI データ仕様検証とロール正規化                               |
|                                                                        |
|  [長文コンテキスト前処理パイプライン]                                  |
|    - 可逆的空白文字圧縮および Tool 定義軽量化                          |
|    - ネイティブコンテキストファイル添付パイプライン (<155 KB)           |
|                                                                        |
|  [トラフィック制御およびスケジューラ]                                  |
|    - IP 出口スロット制限 (デフォルト並行 5、80 RPH 規制防止境界)        |
|    - アカウント排他排他ガード (厳格な 1 対 1 分離で強制ログアウト防止)  |
|                                                                        |
|  [アップストリーム通信エンジン]                                        |
|    - Chrome TLS / JA4 ハンドシェイク偽装 (bogdanfinn/tls-client)       |
|    - Google Web フロントエンド bl ビルドバージョンの動的追従           |
|    - SSE ストリームパーサーおよび reasoning_content 思考プロセスの抽出 |
+------------------------------------------------------------------------+
            |                                           |
            v ホスト直接接続                            v 出口プロキシメッシュ
+-------------------------------+           +-------------------------------+
| ホストネットワーク NIC        |           | プロキシノードプール          |
| 単一 IP (80 RPH 安全境界)     |           | マルチ IP ラウンドロビン      |
+-------------------------------+           +-------------------------------+
            \                                           /
             \                                         /
              v                                       v
         [Google Gemini Web バックエンド (StreamGenerate プロトコル)]
```

### 1.1 リクエストライフサイクルとスケジューリング

```mermaid
flowchart TD
    Req[クライアントリクエスト受信] --> Auth{Bearer Token 検証}
    Auth -- 失敗 --> E401[HTTP 401 Unauthorized 返却]
    Auth -- 成功 --> Ingest[OpenAI 履歴メッセージの正規化]
    
    Ingest --> LengthCheck{Prompt バイト長チェック}
    LengthCheck -- "< 30 KB" --> InflightSlot[IP 出口並行スロット取得]
    LengthCheck -- "30 KB - 155 KB" --> MakeDoc[ネイティブ添付ファイルに変換] --> InflightSlot
    LengthCheck -- "> 155 KB" --> E400[HTTP 400 Context Length Exceeded 返却]
    
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

### 1.2 アカウント分離とゼロログアウト同期設計

従来のリバースプロキシでは、並行アクセスや定期的なトークンリフレッシュ時に、バックエンドから Google へ直接 `RotateCookies` をリクエストすることにより、ブラウザ側トークンがリプレイ攻撃と判定され、Google サーバーによってセッションが強制切断される問題が頻発していました。本システムはアーキテクチャの根幹でこの課題を解決しています：

```mermaid
flowchart LR
    subgraph Firefox コンテナ環境
        C1[コンテナ 1: アカウント A]
        C2[コンテナ 2: アカウント B]
        C3[コンテナ 3: アカウント C]
    end

    subgraph OS ローカルストレージ
        SQLite[(Firefox cookies.sqlite)]
    end

    subgraph ゲートウェイ常駐プロセス
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

* **自発的 RotateCookies の全面廃止**: バックエンドから Google へ模擬的なトークン再発行リクエストを発行しないため、セッション衝突を完全に回避します。
* **アカウント単位の厳格な単一並行制限**: 1 つの Google アカウントにつき、同一瞬間に処理できるストリーミングリクエストを 1 件に制限。並行リクエストは他の空きアカウントへ均等分散されるかキュー待機され、セッション破棄を根絶します。

---

## 2. コアアーキテクチャ構成要素

| コンポーネント | 主な責務 | 障害復旧および保護メカニズム |
| :--- | :--- | :--- |
| **イングレスエンジン (Ingress Engine)** | OpenAI `/v1/chat/completions`、`/v1/models` エンドポイントの提供。 | 標準的な JSON エラー (`invalid_request_error`、`rate_limit_exceeded`) を返却。 |
| **アカウントプールマネージャー (Account Fleet)** | SQLite を基盤としたアカウント資格情報、使用制限、プロキシ設定の管理。 | 3 回連続認証失敗時に自動的にアカウントを一時停止し、全体の遅延を防止。 |
| **単一アカウント排他ガード (In-Flight Guard)** | アカウントごとの同時実行数を厳格に 1 に制限。 | 同時リクエスト時に最大 6 秒間キューイング、または他の空きアカウントへ分散。 |
| **IP スロットリミッター (IP Slot Limiter)** | 出口 IP ごとの同時接続数、RPM、RPH を厳密に制御。 | Google WAF の規制閾値に近づいた際、安全に拒絶または別プロキシへ退避。 |
| **長文コンテキスト前処理部** | 余分な空白の圧縮および長文テキストのファイル添付化。 | 155 KB の上限を超える場合は HTTP 400 を返し、Google 側のアカウント制裁を回避。 |
| **テレメトリ UI (Telemetry UI)** | 内蔵管理画面と Chart.js によるレイテンシ可視化。 | 単一バイナリに 100% 埋め込み、外部 CDN や外部ネットワークへの依存ゼロ。 |

---

## 3. 対応モデル一覧表

| モデル識別子 (Model ID) | バックエンド Hex コード | 思考プロセス対応 (Thinking) | マルチモーダル (画像/ファイル) | 最大出力 Token | 推奨されるユースケース |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini-3.8-flash` | `56fdd199312815e2` | なし | あり | 8,192 | 低レイテンシ、高スループットな応答 |
| `gemini-3.8-flash-thinking` | `56fdd199312815e2` | あり (`reasoning_content`) | あり | 8,192 | 高度な論理推論、推論プロセスの可視化 |
| `gemini-3.7-flash` | `fbb127bbb056c959` | なし | あり | 8,192 | 日常的なテキスト変換・要約処理 |
| `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | なし | あり | 8,192 | 軽量タスク、フォールバック実行 |
| `gemini-3.1-pro` | `e6fa609c3fa255c0` | あり | あり | 8,192 | 深層分析、専門分野のデータ抽出 |

---

## 4. 実行時設定パラメータ一覧

全パラメータは SQLite データベースに永続化され、管理ダッシュボード (`http://localhost:8083/admin/`) からサービスを再起動することなく即座に動的更新可能です。

| パラメータ名 | データ型 | デフォルト値 | 推奨安全範囲 | 機能説明および安定性への影響 |
| :--- | :--- | :--- | :--- | :--- |
| `per_ip_concurrent` | 整数 | `5` | `1 - 10` | 1 つの出口 IP あたりの最大同時実行数。10 を超えると Google の拒絶対象となります。 |
| `per_ip_rpm` | 整数 | `30` | `10 - 60` | 1 つの出口 IP あたりの 1 分間最大リクエスト数。 |
| `per_ip_rph` | 整数 | `80` | `40 - 150` | 1 つの出口 IP あたりの 1 時間最大リクエスト数。Google WAF 規制線未満に保持。 |
| `max_prompt_bytes` | 整数 | `120000` | `50000 - 150000` | 入力プロンプトの UTF-8 バイト上限。この値を超えると添付ファイル化パイプラインへ遷移。 |
| `context_compression` | 真偽値 | `true` | `true / false` | プロンプト内の重複空白文字や無駄なフォーマットを可逆的に軽量化。 |
| `context_file_upload` | 真偽値 | `true` | `true / false` | 30 KB - 155 KB の長文プロンプトを自動的にテキストファイル添付として送信。 |
| `fallback_anon` | 真偽値 | `false` | `true / false` | アカウントが全てビジーの際、思考を伴わないリクエストを一時的に匿名ルートへ迂回。 |
| `impersonate` | 文字列 | `chrome_146` | TLS プロファイル | 偽装するブラウザの TLS および JA4 署名プロファイル。 |
| `gemini_bl_auto` | 真偽値 | `true` | `true / false` | 6 時間ごとに Google Web の最新 bl バージョンを自動取得して適用。 |

---

## 5. 本番環境向けデプロイおよびレート調整指針

高負荷耐久テストに基づく各環境の推奨パラメータは以下の通りです：

| ネットワーク構成 | 有効アカウント数 | 推奨並行スロット | 安全 RPM | 安全 RPH | 運用上の注意点 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **ホスト直接接続 IP** | 1 - 5 アカウント | 3 - 5 | 15 - 25 | 60 - 80 | 個人利用や Obsidian ナレッジベース連携における推奨設定。 |
| **単一専用プロキシ** | 1 - 3 アカウント | 3 - 4 | 15 - 20 | 60 - 75 | ホスト IP の露出を防ぎ、中規模バッチ処理に適しています。 |
| **分散プロキシプール** | 5 - 20 アカウント | 10 - 25 | 60 - 120 | 300 - 800 | 負荷を複数のプロキシノードに分散させる高スループット構成。 |

---

## 6. アップストリーム異常検知と自動復旧マッピング

| アップストリーム検知パターン | HTTP 状態コード | 根本原因 | ゲートウェイの自動対処 |
| :--- | :--- | :--- | :--- |
| `no SNlM0e in page` | 401 Unauthorized | アカウントのセッション期限切れまたはログアウト | 失敗回数を加算し、プール内の別の健全なアカウントへ自動切替。 |
| `BardErrorInfo [1095]` | 429 Too Many Requests | IP レベルの瞬間過負荷またはレート制限 | 当該 IP スロットを満杯とみなし、リクエストを一時停止または別プロキシへ退避。 |
| `302 -> /sorry/index` | 429 Too Many Requests | Google WAF による一時的 IP 遮断 (80-120 分間継続) | プロキシノードの自動サーキットブレーカーを発動し、冷却待機へ移行。 |
| `PromptTooLongError` | 400 Bad Request | プロンプトが 155 KB の添付安全上限を超過 | `context_length_exceeded` を返し、アカウント制裁リスクを未然に防止。 |
| `xsrf token mismatch` | 502 Bad Gateway | キャッシュされた XSRF トークンの失効 | メモリ内キャッシュを破棄し、新しいトークンを再取得して即座に自動再試行。 |

---

## 7. プログラミング言語別インテグレーションコード

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

### 7.6 Obsidian (Smart Connections / Copilot)

1. Obsidian の設定を開き、**Smart Connections** または **Copilot** プラグインへ移動します。
2. **Custom OpenAI API Endpoint** に `http://localhost:8083/v1` を指定します。
3. **API Key** に `sk-gemini-master-key` (または設定した `--admin-token`) を指定します。
4. **Model Identifier** に `gemini-3.8-flash` または `gemini-3.8-flash-thinking` を指定します。

---

## 8. クイックスタート

### 8.1 単一バイナリによる直接起動 (推奨)

```bash
# 1. バイナリのビルド
go build -o gemini-web2api-go main.go

# 2. サーバーの起動 (ポートおよび管理トークンを指定)
./gemini-web2api-go --port 8083 --admin-token YOUR-ADMIN-PASSWORD
```

ブラウザで `http://localhost:8083/admin/` にアクセスして管理ダッシュボードを開きます。

### 8.2 Docker コンテナによる実行

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

## 9. コマンドライン引数一覧

| コマンドライン引数 | 環境変数 | デフォルト値 | 説明 |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8083` | ゲートウェイおよび管理画面がリッスンする HTTP ポート。 |
| `--admin-token` | `ADMIN_TOKEN` | 自動生成 | 管理画面ログインおよびクライアント呼び出し用のマスターキー。 |
| `--db` | `DATABASE_URL` | `sqlite://data/gemini.db` | SQLite データベースファイルのパス、または PostgreSQL / MySQL 接続文字列。 |
| `--cookie-file` | `COOKIE_FILE` | (任意) | 初期投入用の Netscape または JSON 形式の Cookie ファイルパス。 |
| `--proxy` | `PROXY_URL` | (任意) | 静的アップストリームプロキシ URL。 |

---

## 10. ライセンス

MIT ライセンスの下で配布されています。
研究、ローカル開発、パーソナルナレッジベース構築を目的としています。Google の利用規約を遵守してご利用ください。
