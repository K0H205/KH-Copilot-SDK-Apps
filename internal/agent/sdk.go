package agent

import (
	"context"
	"fmt"
)

// SendPrompt はCopilot SDKセッションにプロンプトを送信する。
// TODO: Copilot SDK の copilot.Client を統合する。
func SendPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Copilot SDK 統合のプレースホルダー。
	// 実際の統合時は copilot.NewClient() → session.Send() → response を使用する。
	_ = ctx
	_ = systemPrompt
	_ = userPrompt
	return "", fmt.Errorf("Copilot SDK integration not yet available; use mock for testing")
}
