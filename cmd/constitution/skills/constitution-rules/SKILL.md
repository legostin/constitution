---
name: constitution-rules
description: Create or modify constitution rules for AI agent governance. Use when the user wants to add a security rule, block commands, restrict file access, detect secrets, or configure any agent behavior constraint.
allowed-tools: Bash(constitution *), Read, Write, Edit
argument-hint: "[что нужно заблокировать или контролировать]"
---

# Constitution Rules — создание и изменение правил

Ты помогаешь пользователю создавать правила для constitution — системы контроля поведения AI-агента.

## Процесс

1. **Пойми что нужно** — спроси пользователя что он хочет заблокировать/контролировать
2. **Прочитай текущий конфиг** — `cat .constitution.yaml` чтобы знать существующие правила
3. **Выбери правильный check type** (см. ниже)
4. **Создай правило** через CLI в неинтерактивном режиме
5. **Проверь** — `constitution validate`

## Создание правила

```bash
constitution rules add \
  --id=RULE_ID \
  --name="Rule Name" \
  --severity=block \
  --priority=1 \
  --events=PreToolUse \
  --tools=Bash \
  --check-type=TYPE \
  --params='JSON_PARAMS' \
  --message="Сообщение при срабатывании"
```

## Шаблоны по сценариям

### Заблокировать опасную bash-команду
```bash
constitution rules add \
  --id=block-COMMAND \
  --name="Block COMMAND" \
  --severity=block --priority=1 \
  --events=PreToolUse --tools=Bash \
  --check-type=cmd_validate \
  --params='{"deny_patterns":[{"name":"DESCRIPTION","regex":"REGEX_PATTERN"}]}'
```

### Заблокировать доступ к файлам/директориям
```bash
constitution rules add \
  --id=block-PATH \
  --name="Block PATH access" \
  --severity=block --priority=2 \
  --events=PreToolUse --tools=Read,Write,Edit,Glob,Grep \
  --check-type=dir_acl \
  --params='{"mode":"denylist","path_field":"auto","patterns":["GLOB_PATTERN"],"allow_within_project":true}'
```

### Обнаружить секреты при записи файлов
```bash
constitution rules add \
  --id=detect-SECRET_TYPE \
  --name="Detect SECRET_TYPE" \
  --severity=block --priority=1 \
  --events=PreToolUse --tools=Write,Edit \
  --check-type=secret_regex \
  --params='{"scan_field":"content","patterns":[{"name":"SECRET_NAME","regex":"REGEX"}]}'
```

### CEL-выражение (сложная логика)
```bash
constitution rules add \
  --id=cel-RULE \
  --name="CEL Rule" \
  --severity=block --priority=1 \
  --events=PreToolUse --tools=Bash \
  --check-type=cel \
  --params='{"expression":"CEL_EXPRESSION"}'
```

CEL-переменные: `session_id`, `cwd`, `hook_event_name`, `tool_name`, `tool_input` (map), `prompt`, `permission_mode`, `last_assistant_message`
CEL-функции: `path_match(pattern, path)`, `regex_match(pattern, str)`, `is_within(path, base)`

### Проверка при остановке агента
```bash
constitution rules add \
  --id=stop-CHECK \
  --name="Stop: CHECK" \
  --severity=block --priority=1 \
  --events=Stop \
  --check-type=cmd_check \
  --params='{"command":"SHELL_COMMAND","working_dir":"project","timeout":60000}' \
  --message="Сообщение при блокировке остановки"
```

### Инжект контекста в промпты
```bash
constitution rules add \
  --id=inject-CONTEXT \
  --name="Inject CONTEXT" \
  --severity=audit --priority=5 \
  --events=UserPromptSubmit \
  --check-type=prompt_modify \
  --params='{"system_context":"YOUR CONTEXT TEXT"}'
```

## После создания

```bash
constitution validate          # Проверить конфиг
constitution rules list --json # Посмотреть все правила
```

## Описание запроса пользователя

Пользователь хочет: $ARGUMENTS

Проанализируй запрос, выбери подходящий check type и создай правило.
