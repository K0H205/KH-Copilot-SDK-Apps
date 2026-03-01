package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AgentConfig はエージェント固有の設定。
type AgentConfig struct {
	SystemPrompt string `yaml:"system_prompt"`
}

// ContextConfig はコンテキスト収集関連の設定。
type ContextConfig struct {
	MaxProjectSummaryTokens int      `yaml:"max_project_summary_tokens"`
	MaxCodeTokens           int      `yaml:"max_code_tokens"`
	MaxReviewTokens         int      `yaml:"max_review_tokens"`
	IgnorePatterns          []string `yaml:"ignore_patterns"`
}

// Config はアプリケーション全体の設定。
type Config struct {
	MaxIterations int           `yaml:"max_iterations"`
	WorkDir       string        `yaml:"work_dir"`
	PlanFile      string        `yaml:"plan_file,omitempty"`
	Implementer   AgentConfig   `yaml:"implementer"`
	Reviewer      AgentConfig   `yaml:"reviewer"`
	Context       ContextConfig `yaml:"context"`
}

// DefaultConfig はデフォルト設定を返す。
func DefaultConfig() *Config {
	return &Config{
		MaxIterations: 5,
		WorkDir:       ".",
		Implementer: AgentConfig{
			SystemPrompt: "You are an expert software implementer. Generate high-quality, production-ready code based on task descriptions and project context. When revising based on review feedback, address EVERY item mentioned.",
		},
		Reviewer: AgentConfig{
			SystemPrompt: "You are an expert code reviewer. Review generated code against task requirements, coding conventions, and architecture guidelines. Be thorough but fair. If all requirements are met, respond with APPROVED. Otherwise, respond with NEEDS_REVISION and list all issues.",
		},
		Context: ContextConfig{
			MaxProjectSummaryTokens: 4000,
			MaxCodeTokens:           8000,
			MaxReviewTokens:         4000,
			IgnorePatterns: []string{
				"vendor/",
				"node_modules/",
				".git/",
				"*.exe",
				"*.bin",
			},
		},
	}
}

// Load は指定されたパスからYAMLファイルを読み込み、Config を返す。
// ファイルが存在しない場合はデフォルト設定を返す。
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 5
	}

	return cfg, nil
}
