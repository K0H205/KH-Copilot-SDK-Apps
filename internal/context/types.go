package context

// ProjectContext はプロジェクトの静的コンテキストを保持する。
// 起動時に1回だけ収集し、以降はキャッシュとして共有する。
type ProjectContext struct {
	RootDir   string            // プロジェクトルートパス
	Tree      string            // ディレクトリツリー（文字列表現）
	Files     map[string]string // パス→内容のキャッシュ
	Language  string            // 主要言語
	Framework string            // フレームワーク（検出された場合）
}

// TaskContext はタスク固有のコンテキストを保持する。
// 実行毎に1回だけ設定し、反復ループ中はキャッシュとして使用する。
type TaskContext struct {
	Description    string            // タスクの記述
	TargetFiles    []string          // 対象ファイルパス
	Constraints    []string          // 制約条件
	TargetContents map[string]string // 対象ファイルの内容キャッシュ
	TokenEstimate  int               // 全体のトークン数見積もり

	// 計画ファイル関連フィールド
	PlanFile       string // --plan で指定されたファイルパス
	PlanContent    string // 読み込まれたマークダウン全文（キャッシュ）
	PlanTokenCount int    // トークン数の見積もり（予算管理用）
}

// PlanSection は計画ファイルの ## セクションを表す。
// セクション優先度による切り捨て時に使用する。
type PlanSection struct {
	Heading  string // ## 見出しテキスト
	Content  string // セクション本文
	Priority int    // 優先度（高い方が残る）
	Tokens   int    // 推定トークン数
}
