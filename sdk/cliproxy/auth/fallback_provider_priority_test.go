package auth

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

// The hub's setup: three Claude logins routed soonest-reset behind session affinity, and a Codex login
// at a lower priority that also serves the Claude model name through an alias. Codex never reports a
// weekly reset, so soonest-reset alone would rank it first; priority must keep it out until every
// Claude login is unavailable, and new sessions must return to Claude once one recovers.
func TestLowerPriorityFallbackProviderOnlyWhenEveryPrimaryIsUnavailable(t *testing.T) {
	ctx := context.Background()
	primary := "fallback-test-claude"
	fallback := "fallback-test-codex"
	model := "fallback-test-opus"

	manager := NewManager(nil, nil, nil)
	affinity := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{
		Fallback: &SoonestResetSelector{},
		TTL:      time.Hour,
	})
	defer affinity.Stop()
	manager.SetSelector(affinity)
	manager.RegisterExecutor(schedulerTestExecutor{provider: primary})
	manager.RegisterExecutor(schedulerTestExecutor{provider: fallback})

	reset := strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10)
	claudeIDs := []string{primary + "-a", primary + "-b", primary + "-c"}
	codexID := fallback + "-1"
	auths := []*Auth{{ID: codexID, Provider: fallback, Status: StatusActive, Attributes: map[string]string{"priority": "-1"}}}
	for _, id := range claudeIDs {
		auths = append(auths, &Auth{ID: id, Provider: primary, Status: StatusActive,
			Quota: QuotaState{Signals: map[string]string{weeklyResetSignal: reset}}})
	}
	for _, auth := range auths {
		if _, errRegister := manager.Register(WithSkipPersist(ctx), auth); errRegister != nil {
			t.Fatalf("Register(%s): %v", auth.ID, errRegister)
		}
		registry.GetGlobalRegistry().RegisterClient(auth.ID, auth.Provider, []*registry.ModelInfo{{ID: model}})
		authID := auth.ID
		t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient(authID) })
	}

	session := func(id string) cliproxyexecutor.Options {
		return cliproxyexecutor.Options{Metadata: map[string]any{cliproxyexecutor.DerivedSessionIDMetadataKey: id}}
	}
	pick := func(opts cliproxyexecutor.Options) *Auth {
		t.Helper()
		auth, _, _, errPick := manager.pickNextMixed(ctx, []string{primary, fallback}, model, opts, nil)
		if errPick != nil {
			t.Fatalf("pick: %v", errPick)
		}
		return auth
	}
	exhaust := func(id, provider string) {
		manager.MarkResult(ctx, Result{AuthID: id, Provider: provider, Model: model, Success: false,
			Error: &Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota"}})
	}

	for i := 0; i < 6; i++ {
		if got := pick(session("s" + strconv.Itoa(i))); got.Provider != primary {
			t.Fatalf("new session %d went to %q while Claude logins are available", i, got.ID)
		}
	}

	exhaust(claudeIDs[0], primary)
	exhaust(claudeIDs[1], primary)
	if got := pick(session("two-down")); got.ID != claudeIDs[2] {
		t.Fatalf("with one Claude login left, picked %q, want %q", got.ID, claudeIDs[2])
	}

	exhaust(claudeIDs[2], primary)
	if got := pick(session("all-down")); got.ID != codexID {
		t.Fatalf("with every Claude login exhausted, picked %q, want the fallback %q", got.ID, codexID)
	}

	expireSessionAffinityPriorityModelCooldown(t, manager, claudeIDs[1], model)
	if got := pick(session("after-recovery")); got.ID != claudeIDs[1] {
		t.Fatalf("new session after a Claude login recovered went to %q, want %q", got.ID, claudeIDs[1])
	}
}
