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

// run はアプリケーションのエントリポイント。
// CLI解析 → 設定読み込み → タスク構築 → オーケストレーション実行 の順に処理する。
func run() error {
	opts := parseCLI()

	cfg, err := loadConfig(opts)
	if err != nil {
		return err
	}

	task, err := resolveTask(opts, cfg)
	if err != nil {
		return err
	}

	return execute(cfg, task)
}

// loadConfig は設定ファイルを読み込み、CLI オプションで上書きする。
// 設定の読み込みと上書きに関する唯一の責務を持つ。
func loadConfig(opts CLIOptions) (*config.Config, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if opts.WorkDir != "" {
		cfg.WorkDir = opts.WorkDir
	}

	absWorkDir, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve work directory: %w", err)
	}
	cfg.WorkDir = absWorkDir

	return cfg, nil
}

// resolveTask はCLIオプションと設定からTaskContextを構築・検証する。
// 計画ファイルのパス解決と存在確認に関する唯一の責務を持つ。
func resolveTask(opts CLIOptions, cfg *config.Config) (*appctx.TaskContext, error) {
	planFile := opts.PlanFile
	if planFile == "" {
		planFile = cfg.PlanFile
	}

	if planFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --plan is required (or set plan_file in config.yaml)")
		flag.Usage()
		return nil, fmt.Errorf("--plan is required")
	}

	planPath := planFile
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(cfg.WorkDir, planPath)
	}
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plan file not found: %s", planPath)
	}

	return &appctx.TaskContext{PlanFile: planPath}, nil
}

// execute はオーケストレーションを実行し、結果を報告する。
// オーケストレーションの実行と結果出力に関する唯一の責務を持つ。
func execute(cfg *config.Config, task *appctx.TaskContext) error {
	orch := orchestrator.New(*cfg)
	result, err := orch.Run(context.Background(), task)
	if err != nil {
		log.Printf("Orchestration error: %v", err)
		return err
	}

	if result.Approved {
		fmt.Println("Orchestration completed: Code approved by reviewer")
	} else {
		fmt.Println("Orchestration completed: Max iterations reached")
	}
	return nil
}
