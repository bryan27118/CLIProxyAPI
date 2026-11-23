package auth

import "strings"

// claudeFreePlan reports a Claude login whose subscription has lapsed to the free plan. Anthropic refuses
// Claude Code's OAuth requests from a free organization (403 "OAuth authentication is currently not allowed
// for this organization"), and a login with no weekly reading ranks first under soonest-reset, so without
// this gate every new session would try it, fail, and retry elsewhere each time its cooldown ended.
//
// It is treated like a disabled login: no routing strategy, priority tier or session binding picks it, and
// it doesn't keep a lower tier (the Codex fallback) waiting. The plan is the organization_type the OAuth
// profile lookup stores on every token refresh. Refreshes keep running while the login is skipped, so it
// comes back by itself once the subscription does. Claude API keys carry no plan and are never gated.
func claudeFreePlan(auth *Auth) bool {
	if auth == nil || auth.Metadata == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "claude") {
		return false
	}
	orgType, _ := auth.Metadata["organization_type"].(string)
	switch strings.ToLower(strings.TrimSpace(orgType)) {
	case "claude_free", "free":
		return true
	}
	return false
}
