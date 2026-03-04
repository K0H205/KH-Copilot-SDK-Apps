package agent

import (
	"context"

	"github.com/K0H205/KH-Copilot-SDK-Apps/internal/config"
	appctx "github.com/K0H205/KH-Copilot-SDK-Apps/internal/context"
)

// Agent はエージェントの共通インターフェース。
type Agent interface {
	Run(ctx context.Context) error
}

// BaseAgent はエージェントの共通フィールドを保持する。
type BaseAgent struct {
	Config      config.AgentConfig
	CtxMgr      *appctx.ContextManager
	ProjectRoot string
}
