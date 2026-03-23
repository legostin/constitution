# How To — Constitution

Практическое руководство по сценариям использования.

---

## Для разработчика

### Установка и первый запуск

```bash
# 1. Установить бинарник
go install github.com/legostin/constitution/cmd/constitution@latest

# 2. Создать конфиг
constitution init
# Выбрать шаблон: Full или Minimal

# 3. Установить хуки в Claude Code
constitution setup
# Выбрать хуки через чеклист, указать scope (user/project)

# 4. Проверить
constitution validate
# ✓ .constitution.yaml
#   10 rules (7 enabled)

# 5. Перезапустить Claude Code
```

### Подключение к серверу компании

Если Platform-команда уже подняла сервер:

```bash
# Одна команда — создаёт конфиг + ставит хуки
constitution setup --remote https://constitution.company.com

# Или по шагам
constitution init --remote https://constitution.company.com
constitution setup
```

Если нужна авторизация — задайте токен:

```bash
export CONSTITUTION_TOKEN="your-token"
```

### Тестирование хуков локально

Не нужно запускать Claude Code — можно подать JSON напрямую:

```bash
# Тест: опасная команда
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {"command": "rm -rf /"},
  "cwd": "'$(pwd)'"
}' | constitution

# Ожидаемый вывод: {"hookSpecificOutput":{"hookEventName":"PreToolUse",
#   "permissionDecision":"deny","permissionDecisionReason":"Command blocked: Root deletion"}}
```

```bash
# Тест: секрет в файле
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Write",
  "tool_input": {"file_path": "config.go", "content": "key = AKIAIOSFODNN7ABCDEFG"},
  "cwd": "'$(pwd)'"
}' | constitution

# Ожидаемый вывод: deny, "Secret detected: AWS Access Key pattern matched"
```

```bash
# Тест: безопасная команда (должна пройти — пустой вывод)
echo '{
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {"command": "ls -la"},
  "cwd": "'$(pwd)'"
}' | constitution

# Пустой вывод = разрешено
```

### Что делать если хук блокирует

1. **Прочитать причину** — сообщение в `permissionDecisionReason` объясняет какое правило сработало
2. **Найти правило** — открыть `.constitution.yaml`, искать по имени правила
3. **Проверить вручную** — подать тот же JSON через pipe (см. выше)
4. **Логи** — если в конфиге задан `log_file`, смотреть там:
   ```yaml
   settings:
     log_level: debug
     log_file: /tmp/constitution.log
   ```

### Временно отключить constitution

**Вариант 1**: Отключить конкретное правило в конфиге:
```yaml
rules:
  - id: cmd-validate
    enabled: false    # ← было true
```

**Вариант 2**: Удалить хуки из Claude Code:
```bash
constitution uninstall
# Потом вернуть: constitution setup
```

**Вариант 3**: Переименовать конфиг:
```bash
mv .constitution.yaml .constitution.yaml.disabled
# Без конфига constitution пропускает всё (exit 0)
```

### Troubleshooting

**Конфиг не найден:**
```bash
constitution validate
# ✗ No config file found

# Проверить:
ls -la .constitution.yaml .claude/constitution.yaml
# Или указать явно:
constitution validate --config path/to/config.yaml
```

**Хуки не срабатывают:**
1. Проверить что хуки установлены:
   ```bash
   cat ~/.claude/settings.json | grep constitution
   ```
2. Проверить что бинарник доступен:
   ```bash
   which constitution
   ```
3. Перезапустить Claude Code (хуки загружаются при старте сессии)

**detect-secrets не установлен (для secret_yelp):**
```bash
# macOS
brew install detect-secrets
# или
pip3 install detect-secrets

# Проверить
detect-secrets --version
```
Без detect-secrets правила `secret_yelp` с severity `block` будут блокировать все действия (fail-closed). Установите утилиту или отключите правило. Встроенные `secret_regex` работают всегда.

---

## Для platform-инженера

### Деплой сервера

#### Шаг 1: Создать файл правил

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

#### Шаг 2: docker-compose.yaml

```yaml
services:
  constitutiond:
    image: ghcr.io/legostin/constitutiond:latest
    # или build: . если собираете сами
    ports:
      - "8081:8081"
    volumes:
      - ./company-rules.yaml:/etc/constitution/config.yaml:ro
    environment:
      - CONSTITUTION_TOKEN=${CONSTITUTION_TOKEN}
    restart: unless-stopped
```

#### Шаг 3: Запуск

```bash
# Задать токен
export CONSTITUTION_TOKEN="$(openssl rand -hex 32)"
echo "Token: $CONSTITUTION_TOKEN"

# Запустить
docker compose up -d

# Проверить
curl http://localhost:8081/api/v1/health
# {"status":"ok","version":"1.0.0"}
```

#### Шаг 4: Раздать разработчикам

Отправить команде:
```
constitution setup --remote https://constitution.company.com
export CONSTITUTION_TOKEN="..."
```

### Написание правил

#### Заблокировать секреты

