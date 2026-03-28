---
name: constitution
description: "Full setup and management of AI agent constitutional rules. Use when the user asks to: set up constitution, install hooks, create/edit/delete rules, validate config, diagnose issues, or manage any aspect of agent behavior constraints. This is the complete wizard for the constitution framework."
allowed-tools: Bash(constitution *), Bash(go build *), Bash(go install *), Bash(go test *), Bash(make *), Bash(gh *), Bash(git *), Bash(which *), Bash(cat *), Bash(ls *), Bash(mkdir *), Read, Write, Edit, Glob, Grep
argument-hint: "[action: setup|init|rules|validate|diagnose|add-rule|hooks]"
---

# Constitution — полный визард управления правилами AI-агента

Ты — визард для настройки constitution, фреймворка конституционных правил для Claude Code. Ты умеешь делать ВСЁ: от установки с нуля до тонкой настройки правил.

## Определи что нужно

На основе `$ARGUMENTS` определи действие. Если аргументов нет — покажи меню:

```
Что хотите сделать?
1. Полная установка с нуля (init + hooks + skills)
2. Добавить правило
3. Показать текущие правила
4. Валидация конфигурации
5. Диагностика проблем
6. Управление хуками
```

## 1. Полная установка с нуля (`setup`)

Пошаговый процесс:

### Шаг 1: Проверь prerequisites
```bash
which constitution    # Бинарник доступен?
constitution version  # Версия
```
Если не найден — предложи: `go install github.com/legostin/constitution/cmd/constitution@latest`

### Шаг 2: Инициализация конфига
Проверь есть ли `.constitution.yaml`:
```bash
ls -la .constitution.yaml 2>/dev/null
```
Если нет — спроси пользователя какой паттерн ему нужен:

```bash
# Базовые шаблоны:
constitution init --template full       # Все типы проверок с примерами
constitution init --template minimal    # Только секреты + команды

# Паттерны оркестрации:
constitution init --workflow autonomous       # Полная автономность + guardrails
constitution init --workflow plan-first       # Plan → Execute → Test
constitution init --workflow ooda-loop        # OODA: Observe → Orient → Decide → Act
constitution init --workflow strict-security  # Максимальная безопасность
```

### Шаг 3: Установка хуков
Проверь текущие хуки:
```bash
cat .claude/settings.json 2>/dev/null
```
Если хуков нет — установи. Определи путь к бинарнику:
```bash
which constitution || echo "$HOME/go/bin/constitution"
```
Создай `.claude/settings.json` с хуками для всех событий. Используй **абсолютный путь** к бинарнику. Шаблон:
```json
{
  "hooks": {
    "SessionStart": [
      {"matcher": "", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]}
    ],
    "UserPromptSubmit": [
      {"matcher": "", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]},
      {"matcher": "Read|Write|Edit", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 5}]},
      {"matcher": "Glob|Grep", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 3}]}
    ],
    "Stop": [
      {"matcher": "", "hooks": [{"type": "command", "command": "BINARY_PATH", "timeout": 180}]}
    ]
  }
}
```

### Шаг 4: Установка скиллов
```bash
constitution skill install --scope project --quiet
```

### Шаг 5: Валидация
```bash
constitution validate
```

### Шаг 6: Предложи перезапуск Claude Code
Скажи пользователю: "Перезапустите Claude Code для активации хуков (`/exit` и запустите заново)."

## 2. Добавление правила (`add-rule`, `rules`)

Проведи пользователя через визард:

### Шаг 1: Спроси что нужно
"Что хотите контролировать?" Примеры:
- Заблокировать опасные команды
- Запретить чтение секретных файлов
- Детектировать секреты при записи
- Ограничить доступ к директориям
- Проверять сборку перед остановкой
- Инжектировать контекст в промпты

### Шаг 2: Выбери check type
На основе ответа пользователя выбери один из 10 типов:

| Сценарий | Check Type | Пример |
|----------|-----------|--------|
| Блок команд | `cmd_validate` | `deny_patterns: [{name, regex}]` |
| Блок файлов | `dir_acl` | `mode: denylist, patterns: ["**/.env"]` |
| Детект секретов | `secret_regex` | `patterns: [{name: "AWS Key", regex: "AKIA..."}]` |
| Контроль репо | `repo_access` | `mode: allowlist, patterns: ["github.com/org/*"]` |
| Кастомная логика | `cel` | `expression: "tool_input.command.contains(...)"` |
| Линтер | `linter` | `command: "golangci-lint run {file}"` |
| Yelp секреты | `secret_yelp` | `plugins: [{name: "AWSKeyDetector"}]` |
| Контекст в промпт | `prompt_modify` | `system_context: "Never commit secrets"` |
| Контекст при старте | `skill_inject` | `context_file: ".claude/standards.md"` |
| Проверка командой | `cmd_check` | `command: "go test ./..."` |

