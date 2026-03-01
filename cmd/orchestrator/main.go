package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/config"
	appctx "github.com/K0H205/KH-Copilot-SDK-Apps/internal/context"
	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/orchestrator"
)

func main() {
	// CLI フラグ定義
	configPath := flag.String("config", "config.yaml", "設定ファイルパス")
	planFile := flag.String("plan", "", "マークダウン計画ファイルのパス（必須）")
	workDir := flag.String("workdir", ".", "作業ディレクトリ")
	flag.Parse()

	// 設定読み込み
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// CLI引数で上書き
	if *workDir != "" {
		cfg.WorkDir = *workDir
	}

	// 作業ディレクトリを絶対パスに変換
	absWorkDir, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		log.Fatalf("Failed to resolve work directory: %v", err)
	}
	cfg.WorkDir = absWorkDir

	// 計画ファイルのパス解決（CLI → config.yaml のフォールバック）
	resolvedPlanFile := *planFile
	if resolvedPlanFile == "" {
		resolvedPlanFile = cfg.PlanFile
	}

	// 計画ファイルは必須
	if resolvedPlanFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --plan is required (or set plan_file in config.yaml)")
		flag.Usage()
		os.Exit(1)
	}

	// 計画ファイルの存在確認
	planPath := resolvedPlanFile
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(absWorkDir, planPath)
	}
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		log.Fatalf("Plan file not found: %s", planPath)
	}
	resolvedPlanFile = planPath

	// タスクコンテキストを構築（計画ファイルが全ての指示の源泉）
	task := &appctx.TaskContext{
		PlanFile: resolvedPlanFile,
	}

	// オーケストレーション実行
	orch := orchestrator.New(*cfg)
	result, err := orch.Run(context.Background(), task)
	if err != nil {
		log.Printf("Orchestration error: %v", err)
		os.Exit(1)
	}

	if result.Approved {
		fmt.Println("Orchestration completed: Code approved by reviewer")
	} else {
		fmt.Println("Orchestration completed: Max iterations reached")
	}
}
