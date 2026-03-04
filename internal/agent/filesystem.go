package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
