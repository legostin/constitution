package check

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/legostin/constitution/pkg/types"
)

// RepoAccess checks if the current repository is in the allow/deny list.
type RepoAccess struct {
	mode       string // "allowlist" | "denylist"
	patterns   []string
	detectFrom string // "git_remote" | "directory"
}

func (r *RepoAccess) Name() string { return "repo_access" }

func (r *RepoAccess) Init(params map[string]interface{}) error {
	if m, ok := params["mode"].(string); ok {
		r.mode = m
	}
	if r.mode == "" {
		r.mode = "allowlist"
	}

	rawPatterns, ok := params["patterns"]
	if !ok {
		return fmt.Errorf("repo_access: patterns is required")
	}
	pats, err := toStringSlice(rawPatterns)
	if err != nil {
		return fmt.Errorf("repo_access: invalid patterns: %w", err)
	}
	r.patterns = pats

	if df, ok := params["detect_from"].(string); ok {
		r.detectFrom = df
	}
	if r.detectFrom == "" {
		r.detectFrom = "git_remote"
	}

	return nil
}

func (r *RepoAccess) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	repoID, err := r.detectRepo(ctx, input.CWD)
	if err != nil {
		// Not a git repo or can't detect — pass through
		return &types.CheckResult{Passed: true, Message: "not a git repository, skipping"}, nil
	}

	matched := false
	for _, pattern := range r.patterns {
		if matchRepoPattern(pattern, repoID) {
			matched = true
			break
		}
	}

	switch r.mode {
	case "allowlist":
		if !matched {
			return &types.CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Repository %q is not in the allowlist", repoID),
			}, nil
		}
		return &types.CheckResult{Passed: true}, nil
	case "denylist":
		if matched {
			return &types.CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Repository %q is in the denylist", repoID),
			}, nil
		}
		return &types.CheckResult{Passed: true}, nil
	default:
		return &types.CheckResult{Passed: true}, nil
	}
}

func (r *RepoAccess) detectRepo(ctx context.Context, cwd string) (string, error) {
	switch r.detectFrom {
	case "git_remote":
		return detectGitRemote(ctx, cwd)
	case "directory":
		return filepath.Base(cwd), nil
	default:
		return detectGitRemote(ctx, cwd)
	}
}

func detectGitRemote(ctx context.Context, cwd string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	remote := strings.TrimSpace(string(out))
	return normalizeRemoteURL(remote), nil
}

// normalizeRemoteURL converts git URLs to a canonical form:
// git@github.com:org/repo.git → github.com/org/repo
// https://github.com/org/repo.git → github.com/org/repo
func normalizeRemoteURL(url string) string {
	url = strings.TrimSuffix(url, ".git")

	// SSH format: git@github.com:org/repo
	if strings.Contains(url, "@") {
		parts := strings.SplitN(url, "@", 2)
		if len(parts) == 2 {
			hostPath := strings.Replace(parts[1], ":", "/", 1)
			return hostPath
		}
	}

	// HTTPS format: https://github.com/org/repo
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(url, prefix) {
			return strings.TrimPrefix(url, prefix)
		}
	}

	return url
}

// matchRepoPattern matches a repo identifier against a glob-like pattern.
// e.g., "github.com/acme-corp/*" matches "github.com/acme-corp/myrepo"
func matchRepoPattern(pattern, repo string) bool {
	if pattern == repo {
		return true
	}
	matched, _ := filepath.Match(pattern, repo)
	return matched
}
