package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

// weeklyResetSignal is the canonical header the Claude executor records on each credential
// (QuotaState.Signals) after every response: when its 7-day usage window resets.
var weeklyResetSignal = http.CanonicalHeaderKey("anthropic-ratelimit-unified-7d-reset")

// soonestResetTie is how close two weekly resets must be to count as the same: those credentials
// share the load through the fallback instead of one taking everything.
const soonestResetTie = 30 * time.Minute

// SoonestResetSelector prefers the credential whose weekly (7-day) usage window resets soonest.
// Capacity in a window that is about to reset is lost if unused, so it is spent first and the
// credentials with the most time left are kept for later. A credential that runs out is put in
// cooldown as usual and leaves the candidate list until its window resets.
//
// Credentials with no usable reset signal yet (none recorded since the proxy started, or a window
// that has already reset) rank first, so a single request learns where their window stands.
// Providers that send no such signal therefore rotate exactly as before. Credentials tied on the
// earliest reset, and the unknown ones, are chosen by Fallback (round-robin when nil).
//
// Wrapped by SessionAffinitySelector, a conversation keeps its credential; only new sessions and
// failovers consult this ordering.
type SoonestResetSelector struct {
	Fallback Selector
	// Now returns the current time; tests set it. time.Now when nil.
	Now func() time.Time

	roundRobin RoundRobinSelector // the Fallback when none is set, kept so its rotation persists
}

// Pick selects the available credential whose weekly window resets soonest.
func (s *SoonestResetSelector) Pick(ctx context.Context, provider, model string, opts cliproxyexecutor.Options, auths []*Auth) (*Auth, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	available, err := getSelectorAvailableAuths(ctx, auths, provider, model, now)
	if err != nil {
		return nil, err
	}
	available = preferCodexWebsocketAuths(ctx, provider, available)

	var unknown, known []*Auth
	resets := make(map[*Auth]time.Time, len(available))
	for _, auth := range available {
		reset, ok := weeklyReset(auth, now)
		if !ok {
			unknown = append(unknown, auth)
			continue
		}
		resets[auth] = reset
		known = append(known, auth)
	}

	candidates := unknown
	if len(candidates) == 0 {
		earliest := resets[known[0]]
		for _, auth := range known[1:] {
			if resets[auth].Before(earliest) {
				earliest = resets[auth]
			}
		}
		for _, auth := range known {
			if resets[auth].Sub(earliest) <= soonestResetTie {
				candidates = append(candidates, auth)
			}
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if s.Fallback != nil {
		return s.Fallback.Pick(ctx, provider, model, opts, candidates)
	}
	return s.roundRobin.Pick(ctx, provider, model, opts, candidates)
}

// weeklyReset returns when the credential's 7-day window resets, as last reported by Anthropic.
// ok is false when nothing was recorded or the reported reset has already passed.
func weeklyReset(auth *Auth, now time.Time) (time.Time, bool) {
	if auth == nil {
		return time.Time{}, false
	}
	raw := auth.Quota.Signals[weeklyResetSignal]
	reset, ok := parseResetTime(raw)
	if !ok || !reset.After(now) {
		return time.Time{}, false
	}
	return reset, true
}

// parseResetTime reads a reset as unix seconds (Anthropic's form), RFC 3339 or an HTTP date.
func parseResetTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if sec, err := strconv.ParseFloat(raw, 64); err == nil && sec > 0 {
		whole := int64(sec)
		return time.Unix(whole, int64((sec-float64(whole))*1e9)), true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if t, err := http.ParseTime(raw); err == nil {
		return t, true
	}
	return time.Time{}, false
}
