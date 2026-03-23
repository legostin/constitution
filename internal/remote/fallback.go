package remote

import "github.com/legostin/constitution/pkg/types"

// ApplyFallback determines behavior when the remote service is unreachable.
// Returns: shouldBlock bool, message string
func ApplyFallback(strategy string, ruleIDs []string) (bool, string) {
	switch strategy {
	case "deny":
		return true, "Remote service unreachable; all remote rules blocked (fallback=deny)"
	case "allow":
		return false, ""
	case "local-only":
		return false, ""
	default:
		return false, ""
	}
}

// FilterRemoteRules splits rules into local and remote sets.
func FilterRemoteRules(rules []types.Rule) (local, remote []types.Rule) {
	for _, r := range rules {
		if r.Remote {
			remote = append(remote, r)
		} else {
			local = append(local, r)
		}
	}
	return
}

// RemoteRuleIDs extracts rule IDs from remote rules.
func RemoteRuleIDs(rules []types.Rule) []string {
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, r.ID)
	}
	return ids
}
