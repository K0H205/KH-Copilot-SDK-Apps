package message

import "time"

// MessageType はエージェント間メッセージの種別を表す。
type MessageType string

const (
	TypeImplementation MessageType = "implementation" // 実装者→レビュアー: コード
	TypeReview         MessageType = "review"         // レビュアー→実装者: フィードバック
	TypeApproved       MessageType = "approved"       // レビュアー→オーケストレーター: 承認
	TypeError          MessageType = "error"          // エラー通知
)

// Message はエージェント間で交換するメッセージ。
type Message struct {
	Type      MessageType
	Content   string    // コードまたはレビューコメント
	Iteration int       // 何回目の反復か
	Timestamp time.Time
}
