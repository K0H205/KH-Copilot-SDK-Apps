package context

import "strings"

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
	sortByPriorityAsc(sorted)

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

// sortByPriorityAsc は PlanSection スライスを優先度の昇順（低優先度が先頭）でソートする。
func sortByPriorityAsc(sections []PlanSection) {
	n := len(sections)
	for i := 1; i < n; i++ {
		key := sections[i]
		j := i - 1
		for j >= 0 && sections[j].Priority > key.Priority {
			sections[j+1] = sections[j]
			j--
		}
		sections[j+1] = key
	}
}
