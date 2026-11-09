package helps

import (
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
)

// ClaudeModelFingerprintConfig supplies the supported CLI version for Fable 5.2
// cloaking without changing native-client detection or explicit operator defaults.
// Fable 5.2 rejects the legacy 2.1.280 default (it requires at least 2.1.290).
func ClaudeModelFingerprintConfig(cfg *config.Config, model string) *config.Config {
	if !IsClaudeFable52Model(model) || (cfg != nil && strings.TrimSpace(cfg.ClaudeHeaderDefaults.UserAgent) != "") {
		return cfg
	}
	var scoped config.Config
	if cfg != nil {
		scoped = *cfg
	}
	scoped.ClaudeHeaderDefaults.UserAgent = "claude-cli/2.1.321 (external, cli)"
	return &scoped
}

// IsClaudeFable52Model includes Mythos 5.2, which shares Fable 5.2's
// adaptive thinking, prefix binding, and progress display capabilities.
func IsClaudeFable52Model(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	for _, target := range []string{"fable-5-2", "fable-5.2", "mythos-5-2", "mythos-5.2"} {
		idx := strings.Index(m, target)
		if idx == -1 {
			continue
		}
		next := idx + len(target)
		if next == len(m) || m[next] < '0' || m[next] > '9' {
			return true
		}
	}
	return false
}