```yaml
- id: secret-scan
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
      allow_patterns:
        - "AKIAIOSFODNN7EXAMPLE"          # AWS example key
        - "(?i)test|example|dummy"         # Test values
```

#### Ограничить директории

```yaml
- id: dir-guard
  enabled: true
  priority: 2
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Read, Write, Edit, Glob, Grep]
  check:
    type: dir_acl
    params:
      mode: denylist
      path_field: auto
      patterns:
        - "~/.ssh/**"
        - "~/.aws/**"
        - "/etc/**"
        - "**/.env"
        - "**/*.pem"
      allow_within_project: true        # Разрешить внутри CWD
```

#### Заблокировать опасные команды

```yaml
- id: cmd-block
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
        - { name: "Hard reset", regex: "\\bgit\\s+reset\\s+--hard" }
        - { name: "Chmod 777", regex: "chmod\\s+777" }
        - { name: "Pipe to shell", regex: "curl.*\\|\\s*(bash|sh)" }
      allow_patterns:
        - { name: "Apt exception", regex: "sudo\\s+(apt|brew)" }
```

#### CEL для сложной логики

```yaml
# Блокировать git push в main/master
- id: no-main-push
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Bash]
  check:
    type: cel
    params:
      expression: >
        tool_input.command.contains("git push") &&
        (tool_input.command.contains("main") || tool_input.command.contains("master"))

# Блокировать SQL DROP в .sql файлах
- id: no-drop
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Write]
  check:
    type: cel
    params:
      expression: >
        tool_input.file_path.endsWith(".sql") &&
        tool_input.content.contains("DROP")
```

CEL-переменные: `session_id`, `cwd`, `hook_event_name`, `tool_name`, `tool_input` (map), `prompt`, `permission_mode`.

CEL-функции: `path_match(pattern, path)`, `regex_match(pattern, str)`, `is_within(path, base)`.

#### Инжект стандартов компании

```yaml
- id: standards
  enabled: true
  priority: 10
  severity: audit
  hook_events: [SessionStart]
  check:
    type: skill_inject
    params:
      context: |
        Follow ACME Corp standards:
        - Structured logging with slog
        - Table-driven tests
      # Или загрузить из файла:
      context_file: ".claude/company-standards.md"
```

### Раскатка правил на команду

**Вариант A — Remote-сервер** (рекомендуется):
- Правила живут на сервере, разработчики подключаются через `constitution setup --remote URL`
- Обновление: изменить YAML → передеплоить контейнер
- Разработчики получают новые правила при следующем вызове хука

**Вариант B — Конфиг в репозитории**:
- Положить `.constitution.yaml` в корень репо, закоммитить
- Разработчики запускают `constitution setup` — хуки ставятся, конфиг уже на месте
- Обновление: PR с изменениями конфига

### Обновление правил

Хуки Claude Code читают конфиг при каждом вызове (не кэшируют). Поэтому:

- **Локальный конфиг**: изменили файл → следующий вызов хука уже использует новые правила. Рестарт Claude Code не нужен.
- **Remote-сервер**: обновили контейнер → клиенты получают новые правила при следующем запросе к `/api/v1/evaluate`.

### Мониторинг

Сервер пишет structured JSON logs через slog в stdout:

```json
{"level":"INFO","msg":"audit","session_id":"sess-123","event":"PreToolUse","rule_id":"cmd-block","passed":false,"message":"Command blocked: Force push","severity":"block"}
{"level":"INFO","msg":"request","method":"POST","path":"/api/v1/evaluate","status":200,"duration":"12ms"}
```

Подключите к вашей системе логов (DataDog, Splunk, ELK):
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

### Ротация токенов

```bash
# 1. Сгенерировать новый токен
NEW_TOKEN="$(openssl rand -hex 32)"

# 2. Обновить на сервере
export CONSTITUTION_TOKEN="$NEW_TOKEN"
docker compose up -d  # перезапуск с новым токеном

# 3. Раздать разработчикам новый токен
# Они обновляют CONSTITUTION_TOKEN в своём окружении
```

---

## Для автора плагинов

### Exec-плагин (bash)

Плагин — любой исполняемый файл. Получает JSON на stdin, возвращает JSON на stdout.

```bash
#!/bin/bash
# no-todos.sh — блокирует TODO/FIXME/HACK в коде

INPUT=$(cat)
CONTENT=$(echo "$INPUT" | jq -r '.input.tool_input.content // empty')

if [ -z "$CONTENT" ]; then
  echo '{"passed": true}'
  exit 0
fi

if echo "$CONTENT" | grep -qE 'TODO|FIXME|HACK'; then
  echo '{"passed": false, "message": "Code contains TODO/FIXME/HACK markers"}'
  exit 2
fi

echo '{"passed": true, "message": "OK"}'
```

```bash
chmod +x no-todos.sh
```

### Exec-плагин (Go)