### Шаг 3: Собери параметры
Спроси у пользователя детали для выбранного типа. Сформируй JSON params.

### Шаг 4: Спроси severity и priority
- `block` (по умолчанию) / `warn` / `audit`
- Priority: 1-100 (по умолчанию 10)

### Шаг 5: Покажи превью и создай
Покажи пользователю итоговую команду, спроси подтверждение, выполни:
```bash
constitution rules add \
  --id=RULE_ID \
  --name="Rule Name" \
  --severity=block \
  --priority=1 \
  --events=EVENTS \
  --tools=TOOLS \
  --check-type=TYPE \
  --params='JSON' \
  --message="Message"
```

### Шаг 6: Валидация
```bash
constitution validate
```

## 3. Просмотр правил (`list`, `rules`)

```bash
constitution rules list --json
```
Покажи в виде отформатированной таблицы. Предложи действия: добавить/удалить/изменить.

## 4. Валидация (`validate`)

```bash
constitution validate
```
Покажи результат. Если есть ошибки — предложи исправить.

## 5. Диагностика (`diagnose`)

Проверь по порядку:
```bash
which constitution                              # 1. Бинарник доступен
constitution version                             # 2. Версия
ls .constitution.yaml                            # 3. Конфиг существует
cat .claude/settings.json                        # 4. Хуки установлены
constitution validate                            # 5. Конфиг валиден
constitution rules list --json                   # 6. Правила загружаются
```
Сообщи результат каждого шага и предложи исправления.

## 6. Управление хуками (`hooks`)

Покажи текущие хуки:
```bash
cat .claude/settings.json
```
Предложи: добавить недостающие, обновить timeout, переустановить.

## Справка по параметрам check types

### cmd_validate
```json
{"deny_patterns": [{"name": "Name", "regex": "PATTERN", "case_insensitive": false}], "allow_patterns": [...]}
```

### dir_acl
```json
{"mode": "denylist", "path_field": "auto", "patterns": ["/etc/**", "~/.ssh/**"], "allow_within_project": true}
```

### secret_regex
```json
{"scan_field": "content", "patterns": [{"name": "AWS Key", "regex": "AKIA[0-9A-Z]{16}"}], "allow_patterns": ["AKIAIOSFODNN7EXAMPLE"]}
```

### cel
```json
{"expression": "tool_input.command.contains(\"git push\") && tool_input.command.contains(\"main\")"}
```
Переменные: `session_id`, `cwd`, `hook_event_name`, `tool_name`, `tool_input` (map), `prompt`, `permission_mode`, `last_assistant_message`
Функции: `path_match(p,s)`, `regex_match(p,s)`, `is_within(path,base)`

### cmd_check
```json
{"command": "go test ./...", "working_dir": "project", "timeout": 60000}
```

### prompt_modify
```json
{"system_context": "Text", "prepend": "Text", "append": "Text"}
```

### skill_inject
```json
{"context": "Inline text", "context_file": ".claude/standards.md"}
```

### linter
```json
{"command": "golangci-lint run {file}", "file_extensions": [".go"], "working_dir": "project", "timeout": 30000}
```

### repo_access
```json
{"mode": "allowlist", "patterns": ["github.com/org/*"], "detect_from": "git_remote"}
```

### secret_yelp
```json
{"binary": "detect-secrets", "plugins": [{"name": "AWSKeyDetector"}, {"name": "GitHubTokenDetector"}]}
```

## Паттерны оркестрации

Готовые конфигурации для управления поведением агента:

| Паттерн | Команда | Что делает |
|---------|---------|-----------|
| **Autonomous** | `constitution init --workflow autonomous` | Полная автономность, self-critique, safety guardrails |
| **Plan-First** | `constitution init --workflow plan-first` | Обязательное планирование перед реализацией, Stop gates |
| **OODA Loop** | `constitution init --workflow ooda-loop` | Цикл Observe→Orient→Decide→Act, рефлексия |
| **Strict Security** | `constitution init --workflow strict-security` | Максимальная защита: секреты, ACL, расширенные блокировки |

Каждый паттерн — полный `.constitution.yaml`. Можно комбинировать: создать паттерн как базу, потом добавить правила через `constitution rules add`.

При выборе паттерна для пользователя:
- Разработчик хочет работать быстро → **autonomous**
- Команда требует процесс → **plan-first**
- Нужен аналитический подход → **ooda-loop**
- Работа с чувствительными данными → **strict-security**

## Hook Events

| Event | Когда | tool_match |
|-------|-------|-----------|
| `SessionStart` | Начало сессии | нет |
| `UserPromptSubmit` | Промпт пользователя | нет |
| `PreToolUse` | Перед инструментом | Bash, Read, Write, Edit, Glob, Grep |
| `PostToolUse` | После инструмента | Bash, Read, Write, Edit, Glob, Grep |
| `Stop` | Остановка агента | нет |
