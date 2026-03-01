package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/config"
	appctx "github.com/K0H205/KH-Copilot-SDK-Apps/internal/context"
	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/message"
)

// Agent はエージェントの共通インターフェース。
type Agent interface {
	Run(ctx context.Context) error
}

// BaseAgent はエージェントの共通フィールドを保持する。
type BaseAgent struct {
	Config    config.AgentConfig
	CtxMgr   *appctx.ContextManager
	ProjectRoot string
}

// ReadFile はプロジェクト内のファイルを安全に読み取る。
// パストラバーサルを防止する。
func (ba *BaseAgent) ReadFile(relPath string) (string, error) {
	absPath := filepath.Join(ba.ProjectRoot, filepath.Clean(relPath))
	if !strings.HasPrefix(absPath, ba.ProjectRoot) {
		return "", fmt.Errorf("path traversal denied: %s", relPath)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListFiles はプロジェクト内のディレクトリの内容を一覧する。
func (ba *BaseAgent) ListFiles(relDir string) ([]string, error) {
	absDir := filepath.Join(ba.ProjectRoot, filepath.Clean(relDir))
	if !strings.HasPrefix(absDir, ba.ProjectRoot) {
		return nil, fmt.Errorf("path traversal denied: %s", relDir)
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		prefix := "  "
		if e.IsDir() {
			prefix = "d "
		}
		result = append(result, prefix+e.Name())
	}
	return result, nil
}

// SearchCode はプロジェクト内でパターンを検索する。
func (ba *BaseAgent) SearchCode(pattern, relDir string) (string, error) {
	searchDir := filepath.Join(ba.ProjectRoot, filepath.Clean(relDir))
	if !strings.HasPrefix(searchDir, ba.ProjectRoot) {
		return "", fmt.Errorf("path traversal denied: %s", relDir)
	}
	out, err := exec.Command("grep", "-rn",
		"--include=*.go", "--include=*.ts", "--include=*.js", "--include=*.py",
		pattern, searchDir).Output()
	if err != nil {
		return "No matches found", nil
	}
	result := string(out)
	if len(result) > 8000 {
		result = result[:8000] + "\n... (truncated)"
	}
	return result, nil
}

// SendPrompt はCopilot SDKセッションにプロンプトを送信する。
// 現時点ではSDKの代替としてプロンプトをログ出力する。
// TODO: Copilot SDK の copilot.Client を統合する。
func SendPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Copilot SDK 統合のプレースホルダー。
	// 実際の統合時は copilot.NewClient() → session.Send() → response を使用する。
	_ = ctx
	_ = systemPrompt
	_ = userPrompt
	return "", fmt.Errorf("Copilot SDK integration not yet available; use mock for testing")
}

// NewImplementer は実装者エージェントを生成する。
func NewImplementer(
	ctxMgr *appctx.ContextManager,
	projectRoot string,
	implCh chan<- message.Message,
	reviewCh <-chan message.Message,
	cfg config.AgentConfig,
) *Implementer {
	return &Implementer{
		BaseAgent: BaseAgent{
			Config:      cfg,
			CtxMgr:      ctxMgr,
			ProjectRoot: projectRoot,
		},
		implCh:   implCh,
		reviewCh: reviewCh,
	}
}

// NewReviewer はレビュアーエージェントを生成する。
func NewReviewer(
	ctxMgr *appctx.ContextManager,
	projectRoot string,
	implCh <-chan message.Message,
	reviewCh chan<- message.Message,
	cfg config.AgentConfig,
) *Reviewer {
	return &Reviewer{
		BaseAgent: BaseAgent{
			Config:      cfg,
			CtxMgr:      ctxMgr,
			ProjectRoot: projectRoot,
		},
		implCh:   implCh,
		reviewCh: reviewCh,
	}
}
