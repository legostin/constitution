# Remote Server Guide

Centralized rule enforcement for teams. The platform team runs `constitutiond`, developers connect via `constitution setup`.

## Architecture

```
Developer machines                    Server
┌─────────────────┐                ┌──────────────────┐
│ Claude Code      │                │ constitutiond     │
│   ↓              │                │   ↓               │
│ constitution     │── POST ──────►│ /api/v1/evaluate   │
│ (local binary)   │◄── JSON ──────│   rule evaluation  │
│   ↓              │                │   ↓               │
│ Local rules +    │── POST ──────►│ /api/v1/audit      │
│ Remote rules     │                │   structured logs  │
└─────────────────┘                └──────────────────┘
```

Local rules run locally (< 50ms). Remote rules are delegated to the server. Both combine into a single policy.

## Quick Start

### 1. Create a rules file

```yaml
# company-rules.yaml
version: "1"
name: "acme-corp"

rules:
  - id: secret-scan
    name: "Secret Detection"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    tool_match: [Write, Edit]
    check:
      type: secret_regex
      params:
        scan_field: content
        patterns:
          - { name: "AWS Key", regex: "AKIA[0-9A-Z]{16}" }
          - { name: "GitHub Token", regex: "gh[ps]_[A-Za-z0-9_]{36,}" }
          - { name: "Private Key", regex: "-----BEGIN .* PRIVATE KEY-----" }

  - id: cmd-block
    name: "Dangerous Commands"
    enabled: true
    priority: 1
    severity: block
    hook_events: [PreToolUse]
    tool_match: [Bash]
    check:
      type: cmd_validate
      params:
        deny_patterns:
          - { name: "Root deletion", regex: "rm\\s+-rf\\s+/" }
          - { name: "Force push", regex: "\\bgit\\s+push\\s+.*--force" }
          - { name: "Drop database", regex: "\\bdrop\\s+database\\b", case_insensitive: true }

  - id: company-standards
    name: "Inject Standards"
    enabled: true
    priority: 10
    severity: audit
    hook_events: [SessionStart]
    check:
      type: skill_inject
      params:
        context: |
          You follow ACME Corp coding standards:
          - Use structured logging (slog)
          - Write table-driven tests
          - No TODO/FIXME in production code
```

### 2. Start the server

#### Docker Compose (recommended)

```yaml
# docker-compose.yaml
services:
  constitutiond:
    image: ghcr.io/legostin/constitutiond:latest
    # or build from source:
    # build: .
    ports:
      - "8081:8081"
    volumes:
      - ./company-rules.yaml:/etc/constitution/config.yaml:ro
    environment:
      - CONSTITUTION_TOKEN=${CONSTITUTION_TOKEN:-}
    restart: unless-stopped
```

```bash
# Generate a token
export CONSTITUTION_TOKEN="$(openssl rand -hex 32)"
echo "Token: $CONSTITUTION_TOKEN"

# Start
docker compose up -d

# Verify
curl http://localhost:8081/api/v1/health
# {"status":"ok","version":"1.0.0"}
```

#### From binary

```bash
go install github.com/legostin/constitution/cmd/constitutiond@latest

constitutiond \
  --config company-rules.yaml \
  --addr :8081 \
  --token "$CONSTITUTION_TOKEN"
```

### 3. Connect developers

Send your team:

```bash
# Install constitution
go install github.com/legostin/constitution/cmd/constitution@latest

# Run setup — Step 2 asks for remote server
constitution setup
# Enter URL: https://constitution.company.com
# Enter token env var: CONSTITUTION_TOKEN

# Set the token
export CONSTITUTION_TOKEN="<token-from-admin>"
```

Or non-interactive:

```bash
constitution setup --yes --platform claude --scope user
# Then manually add remote section to ~/.config/constitution/constitution.yaml
```

## Client Configuration

The local `.constitution.yaml` connects to the remote server:

