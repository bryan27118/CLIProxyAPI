package helps

import "testing"

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
