package auth

import (
	"context"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

func claudeLogin(id, orgType string) *Auth {
	return &Auth{ID: id, Provider: "claude", Status: StatusActive, Metadata: map[string]any{"organization_type": orgType}}
}

func TestClaudeFreePlan(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		auth *Auth
		want bool
	}{
		{"free", claudeLogin("a", "claude_free"), true},
		{"free, other spelling", claudeLogin("a", " Free "), true},
		{"max", claudeLogin("a", "claude_max"), false},
		{"pro", claudeLogin("a", "claude_pro"), false},
		{"no plan read yet", &Auth{ID: "a", Provider: "claude", Metadata: map[string]any{}}, false},
		{"API key", &Auth{ID: "a", Provider: "claude"}, false},
		{"not Claude", &Auth{ID: "a", Provider: "codex", Metadata: map[string]any{"organization_type": "claude_free"}}, false},
		{"nil", nil, false},
	} {
		if got := claudeFreePlan(tc.auth); got != tc.want {
			t.Errorf("%s: claudeFreePlan = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSoonestResetSkipsFreePlanLogins(t *testing.T) {
	t.Parallel()
	// The free login has no weekly reading, which would rank it first; it must not be picked at all.
	paid := weeklyAuth("paid", 3*24*time.Hour)
	paid.Provider = "claude"
	auths := []*Auth{claudeLogin("free", "claude_free"), paid}
	assertPicks(t, soonestResetPicks(t, newSoonestReset(), "claude", cliproxyexecutor.Options{}, auths, 3), "paid", "paid", "paid")

	if got, err := newSoonestReset().Pick(context.Background(), "claude", "claude-sonnet", cliproxyexecutor.Options{}, auths[:1]); err == nil {
		t.Fatalf("Pick with only a free login = %q, want an error", got.ID)
	}
}

// The hub's setup (D49): Claude logins at priority 0 and the Codex fallback at -1 serving the same model.
// A free login must not hold the Claude tier, a session bound to a login whose plan lapses must move, and a
// login whose subscription returns (the next token refresh stores the new plan) must take sessions again.
func TestManagerFreePlanFallsThroughToLowerTier(t *testing.T) {
	ctx := context.Background()
	model := "plan-gate-model"
	manager := NewManager(nil, nil, nil)
	affinity := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{Fallback: &SoonestResetSelector{}, TTL: time.Hour})
	defer affinity.Stop()
	manager.SetSelector(affinity)
	manager.RegisterExecutor(schedulerTestExecutor{provider: "claude"})
	manager.RegisterExecutor(schedulerTestExecutor{provider: "codex"})

	claudeID, codexID := "plan-gate-claude", "plan-gate-codex"
	for _, auth := range []*Auth{
		{ID: claudeID, Provider: "claude", Status: StatusActive, Metadata: map[string]any{"organization_type": "claude_max"}},
		{ID: codexID, Provider: "codex", Status: StatusActive, Attributes: map[string]string{"priority": "-1"}},
	} {
		if _, err := manager.Register(WithSkipPersist(ctx), auth); err != nil {
			t.Fatalf("Register(%s): %v", auth.ID, err)
		}
		registry.GetGlobalRegistry().RegisterClient(auth.ID, auth.Provider, []*registry.ModelInfo{{ID: model}})
		t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient(auth.ID) })
	}
	session := func(id string) cliproxyexecutor.Options {
		return cliproxyexecutor.Options{Metadata: map[string]any{cliproxyexecutor.DerivedSessionIDMetadataKey: id}}
	}
	pick := func(opts cliproxyexecutor.Options) string {
		t.Helper()
		auth, _, _, err := manager.pickNextMixed(ctx, []string{"claude", "codex"}, model, opts, nil)
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		return auth.ID
	}
	setPlan := func(orgType string) {
		t.Helper()
		auth, ok := manager.GetByID(claudeID)
		if !ok {
			t.Fatalf("GetByID(%s) not found", claudeID)
		}
		auth.Metadata["organization_type"] = orgType
		if _, err := manager.Update(WithSkipPersist(ctx), auth); err != nil {
			t.Fatalf("Update: %v", err)
		}
	}

	if got := pick(session("one")); got != claudeID {
		t.Fatalf("on Max: pick = %q, want %q", got, claudeID)
	}
	setPlan("claude_free")
	if got := pick(session("two")); got != codexID {
		t.Fatalf("new session on a free plan: pick = %q, want the fallback %q", got, codexID)
	}
	if got := pick(session("one")); got != codexID {
		t.Fatalf("session bound before the plan lapsed: pick = %q, want the fallback %q", got, codexID)
	}
	setPlan("claude_max")
	if got := pick(session("three")); got != claudeID {
		t.Fatalf("new session after the subscription returned: pick = %q, want %q", got, claudeID)
	}
}