```yaml
version: "1"
name: "my-project"

remote:
  enabled: true
  url: "https://constitution.company.com"
  auth_token_env: "CONSTITUTION_TOKEN"    # reads token from this env var
  timeout: 5000                            # ms
  fallback: "local-only"                   # what to do if server is unreachable

rules:
  # Local rules run locally
  - id: local-rule
    remote: false    # default
    # ...

  # Remote rules are delegated to the server
  - id: remote-deep-scan
    remote: true
    # ...
```

### Fallback strategies

| Value | Behavior when server is unreachable |
|-------|-------------------------------------|
| `local-only` | Skip remote rules, run local rules only |
| `allow` | Skip all remote rules, allow everything |
| `deny` | Block everything if remote is unreachable |

## API Reference

All endpoints return JSON. Auth via `Authorization: Bearer <token>` header.

### `GET /api/v1/health`

Health check. **No auth required.**

```bash
curl http://localhost:8081/api/v1/health
```

```json
{"status": "ok", "version": "1.0.0"}
```

### `POST /api/v1/evaluate`

Evaluate rules against a hook input. This is called by the local `constitution` binary for rules marked `remote: true`.

```bash
curl -X POST http://localhost:8081/api/v1/evaluate \
  -H "Authorization: Bearer $CONSTITUTION_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "session_id": "sess-123",
      "hook_event_name": "PreToolUse",
      "tool_name": "Bash",
      "tool_input": {"command": "rm -rf /"},
      "cwd": "/home/user/project"
    },
    "rule_ids": ["cmd-block", "secret-scan"]
  }'
```

```json
{
  "results": [
    {
      "rule_id": "cmd-block",
      "passed": false,
      "message": "Command blocked: Root deletion",
      "severity": "block"
    }
  ]
}
```

### `POST /api/v1/audit`

Submit audit log entries. Fire-and-forget — returns `204 No Content`.

```bash
curl -X POST http://localhost:8081/api/v1/audit \
  -H "Authorization: Bearer $CONSTITUTION_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess-123",
    "event": "PreToolUse",
    "results": [
      {"rule_id": "cmd-block", "passed": false, "message": "blocked", "severity": "block"}
    ],
    "timestamp": "2025-01-01T00:00:00Z"
  }'
```

### `GET /api/v1/config`

Get the current server policy.

```bash
curl http://localhost:8081/api/v1/config \
  -H "Authorization: Bearer $CONSTITUTION_TOKEN"
```

```json
{
  "config": {
    "version": "1",
    "name": "acme-corp",
    "rules": [...]
  }
}
```

## Server Logging

`constitutiond` writes structured JSON logs to stdout via `slog`:

```json
{"level":"INFO","msg":"audit","session_id":"sess-123","event":"PreToolUse","rule_id":"cmd-block","passed":false,"message":"Command blocked","severity":"block"}
{"level":"INFO","msg":"request","method":"POST","path":"/api/v1/evaluate","status":200,"duration":"12ms"}
```

Connect to your logging system (Datadog, Splunk, ELK):

```yaml
# docker-compose.yaml
services:
  constitutiond:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## Token Rotation

```bash
# 1. Generate new token
NEW_TOKEN="$(openssl rand -hex 32)"

# 2. Update server
export CONSTITUTION_TOKEN="$NEW_TOKEN"
docker compose up -d    # restart with new token

# 3. Distribute to team
# Developers update their CONSTITUTION_TOKEN env var
```

## Updating Rules

The server reads the config at startup. To update rules:

1. Edit `company-rules.yaml`
2. Restart the server: `docker compose restart`
3. Developers get new rules on their next hook invocation (no client-side changes needed)

For GitOps:

```
company-constitution/
├── company-rules.yaml          # rules
├── docker-compose.yaml         # deployment
└── .github/workflows/deploy.yaml  # CI: push → redeploy
```

## Production Checklist

- [ ] TLS termination (nginx/caddy/load balancer in front of constitutiond)
- [ ] Token stored in secrets manager (not in plain text)
- [ ] Health check monitoring (`/api/v1/health`)
- [ ] Log aggregation configured
- [ ] Fallback strategy decided (`local-only` recommended)
- [ ] Token rotation procedure documented
- [ ] Rules reviewed and tested before deployment
