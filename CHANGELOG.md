# Changelog

## Unreleased

### Added
- Unified setup wizard (`constitution setup`) — 7-step guided installation
- OpenAI Codex hooks support (`--platform codex`)
- Interactive rule manager (`constitution rules add/edit/delete/toggle/list`)
- Non-interactive CLI mode (`--json`, `--yes`, `--id` flags)
- Claude Code skill (`/constitution`) for in-agent rule management
- 5 orchestration patterns: autonomous, plan-first, ooda-loop, ralph-loop, strict-security
- `cmd_check` check type for running shell commands on Stop events
- `last_assistant_message` CEL variable for Stop event content analysis
- E2E test framework (35 test cases)
- CI/CD pipeline (GitHub Actions)
- Goreleaser for multi-platform binary releases

### Changed
- Merged `init` and `skill` commands into `setup` (6 commands total)
- Stop hook output uses top-level `decision`/`reason` fields (per Claude Code schema)
- Documentation is now platform-agnostic (supports Claude Code + Codex)
- All CLI text translated to English
- Russian documentation moved to `docs/ru/`

### Fixed
- `no-main-push` rule: precise regex instead of broad `contains()` match
- `stop-pr-exists` rule: skip check on main/master branch
- Stop hook timeout: 5s → 180s for build/test checks
