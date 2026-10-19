package auth

import (
	"context"
	"strconv"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

var soonestResetBase = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// weeklyAuth is a credential whose last Claude response reported a 7-day reset at `in` from the base time.
func weeklyAuth(id string, in time.Duration) *Auth {
	return &Auth{ID: id, Quota: QuotaState{Signals: map[string]string{
		weeklyResetSignal: strconv.FormatInt(soonestResetBase.Add(in).Unix(), 10),
	}}}
}

func soonestResetPicks(t *testing.T, selector Selector, provider string, opts cliproxyexecutor.Options, auths []*Auth, n int) []string {
	t.Helper()
	got := make([]string, 0, n)
	for i := 0; i < n; i++ {
		auth, err := selector.Pick(context.Background(), provider, "claude-sonnet", opts, auths)
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i, err)
		}
		got = append(got, auth.ID)
	}
	return got
}

func assertPicks(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("picks = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("picks = %v, want %v", got, want)
		}
	}
}

func newSoonestReset() *SoonestResetSelector {
	return &SoonestResetSelector{Now: func() time.Time { return soonestResetBase }}
}

func TestSoonestResetSelectorPick_PrefersEarliestWeeklyReset(t *testing.T) {
	t.Parallel()
	auths := []*Auth{weeklyAuth("a", 5*24*time.Hour), weeklyAuth("b", 24*time.Hour), weeklyAuth("c", 3*24*time.Hour)}
	assertPicks(t, soonestResetPicks(t, newSoonestReset(), "claude", cliproxyexecutor.Options{}, auths, 3), "b", "b", "b")
}

func TestSoonestResetSelectorPick_UnknownCredentialsFirst(t *testing.T) {
	t.Parallel()
	// b has reported nothing since the proxy started: one request learns where its window stands
	auths := []*Auth{weeklyAuth("a", 24*time.Hour), {ID: "b"}}
	assertPicks(t, soonestResetPicks(t, newSoonestReset(), "claude", cliproxyexecutor.Options{}, auths, 2), "b", "b")
}

func TestSoonestResetSelectorPick_PassedResetCountsAsUnknown(t *testing.T) {
	t.Parallel()
	// a's window already reset, so its new reset is unknown until it serves a request
	auths := []*Auth{weeklyAuth("a", -time.Hour), weeklyAuth("b", 24*time.Hour)}
	assertPicks(t, soonestResetPicks(t, newSoonestReset(), "claude", cliproxyexecutor.Options{}, auths, 1), "a")
}

func TestSoonestResetSelectorPick_TiedResetsShareTheLoad(t *testing.T) {
	t.Parallel()
	auths := []*Auth{weeklyAuth("a", 24*time.Hour), weeklyAuth("b", 24*time.Hour+10*time.Minute), weeklyAuth("c", 3*24*time.Hour)}
	assertPicks(t, soonestResetPicks(t, newSoonestReset(), "claude", cliproxyexecutor.Options{}, auths, 4), "a", "b", "a", "b")
}

func TestSoonestResetSelectorPick_NoSignalsRotatesRoundRobin(t *testing.T) {
	t.Parallel()
	// providers that report no weekly window behave exactly as round-robin
	auths := []*Auth{{ID: "b"}, {ID: "a"}, {ID: "c"}}
	assertPicks(t, soonestResetPicks(t, newSoonestReset(), "gemini", cliproxyexecutor.Options{}, auths, 4), "a", "b", "c", "a")
}

func TestSoonestResetSelectorPick_SkipsCredentialInCooldown(t *testing.T) {
	t.Parallel()
	exhausted := weeklyAuth("a", 2*time.Hour)
	exhausted.Quota.Exceeded = true
	exhausted.Quota.Reason = "credential_quota"
	exhausted.Quota.NextRecoverAt = soonestResetBase.Add(2 * time.Hour)
	auths := []*Auth{exhausted, weeklyAuth("b", 24*time.Hour), weeklyAuth("c", 3*24*time.Hour)}
	assertPicks(t, soonestResetPicks(t, newSoonestReset(), "claude", cliproxyexecutor.Options{}, auths, 2), "b", "b")
}

func TestSoonestResetSelector_SessionAffinityKeepsConversation(t *testing.T) {
	t.Parallel()
	selector := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{Fallback: newSoonestReset(), TTL: time.Hour})
	defer selector.Stop()

	session := func(id string) cliproxyexecutor.Options {
		return cliproxyexecutor.Options{OriginalRequest: []byte(`{"metadata":{"user_id":"user_x_account__session_` + id + `"}}`)}
	}
	auths := []*Auth{weeklyAuth("a", 3*24*time.Hour), weeklyAuth("b", 24*time.Hour)}
	assertPicks(t, soonestResetPicks(t, selector, "claude", session("one"), auths, 1), "b")

	// a now resets sooner: a new conversation goes there, the running one stays on b
	auths = []*Auth{weeklyAuth("a", 2*time.Hour), weeklyAuth("b", 24*time.Hour)}
	assertPicks(t, soonestResetPicks(t, selector, "claude", session("one"), auths, 2), "b", "b")
	assertPicks(t, soonestResetPicks(t, selector, "claude", session("two"), auths, 1), "a")
}

func TestParseResetTime(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"1792411200", "1792411200.5", "2026-10-20T12:00:00Z", "Tue, 20 Oct 2026 12:00:00 GMT"} {
		if _, ok := parseResetTime(raw); !ok {
			t.Fatalf("parseResetTime(%q) not ok", raw)
		}
	}
	for _, raw := range []string{"", "soon", "-5"} {
		if _, ok := parseResetTime(raw); ok {
			t.Fatalf("parseResetTime(%q) ok, want not ok", raw)
		}
	}
}
