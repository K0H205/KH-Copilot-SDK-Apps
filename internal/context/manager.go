package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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

// BuildImplementerPrompt は実装者エージェント用のプロンプトを組み立てる。
func (cm *ContextManager) BuildImplementerPrompt(iteration int, feedback string) string {
	var b strings.Builder

	// 計画セクション（最高優先度の L2 コンテンツ）
	if cm.task.PlanContent != "" {
		b.WriteString("## Implementation Plan (AUTHORITATIVE SPECIFICATION)\n\n")
		b.WriteString("The following plan is your primary set of instructions. ")
		b.WriteString("You MUST implement exactly what this plan specifies. ")
		b.WriteString("Every requirement, file change, and constraint described in this plan ")
		b.WriteString("takes precedence over any other context.\n\n")
		b.WriteString("```markdown\n")
		b.WriteString(cm.task.PlanContent)
		b.WriteString("\n```\n\n")
		b.WriteString("--- END OF PLAN ---\n\n")
	}

	// L2: タスクコンテキスト（計画ファイルがタスク記述を兼ねる）
	if len(cm.task.TargetFiles) > 0 || len(cm.task.Constraints) > 0 {
		b.WriteString("## Task Context\n\n")

		if len(cm.task.TargetFiles) > 0 {
			b.WriteString("**Target Files:**\n")
			for _, f := range cm.task.TargetFiles {
				b.WriteString(fmt.Sprintf("- `%s`\n", f))
			}
			b.WriteString("\n")
		}

		if len(cm.task.Constraints) > 0 {
			b.WriteString("**Constraints:**\n")
			for _, c := range cm.task.Constraints {
				b.WriteString(fmt.Sprintf("- %s\n", c))
			}
			b.WriteString("\n")
		}
	}

	// L1: プロジェクトコンテキスト
	if cm.projectCtx != nil {
		b.WriteString("## Project Context\n\n")
		b.WriteString("### Directory Structure\n```\n")
		tree := cm.projectCtx.Tree
		budget := cm.budgetForSection("project")
		if estimateTokens(tree) > budget {
			tree = truncateText(tree, budget)
		}
		b.WriteString(tree)
		b.WriteString("```\n\n")

		for path, content := range cm.projectCtx.Files {
			b.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", path, content))
		}
	}

	// L4: レビューフィードバック（2回目以降）
	if iteration > 0 && feedback != "" {
		b.WriteString("## Previous Review Feedback\n\n")
		b.WriteString(fmt.Sprintf("This is iteration %d. The reviewer provided the following feedback ", iteration))
		b.WriteString("on your previous implementation. Address every point:\n\n")
		b.WriteString(feedback)
		b.WriteString("\n\n")
	}

	// 対象ファイル内容
	if len(cm.task.TargetContents) > 0 {
		b.WriteString("## Current File Contents\n\n")
		for path, content := range cm.task.TargetContents {
			b.WriteString(fmt.Sprintf("### `%s`\n```\n%s\n```\n\n", path, content))
		}
	}

	// 指示
	b.WriteString("## Instructions\n\n")
	if iteration == 0 {
		b.WriteString("Generate the implementation for the task described above. ")
		b.WriteString("Output the complete file contents for each file you create or modify. ")
		b.WriteString("Use the tools available to you to read any additional files you need.\n")
	} else {
		b.WriteString("Revise your implementation based on the review feedback above. ")
		b.WriteString("Address every issue. Output the complete revised file contents.\n")
	}

	return b.String()
}

// BuildReviewerPrompt はレビュアーエージェント用のプロンプトを組み立てる。
func (cm *ContextManager) BuildReviewerPrompt(iteration int, codeOutput string) string {
	var b strings.Builder

	// 計画セクション（検証リファレンス）
	if cm.task.PlanContent != "" {
		b.WriteString("## Implementation Plan (VERIFICATION REFERENCE)\n\n")
		b.WriteString("The following plan is the authoritative specification that the ")
		b.WriteString("implementer was instructed to follow. Your review MUST verify that ")
		b.WriteString("the implementation satisfies every requirement in this plan. ")
		b.WriteString("For each item in the plan, confirm whether it has been correctly ")
		b.WriteString("implemented, partially implemented, or missed entirely.\n\n")
		b.WriteString("```markdown\n")
		b.WriteString(cm.task.PlanContent)
		b.WriteString("\n```\n\n")
		b.WriteString("--- END OF PLAN ---\n\n")
	}

	// L2: タスクコンテキスト（計画ファイルがタスク記述を兼ねる）
	if len(cm.task.Constraints) > 0 {
		b.WriteString("## Task Context\n\n")
		b.WriteString("**Constraints:**\n")
		for _, c := range cm.task.Constraints {
			b.WriteString(fmt.Sprintf("- %s\n", c))
		}
		b.WriteString("\n")
	}

	// L4: 実装者のコード出力
	b.WriteString("## Implementation to Review\n\n")
	b.WriteString(fmt.Sprintf("This is iteration %d. Review the following code output:\n\n", iteration))
	b.WriteString(codeOutput)
	b.WriteString("\n\n")

	// L1: プロジェクトコンテキスト（レビュアー向けは軽量）
	if cm.projectCtx != nil {
		b.WriteString("## Project Context\n\n")
		b.WriteString("### Directory Structure\n```\n")
		b.WriteString(cm.projectCtx.Tree)
		b.WriteString("```\n\n")
	}

	// レビューチェックリスト
	if cm.task.PlanContent != "" {
		b.WriteString("## Review Checklist\n\n")
		b.WriteString("Structure your review as follows:\n")
		b.WriteString("1. **Plan Compliance**: For each section/requirement in the plan, state whether it is DONE, PARTIAL, or MISSING.\n")
		b.WriteString("2. **Correctness**: Are there logic errors, edge cases, or bugs?\n")
		b.WriteString("3. **Code Quality**: Does the code follow the project conventions?\n")
		b.WriteString("4. **Verdict**: APPROVE if all plan items are DONE and code is correct. REQUEST_CHANGES otherwise.\n\n")
	} else {
		b.WriteString("## Review Instructions\n\n")
		b.WriteString("Review the code against the task requirements. ")
		b.WriteString("If the code is acceptable, respond with APPROVED. ")
		b.WriteString("If there are issues, respond with NEEDS_REVISION and list all issues.\n\n")
	}

	return b.String()
}

