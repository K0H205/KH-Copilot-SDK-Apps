package context

import (
	"fmt"
	"strings"
)

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
