package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/message"
)

// Implementer は実装者エージェント。
// タスク記述と計画ファイルに基づいてコードを生成し、
// レビューフィードバックを受けて修正を繰り返す。
type Implementer struct {
	BaseAgent
	implCh   chan<- message.Message // 実装結果を送信
	reviewCh <-chan message.Message // レビュー結果を受信
}

// Run は実装者エージェントのメインループ。
func (impl *Implementer) Run(ctx context.Context) error {
	maxIterations := 5
	feedback := ""

	for iteration := 0; iteration < maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// プロンプトを組み立て
		prompt := impl.CtxMgr.BuildImplementerPrompt(iteration, feedback)

		log.Printf("[Implementer] Iteration %d: sending prompt (%d chars)", iteration, len(prompt))

		// Copilot SDK にプロンプトを送信
		response, err := SendPrompt(ctx, impl.Config.SystemPrompt, prompt)
		if err != nil {
			log.Printf("[Implementer] Iteration %d: SDK call failed (expected in dev): %v", iteration, err)
			// 開発中はプロンプト自体を実装結果として送信
			response = fmt.Sprintf("[Implementer Iteration %d] Prompt prepared (%d chars). SDK integration pending.", iteration, len(prompt))
		}

		// 実装結果をレビュアーに送信
		impl.implCh <- message.Message{
			Type:      message.TypeImplementation,
			Content:   response,
			Iteration: iteration,
			Timestamp: time.Now(),
		}

		log.Printf("[Implementer] Iteration %d: code sent to reviewer", iteration)

		// レビューフィードバックを待機
		select {
		case <-ctx.Done():
			return ctx.Err()
		case review := <-impl.reviewCh:
			if review.Type == message.TypeApproved {
				log.Printf("[Implementer] Code approved at iteration %d", iteration)
				return nil
			}
			if review.Type == message.TypeError {
				return fmt.Errorf("reviewer error: %s", review.Content)
			}
			feedback = review.Content
			log.Printf("[Implementer] Received review feedback at iteration %d", iteration)
		}
	}

	return fmt.Errorf("max iterations (%d) reached without approval", maxIterations)
}