```go
package main

import (
    "encoding/json"
    "os"
    "strings"
)

type Input struct {
    Input struct {
        ToolInput map[string]interface{} `json:"tool_input"`
    } `json:"input"`
    Params map[string]interface{} `json:"params"`
}

type Result struct {
    Passed  bool   `json:"passed"`
    Message string `json:"message"`
}

func main() {
    var input Input
    json.NewDecoder(os.Stdin).Decode(&input)

    content, _ := input.Input.ToolInput["content"].(string)
    if strings.Contains(content, "TODO") {
        json.NewEncoder(os.Stdout).Encode(Result{Passed: false, Message: "Contains TODO"})
        os.Exit(2)
    }
    json.NewEncoder(os.Stdout).Encode(Result{Passed: true, Message: "OK"})
}
```

### Протокол

**Stdin** (JSON):
```json
{
  "input": {
    "session_id": "sess-123",
    "cwd": "/home/user/project",
    "hook_event_name": "PreToolUse",
    "tool_name": "Write",
    "tool_input": {
      "file_path": "/project/main.go",
      "content": "package main..."
    }
  },
  "params": {
    "custom_param": "value"
  }
}
```

**Stdout** (JSON):
```json
{
  "passed": true,
  "message": "OK",
  "details": {"key": "value"},
  "additional_context": "Optional context for the agent"
}
```

**Exit codes:**
| Code | Значение |
|------|----------|
| `0`  | Проверка пройдена (`passed` из stdout) |
| `2`  | Проверка не пройдена, блокировать действие |
| Другие | Ошибка плагина, action пропускается (не блокирует) |

### Тестирование вручную

```bash
echo '{
  "input": {
    "hook_event_name": "PreToolUse",
    "tool_name": "Write",
    "tool_input": {"content": "// TODO: fix this"}
  },
  "params": {}
}' | ./no-todos.sh

# {"passed": false, "message": "Code contains TODO/FIXME/HACK markers"}
# Exit code: 2
```

### Регистрация в конфиге

```yaml
plugins:
  - name: "no-todos"
    type: exec
    path: "/usr/local/bin/no-todos.sh"
    timeout: 3000

rules:
  - id: no-todos
    name: "No TODO markers"
    enabled: true
    priority: 5
    severity: warn        # warn = разрешить, но предупредить
    hook_events: [PreToolUse]
    tool_match: [Write, Edit]
    check:
      type: plugin
      params:
        plugin_name: "no-todos"
```

HTTP-плагин работает аналогично, но вместо exec вызывает HTTP endpoint:
```yaml
plugins:
  - name: "compliance-api"
    type: http
    url: "https://compliance.internal/api/check"
    timeout: 5000
```

---

## Рецепты

### Заблокировать push в main

```yaml
- id: no-main-push
  name: "Block main push"
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Bash]
  check:
    type: cel
    params:
      expression: >
        tool_input.command.contains("git push") &&
        (tool_input.command.contains("main") || tool_input.command.contains("master"))
```

### Разрешить sudo только для apt

```yaml
- id: sudo-control
  name: "Sudo control"
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Bash]
  check:
    type: cmd_validate
    params:
      deny_patterns:
        - { name: "Sudo", regex: "\\bsudo\\b" }
      allow_patterns:
        - { name: "Apt", regex: "sudo\\s+(apt|apt-get)" }
```

### Сканировать через Yelp detect-secrets

```bash
# Установить
pip3 install detect-secrets
```

```yaml
- id: yelp-scan
  name: "Yelp Secret Scan"
  enabled: true
  priority: 1
  severity: block
  hook_events: [PreToolUse]
  tool_match: [Write, Edit]
  check:
    type: secret_yelp
    params:
      plugins:
        - name: AWSKeyDetector
        - name: GitHubTokenDetector
        - name: PrivateKeyDetector
        - name: Base64HighEntropyString
          limit: 4.5
        - name: KeywordDetector
        - name: SlackDetector
        - name: StripeDetector
      exclude_lines: ["pragma: allowlist"]
```

### Запустить golangci-lint после записи Go-файлов

```yaml
- id: lint-go
  name: "Go Linter"
  enabled: true
  priority: 10
  severity: warn      # warn = не блокировать, но сообщить агенту
  hook_events: [PostToolUse]
  tool_match: [Write, Edit]
  check:
    type: linter
    params:
      file_extensions: [".go"]
      command: "golangci-lint run --timeout=30s {file}"
      working_dir: project
      timeout: 30000
```

### Ограничить агента одним репозиторием

```yaml
- id: repo-lock
  name: "Repo Allowlist"
  enabled: true
  priority: 1
  severity: block
  hook_events: [SessionStart]
  check:
    type: repo_access
    params:
      mode: allowlist
      patterns:
        - "github.com/acme-corp/*"
        - "github.com/acme-corp-internal/*"
      detect_from: git_remote
```

Агент получит ошибку при старте сессии если репозиторий не в списке.

### Добавить safety-контекст к каждому промпту

```yaml
- id: safety-context
  name: "Safety Reminder"
  enabled: true
  priority: 5
  severity: audit
  hook_events: [UserPromptSubmit]
  check:
    type: prompt_modify
    params:
      system_context: |
        IMPORTANT: Never commit secrets.
        Always run tests after changes.
        Never run destructive commands without confirmation.
```
