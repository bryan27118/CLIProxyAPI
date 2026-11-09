package helps

import "strings"

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
