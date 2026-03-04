package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"unicode/utf8"
)

const (
	// DefaultContextWindow はモデルのデフォルトコンテキストウィンドウサイズ（トークン数）。
	DefaultContextWindow = 128000

	// ReservedTokensForResponse はモデルの応答用に確保するトークン数。
	ReservedTokensForResponse = 4096

	// MaxPlanTokenRatio は計画ファイルがコンテキストウィンドウに占める最大割合。
	MaxPlanTokenRatio = 0.40
)

// ContextManager はコンテキストの読み込み・キャッシュ・プロンプト組み立てを担う。
// Orchestrator が1つ作成し、両エージェントから共有される。
type ContextManager struct {
	projectRoot        string
	projectCtx         *ProjectContext
	task               *TaskContext
	implementerPersona string
	reviewerPersona    string
	contextWindow      int
}

// NewContextManager は ContextManager を生成する。
func NewContextManager(projectRoot string, contextWindow int, implementerPersona, reviewerPersona string) *ContextManager {
	if contextWindow <= 0 {
		contextWindow = DefaultContextWindow
	}
	return &ContextManager{
		projectRoot:        projectRoot,
		contextWindow:      contextWindow,
		implementerPersona: implementerPersona,
		reviewerPersona:    reviewerPersona,
	}
}

// LoadProject はプロジェクトの静的コンテキストを収集する。起動時に1回だけ呼ぶ。
func (cm *ContextManager) LoadProject(ignorePatterns []string) error {
	pCtx := &ProjectContext{
		RootDir: cm.projectRoot,
		Files:   make(map[string]string),
	}

	// ディレクトリツリーを生成
	args := []string{cm.projectRoot, "-L", "3", "--charset=ascii"}
	for _, p := range ignorePatterns {
		args = append(args, "-I", p)
	}
	treeOut, err := exec.Command("tree", args...).Output()
	if err != nil {
		// フォールバック: find コマンド
		treeOut, _ = exec.Command("find", cm.projectRoot, "-maxdepth", "3",
			"-not", "-path", "*/.git/*",
			"-not", "-path", "*/node_modules/*",
			"-not", "-path", "*/vendor/*").Output()
	}
	pCtx.Tree = string(treeOut)

	// 主要設定ファイルを読み込み
	keyFiles := []string{"go.mod", "go.sum", "package.json", "tsconfig.json",
		"Makefile", "Dockerfile", ".conventions.md", "ARCHITECTURE.md"}
	for _, name := range keyFiles {
		path := filepath.Join(cm.projectRoot, name)
		data, err := os.ReadFile(path)
		if err == nil {
			pCtx.Files[name] = string(data)
		}
	}

	// 言語検出
	if _, ok := pCtx.Files["go.mod"]; ok {
		pCtx.Language = "Go"
	} else if _, ok := pCtx.Files["package.json"]; ok {
		pCtx.Language = "TypeScript/JavaScript"
	}

	cm.projectCtx = pCtx
	return nil
}

// SetTask はタスクコンテキストを設定する。実行毎に1回だけ呼ぶ。
func (cm *ContextManager) SetTask(task *TaskContext) {
	// 対象ファイルの内容をプリロード
	if task.TargetContents == nil {
		task.TargetContents = make(map[string]string)
	}
	for _, f := range task.TargetFiles {
		fullPath := filepath.Join(cm.projectRoot, f)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			task.TargetContents[f] = string(data)
		}
	}
	task.TokenEstimate = 0
	for _, c := range task.TargetContents {
		task.TokenEstimate += estimateTokens(c)
	}
	cm.task = task
}

// LoadPlan は計画ファイルを読み込み、検証し、TaskContext にキャッシュする。
// SetTask() の後、反復ループの前に1回だけ呼ぶ。
func (cm *ContextManager) LoadPlan() error {
	if cm.task == nil {
		return fmt.Errorf("SetTask must be called before LoadPlan")
	}
	if cm.task.PlanFile == "" {
		return fmt.Errorf("plan file is required: specify --plan flag or plan_file in config")
	}

	// 相対パスならプロジェクトルート基準で解決
	planPath := cm.task.PlanFile
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(cm.projectRoot, planPath)
	}

	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("failed to read plan file %q: %w", planPath, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("plan file %q is empty", planPath)
	}

	// UTF-8 妥当性チェック
	if !utf8.Valid(data) {
		return fmt.Errorf("plan file %q is not valid UTF-8", planPath)
	}

	content := string(data)
	tokenCount := estimateTokens(content)

	// コンテキストウィンドウの40%超過チェック
	availableTokens := cm.contextWindow - ReservedTokensForResponse
	maxPlanTokens := int(float64(availableTokens) * MaxPlanTokenRatio)

	if tokenCount > maxPlanTokens {
		// セクション優先度による切り捨てを試みる
		truncated := truncatePlanBySection(content, maxPlanTokens)
		if truncated != "" {
			content = truncated
			tokenCount = estimateTokens(content)
		} else {
			return fmt.Errorf(
				"plan file %q is too large: ~%d tokens (max allowed: %d tokens, %.0f%% of %d context window). "+
					"Consider splitting the plan into smaller task-specific plans",
				planPath, tokenCount, maxPlanTokens,
				MaxPlanTokenRatio*100, cm.contextWindow,
			)
		}
	}

	cm.task.PlanContent = content
	cm.task.PlanTokenCount = tokenCount

	return nil
}

// ProjectContext はプロジェクトコンテキストのゲッター。
func (cm *ContextManager) ProjectContext() *ProjectContext {
	return cm.projectCtx
}

// Task はタスクコンテキストのゲッター。
func (cm *ContextManager) Task() *TaskContext {
	return cm.task
}
