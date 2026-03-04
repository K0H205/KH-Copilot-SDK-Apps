package main

import "flag"

// CLIOptions はコマンドラインオプションを保持する。
// CLI引数の解析に関する唯一の責務を持つ。
type CLIOptions struct {
	ConfigPath string
	PlanFile   string
	WorkDir    string
}

// parseCLI はコマンドライン引数を解析し、CLIOptions を返す。
func parseCLI() CLIOptions {
	configPath := flag.String("config", "config.yaml", "設定ファイルパス")
	planFile := flag.String("plan", "", "マークダウン計画ファイルのパス（必須）")
	workDir := flag.String("workdir", ".", "作業ディレクトリ")
	flag.Parse()

	return CLIOptions{
		ConfigPath: *configPath,
		PlanFile:   *planFile,
		WorkDir:    *workDir,
	}
}
