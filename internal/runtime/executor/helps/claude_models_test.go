package helps

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
)

func TestIsClaudeFable52Model(t *testing.T) {
	for _, model := range []string{"claude-fable-5-2", "CLAUDE-FABLE-5.2", "claude-fable-5-2-20261109", "claude-fable-5-2[1m]", "claude-mythos-5-2", "claude-mythos-5.2(high)"} {
		if !IsClaudeFable52Model(model) {
			t.Errorf("%q should have Fable 5.2 capabilities", model)
		}
	}
	for _, model := range []string{"claude-fable-5-1", "claude-mythos-5-1", "claude-fable-5-20", "claude-mythos-5.21", "claude-opus-5-5", ""} {
		if IsClaudeFable52Model(model) {
			t.Errorf("%q should not have Fable 5.2 capabilities", model)
		}
	}
}

func TestClaudeModelFingerprintConfigPreservesOperatorAndOtherModelDefaults(t *testing.T) {
	cfg := &config.Config{ClaudeHeaderDefaults: config.ClaudeHeaderDefaults{PackageVersion: "0.128.0"}}
	scoped := ClaudeModelFingerprintConfig(cfg, "claude-fable-5-2")
	if DefaultClaudeVersion(scoped) != "2.1.321" || scoped.ClaudeHeaderDefaults.PackageVersion != "0.128.0" {
		t.Fatalf("Fable model defaults = %#v", scoped.ClaudeHeaderDefaults)
	}
	if cfg.ClaudeHeaderDefaults.UserAgent != "" {
		t.Fatal("Fable defaults mutated the shared config")
	}
	if got := ClaudeModelFingerprintConfig(cfg, "claude-fable-5-1"); got != cfg || DefaultClaudeVersion(got) != "2.1.280" {
		t.Fatal("Fable 5.1 defaults changed")
	}
	cfg.ClaudeHeaderDefaults.UserAgent = "claude-cli/2.2.0 (external, cli)"
	if got := ClaudeModelFingerprintConfig(cfg, "claude-fable-5-2"); got != cfg || DefaultClaudeVersion(got) != "2.2.0" {
		t.Fatal("explicit operator client version was overridden")
	}
}
