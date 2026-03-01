package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/message"
)

// Reviewer はレビュアーエージェント。
// 実装者が生成したコードを計画ファイルとタスク要件に照らしてレビューし、
// 承認またはフィードバックを返す。
type Reviewer struct {
	BaseAgent
	implCh   <-chan message.Message // 実装結果を受信
	reviewCh chan<- message.Message // レビュー結果を送信
}

// Run はレビュアーエージェントのメインループ。
func (rev *Reviewer) Run(ctx context.Context) error {
	for {
		// 実装者からのコードを待機
		var code message.Message
		select {
		case <-ctx.Done():
			return ctx.Err()
		case code = <-rev.implCh:
		}

		if code.Type == message.TypeError {
			return fmt.Errorf("implementer error: %s", code.Content)
		}

		log.Printf("[Reviewer] Reviewing iteration %d", code.Iteration)

		// レビュープロンプトを組み立て
		prompt := rev.CtxMgr.BuildReviewerPrompt(code.Iteration, code.Content)

		// Copilot SDK にプロンプトを送信
		response, err := SendPrompt(ctx, rev.Config.SystemPrompt, prompt)
		if err != nil {
			log.Printf("[Reviewer] Iteration %d: SDK call failed (expected in dev): %v", code.Iteration, err)
			// 開発中はプロンプト準備状況を返す
			response = fmt.Sprintf("[Reviewer Iteration %d] Review prompt prepared (%d chars). SDK integration pending.\nNEEDS_REVISION: SDK not yet integrated.", code.Iteration, len(prompt))
		}

		// 承認判定
		approved := strings.HasPrefix(strings.TrimSpace(response), "APPROVED") ||
			strings.HasPrefix(strings.TrimSpace(response), "APPROVE")

		if approved {
			log.Printf("[Reviewer] Code approved at iteration %d", code.Iteration)
			rev.reviewCh <- message.Message{
				Type:      message.TypeApproved,
				Content:   response,
				Iteration: code.Iteration,
				Timestamp: time.Now(),
			}
			return nil
		}

		log.Printf("[Reviewer] Requesting revision at iteration %d", code.Iteration)
		rev.reviewCh <- message.Message{
			Type:      message.TypeReview,
			Content:   response,
			Iteration: code.Iteration,
			Timestamp: time.Now(),
		}
	}
}
