package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/agent"
	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/config"
	appctx "github.com/K0H205/KH-Copilot-SDK-Apps/internal/context"
	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/message"
	"golang.org/x/sync/errgroup"
)

// Result はオーケストレーションの最終結果。
type Result struct {
	FinalCode     string // 最終的なコード出力
	FinalReview   string // 最終レビュー
	Iterations    int    // 実行された反復回数
	Approved      bool   // レビュアーに承認されたか
}

// Orchestrator は実装者とレビュアーの並列実行を管理する。
type Orchestrator struct {
	config config.Config
}

// New は Orchestrator を生成する。
func New(cfg config.Config) *Orchestrator {
	return &Orchestrator{config: cfg}
}

// Run はオーケストレーションのメインループ。
// 1. プロジェクトコンテキストを収集
// 2. タスクコンテキストを設定
// 3. 計画ファイルを読み込み
// 4. 実装者とレビュアーを並列起動
// 5. 収束またはタイムアウトまで待機
func (o *Orchestrator) Run(ctx context.Context, task *appctx.TaskContext) (*Result, error) {
	log.Println("[Orchestrator] Starting orchestration...")

	// ContextManager を生成
	ctxMgr := appctx.NewContextManager(
		o.config.WorkDir,
		appctx.DefaultContextWindow,
		o.config.Implementer.SystemPrompt,
		o.config.Reviewer.SystemPrompt,
	)

	// L1: プロジェクトコンテキストを収集
	log.Println("[Orchestrator] Loading project context...")
	if err := ctxMgr.LoadProject(o.config.Context.IgnorePatterns); err != nil {
		return nil, fmt.Errorf("loading project context: %w", err)
	}

	// L2: タスクコンテキストを設定
	log.Println("[Orchestrator] Setting task context...")
	ctxMgr.SetTask(task)

	// 計画ファイルを読み込み
	if task.PlanFile != "" {
		log.Printf("[Orchestrator] Loading plan file: %s", task.PlanFile)
	}
	if err := ctxMgr.LoadPlan(); err != nil {
		return nil, fmt.Errorf("loading plan: %w", err)
	}
	if task.PlanContent != "" {
		log.Printf("[Orchestrator] Plan loaded (%d tokens estimated)", task.PlanTokenCount)
	}

	// チャネル生成
	implCh := make(chan message.Message, 1)
	reviewCh := make(chan message.Message, 1)

	// エージェント生成
	impl := agent.NewImplementer(ctxMgr, o.config.WorkDir, implCh, reviewCh, o.config.Implementer)
	rev := agent.NewReviewer(ctxMgr, o.config.WorkDir, implCh, reviewCh, o.config.Reviewer)

	// 並列実行
	log.Println("[Orchestrator] Starting agents...")
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return impl.Run(gctx) })
	g.Go(func() error { return rev.Run(gctx) })

	err := g.Wait()

	result := &Result{
		Approved: err == nil,
	}

	if err != nil {
		log.Printf("[Orchestrator] Orchestration completed with error: %v", err)
		return result, err
	}

	log.Println("[Orchestrator] Orchestration completed successfully")
	return result, nil
}