// ProjectContext はプロジェクトコンテキストのゲッター。
func (cm *ContextManager) ProjectContext() *ProjectContext {
	return cm.projectCtx
}

// Task はタスクコンテキストのゲッター。
func (cm *ContextManager) Task() *TaskContext {
	return cm.task
}

// budgetForSection は各セクションに割り当て可能なトークン数を返す。
func (cm *ContextManager) budgetForSection(section string) int {
	total := cm.contextWindow - ReservedTokensForResponse

	// 固定割り当て（切り捨て不可）
	fixedUsed := estimateTokens(cm.implementerPersona) + cm.task.PlanTokenCount
	remaining := total - fixedUsed
	if remaining < 0 {
		remaining = 0
	}

	switch section {
	case "feedback":
		return int(float64(remaining) * 0.25)
	case "task":
		return int(float64(remaining) * 0.10)
	case "project":
		return int(float64(remaining) * 0.25)
	case "files":
		return int(float64(remaining) * 0.40)
	default:
		return remaining
	}
}

// estimateTokens はテキストのトークン数を概算する。
// 1トークン ≈ 4文字のヒューリスティックを使用。
func estimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return len(text) / 4
}

// truncateText はテキストを指定トークン数以内に切り詰める。
func truncateText(text string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "\n... (truncated)"
}

// truncatePlanBySection は計画ファイルを ## 見出しで分割し、
// 優先度の低いセクションから除去してトークン予算内に収める。
// 切り捨てに成功した場合は切り詰められた内容を、失敗した場合は空文字を返す。
func truncatePlanBySection(content string, maxTokens int) string {
	sections := splitByH2(content)
	if len(sections) == 0 {
		return ""
	}

	// 各セクションに優先度を割り当て
	for i := range sections {
		sections[i].Priority = sectionPriority(sections[i].Heading)
		sections[i].Tokens = estimateTokens(sections[i].Content)
	}

	// 全セクションのトークン数を計算
	totalTokens := 0
	for _, s := range sections {
		totalTokens += s.Tokens
	}

	if totalTokens <= maxTokens {
		return content // すでに予算内
	}

	// 優先度でソート（低優先度が先頭）
	sorted := make([]PlanSection, len(sections))
	copy(sorted, sections)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	// 低優先度から除去
	removed := make(map[string]bool)
	for _, s := range sorted {
		if totalTokens <= maxTokens {
			break
		}
		removed[s.Heading] = true
		totalTokens -= s.Tokens
	}

	// 元の順序で残ったセクションを再構築
	var b strings.Builder
	for _, s := range sections {
		if !removed[s.Heading] {
			b.WriteString(s.Content)
		}
	}

	result := b.String()
	if estimateTokens(result) > maxTokens {
		return "" // まだ超過 → 切り捨て不可
	}
	return result
}

// splitByH2 はマークダウンを ## 見出しで分割する。
func splitByH2(content string) []PlanSection {
	lines := strings.Split(content, "\n")
	var sections []PlanSection
	var current *PlanSection

	// 見出し前のテキスト（プリアンブル）
	preamble := &PlanSection{Heading: "__preamble__", Priority: 100}
	var preambleLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// 前のセクションを保存
			if current != nil {
				sections = append(sections, *current)
			} else if len(preambleLines) > 0 {
				preamble.Content = strings.Join(preambleLines, "\n") + "\n"
				sections = append(sections, *preamble)
			}
			heading := strings.TrimPrefix(line, "## ")
			heading = strings.TrimSpace(heading)
			current = &PlanSection{
				Heading: heading,
				Content: line + "\n",
			}
		} else {
			if current != nil {
				current.Content += line + "\n"
			} else {
				preambleLines = append(preambleLines, line)
			}
		}
	}

	// 最後のセクションを保存
	if current != nil {
		sections = append(sections, *current)
	} else if len(preambleLines) > 0 {
		preamble.Content = strings.Join(preambleLines, "\n") + "\n"
		sections = append(sections, *preamble)
	}

	return sections
}

// sectionPriority は見出しテキストから優先度を返す。
// 数値が大きいほど優先度が高い（最後まで残る）。
func sectionPriority(heading string) int {
	lower := strings.ToLower(heading)

	// 高優先度: 実装手順・要件
	highPriority := []string{"実装手順", "実装順序", "steps", "implementation", "要件", "requirements", "仕様", "spec"}
	for _, kw := range highPriority {
		if strings.Contains(lower, kw) {
			return 90
		}
	}

	// 中優先度: 概要・設計
	medPriority := []string{"概要", "overview", "設計", "design", "アーキテクチャ", "architecture"}
	for _, kw := range medPriority {
		if strings.Contains(lower, kw) {
			return 50
		}
	}

	// 低優先度: 背景・検証
	lowPriority := []string{"背景", "context", "background", "検証", "verification", "テスト", "test", "参考", "reference"}
	for _, kw := range lowPriority {
		if strings.Contains(lower, kw) {
			return 20
		}
	}

	// プリアンブル（# タイトル等）は高優先度
	if heading == "__preamble__" {
		return 100
	}

	// デフォルト: 中程度
	return 40
}
