# KH-Copilot-SDK-Apps

AI エージェントによるコード生成とレビューを反復的に行うマルチエージェントオーケストレーションシステムです。**Implementer Agent** が仕様に基づいてコードを生成し、**Reviewer Agent** が要件に照らしてコードを検証します。承認されるか、最大反復回数に達するまでこのサイクルが繰り返されます。

## アーキテクチャ

```
┌─────────────────────────────────────────────────┐
│                 Orchestrator                     │
│                                                  │
│  ┌──────────────┐    Channel    ┌─────────────┐ │
│  │ Implementer  │──────────────▶│  Reviewer    │ │
│  │    Agent      │◀──────────────│    Agent     │ │
│  └──────┬───────┘   Feedback    └──────┬──────┘ │
│         │                              │         │
│         └──────────┬───────────────────┘         │
│                    │                             │
│           ┌────────▼────────┐                    │
│           │ Context Manager │                    │
│           │  L1: Project    │                    │
│           │  L2: Task/Plan  │                    │
│           │  L4: Feedback   │                    │
│           └─────────────────┘                    │
└─────────────────────────────────────────────────┘
```

### ワークフロー

1. Orchestrator がプランファイルとプロジェクトコンテキストを読み込む
2. Implementer Agent がプランに基づいてコードを生成
3. Reviewer Agent が生成されたコードを要件に照らしてレビュー
4. 承認（APPROVED）の場合 → 完了
5. 修正要求（NEEDS_REVISION）の場合 → フィードバックを Implementer に送り、ステップ 2 に戻る
6. 最大反復回数に達した場合 → 最終結果を返して終了

## ディレクトリ構成

```
.
├── cmd/
│   └── orchestrator/
│       └── main.go              # エントリーポイント
├── internal/
│   ├── agent/
│   │   ├── agent.go             # Agent インターフェースと共通ユーティリティ
│   │   ├── implementer.go       # コード生成エージェント
│   │   └── reviewer.go          # コードレビューエージェント
│   ├── config/
│   │   └── config.go            # YAML 設定の読み込みと管理
│   ├── context/
│   │   ├── manager.go           # 階層的コンテキスト管理
│   │   └── types.go             # データ構造の定義
│   ├── message/
│   │   └── message.go           # エージェント間メッセージングプロトコル
│   └── orchestrator/
│       └── orchestrator.go      # オーケストレーションロジック
├── config.yaml                  # 設定ファイル
├── go.mod
└── go.sum
```

## 技術スタック

- **言語**: Go 1.24
- **依存ライブラリ**:
  - `golang.org/x/sync` — 並行エージェント実行のための errgroup
  - `gopkg.in/yaml.v3` — YAML 設定ファイルの解析

## セットアップ

### 前提条件

- Go 1.24 以上

### ビルド

```bash
go build -o orchestrator ./cmd/orchestrator/main.go
```

## 使い方

### 基本的な実行

```bash
# プランファイルを指定して実行（必須）
./orchestrator --plan path/to/plan.md
```

### 全オプション指定

```bash
./orchestrator \
  --config config.yaml \
  --plan plans/my-plan.md \
  --workdir /path/to/project
```

### CLI フラグ

| フラグ | 必須 | デフォルト | 説明 |
|--------|------|------------|------|
| `--plan` | Yes | — | 実装プランの Markdown ファイルパス |
| `--config` | No | `config.yaml` | 設定ファイルのパス |
| `--workdir` | No | `.` | 解析対象のプロジェクトディレクトリ |

### プランファイル

プランファイルは Markdown 形式で記述します。Context Manager がセクションごとに優先度を判定し、トークン予算内に収まるよう自動的に調整します。

**セクション優先度**:

| 優先度 | セクション例 |
|--------|-------------|
| 高 | Implementation steps, Requirements, Specs |
| 中 | Overview, Design, Architecture |
| 低 | Background, Verification, Tests, References |

## 設定

`config.yaml` でシステムの動作をカスタマイズできます。

```yaml
# 最大反復回数
max_iterations: 5

# 作業ディレクトリ
work_dir: "."

# エージェントのシステムプロンプト
implementer:
  system_prompt: |
    You are an expert software implementer...

reviewer:
  system_prompt: |
    You are an expert code reviewer...

# コンテキストのトークン予算
context:
  max_project_summary_tokens: 4000
  max_code_tokens: 8000
  max_review_tokens: 4000
  ignore_patterns:
    - "vendor/"
    - "node_modules/"
    - ".git/"
```

## 設計上の特徴

### 階層的コンテキスト管理

Context Manager は 3 層構造でコンテキストを管理します。

- **L1（Project Context）**: プロジェクト構造、依存関係、設定ファイル
- **L2（Task Context）**: プランファイル、対象ファイル、制約条件
- **L4（Agent Context）**: 反復ごとのフィードバックと出力

トークン予算（デフォルト 128KB、プランに 40% を確保）に基づき、コンテキストウィンドウを効率的に利用します。

### エージェント間通信

Go チャネルを使った型付きメッセージングにより、Implementer と Reviewer が疎結合で通信します。メッセージタイプには `implementation`、`review`、`approved`、`error` があります。

### セキュリティ

- ファイル読み込み時のパストラバーサル防止
- 設定された ignore パターンによるファイルフィルタリング
- トークン予算の強制によるコンテキストオーバーフロー防止

## 対応言語

プロジェクト解析で以下の言語を自動検出します:

- Go（`.go`）
- TypeScript（`.ts`）
- JavaScript（`.js`）
- Python（`.py`）

## 現在のステータス

> **Note**: Copilot SDK との統合は開発中です。現在 `SendPrompt()` はプレースホルダー実装を返します。SDK 統合が完了すると、AI によるコード生成・レビューが完全に動作するようになります。

## ライセンス

本リポジトリのライセンスについては、リポジトリオーナーにお問い合わせください。
