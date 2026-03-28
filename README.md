# Constitution

Go-фреймворк для управления поведением Claude Code через систему хуков. Незыблемые правила — «конституция» агента — задаются через YAML-конфиг и не могут быть обойдены агентом.

## Архитектура

```
Claude Code hooks ──► constitution (Go binary)
                          │
                          ├── Локальные проверки (< 50ms)
                          │     ├── Детекция секретов
                          │     ├── ACL директорий
                          │     ├── Валидация команд
                          │     ├── CEL-выражения
                          │     └── Контроль репозиториев
                          │
                          └── POST ──► constitutiond (remote service)
                                        ├── Stateful проверки
                                        ├── Аудит лог (slog → stdout)
                                        └── Централизованный конфиг
```

Один бинарник обслуживает все хуки Claude Code. Он читает JSON из stdin, определяет тип события по полю `hook_event_name`, применяет правила из YAML-конфига и возвращает JSON в stdout.

## Быстрый старт

### Установка

```bash
go install github.com/legostin/constitution/cmd/constitution@latest
```

### Сценарий 1: Локальные правила

```bash
constitution init                 # Создать .constitution.yaml из шаблона
constitution setup                # Интерактивно установить хуки в Claude Code
```

### Сценарий 2: Подключение к серверу компании

```bash
constitution setup --remote https://constitution.company.com
# → Создаёт .constitution.yaml с remote URL + ставит хуки
```

### Сценарий 3: Конфиг уже в репозитории

Если `.constitution.yaml` уже лежит в репе (Platform-команда добавила):

```bash
constitution setup                # Находит конфиг, ставит хуки
```

## CLI

```
constitution                      # Hook handler (stdin/stdout) — вызывается Claude Code
constitution init                 # Создать .constitution.yaml
constitution init --template minimal
constitution init --remote URL    # Создать remote-only конфиг
constitution setup                # Интерактивная установка хуков
constitution setup --remote URL   # Быстрая настройка remote + хуки
constitution setup --scope user   # Установить в ~/.claude/settings.json
constitution validate             # Проверить конфиг
constitution uninstall            # Удалить хуки из settings.json
constitution rules                # Интерактивный менеджер правил
constitution rules add            # Пошаговый визард создания правила
constitution rules list           # Показать все правила
constitution rules edit <id>      # Редактировать правило
constitution rules delete <id>    # Удалить правило
constitution rules toggle <id>    # Включить/выключить
constitution rules add --id=X --check-type=Y --events=Z --params='{...}'  # Неинтерактивный режим
constitution rules list --json    # JSON-вывод для скриптов
constitution skill install        # Установить Claude Code skills
constitution skill uninstall      # Удалить skills
constitution skill list           # Показать установленные
constitution version
```

### Claude Code Skills

Constitution поставляется с двумя Claude Code skills:

| Skill | Описание |
|-------|----------|
| `/constitution` | Управление правилами, валидация, диагностика — Claude вызывает CLI |
| `/constitution-rules` | Быстрое создание правил через диалог с пользователем |

```bash
constitution skill install --scope project   # Установить для этого проекта
constitution skill install --scope user      # Установить для всех проектов
```

Skills используют неинтерактивный режим CLI (`--json`, `--yes` флаги) чтобы Claude мог программно вызывать команды.

### Паттерны оркестрации

Готовые конфигурации для популярных паттернов управления агентом:

```bash
constitution init --workflow autonomous       # Полная автономность + guardrails
constitution init --workflow plan-first       # Plan → Execute → Test
constitution init --workflow ooda-loop        # OODA: Observe → Orient → Decide → Act
constitution init --workflow strict-security  # Максимальная безопасность
```

| Паттерн | Описание | Ключевые правила |
|---------|----------|-----------------|
| **Autonomous** | Агент принимает решения сам, safety guardrails | skill_inject (self-critique), cmd_validate, secret_regex, Stop gates |
| **Plan-First** | Сначала план, потом код, потом тесты | skill_inject (workflow), prompt_modify (reminder), Stop: build+tests+commit |
| **OODA Loop** | Военный фреймворк: наблюдение → анализ → решение → действие | skill_inject (OODA cycle), prompt_modify (cycle reminder) |
| **Strict Security** | Максимальная защита | Extended secrets, Yelp detect-secrets, strict ACL, expanded cmd blocklist, repo control |

Каждый паттерн — это полный `.constitution.yaml` с настроенными правилами. Можно комбинировать: создайте паттерн как базу, затем добавьте правила через `constitution rules add`.

## Деплой сервера (для компаний)

Platform-команда поднимает `constitutiond` с правилами компании. Разработчики подключаются через `constitution setup --remote URL`.

### Docker Compose

```yaml
# docker-compose.yaml
services:
  constitutiond:
    image: ghcr.io/legostin/constitutiond:latest
    ports:
      - "8081:8081"
    volumes:
      - ./company-rules.yaml:/etc/constitution/config.yaml:ro
    environment:
      - CONSTITUTION_TOKEN=${CONSTITUTION_TOKEN}
```

```bash
docker compose up -d
```

### Из исходников

```bash
go install github.com/legostin/constitution/cmd/constitutiond@latest
constitutiond --config rules.yaml --addr :8081
```

### Управление правилами

```
company-constitution/              ← Git-репо Platform-команды
├── company-rules.yaml             ← правила
├── docker-compose.yaml            ← деплой
└── .github/workflows/deploy.yaml  ← CI: push → redeploy
```

Platform-команда правит YAML, пушит, CI обновляет контейнер. Разработчики ничего не делают.

## Конфигурация

### Иерархия конфигов (4 уровня)

Constitution использует многоуровневую систему конфигурации по принципу конституционной иерархии: **более глобальный уровень имеет больший авторитет** и не может быть ослаблен нижестоящим.

| Уровень | Авторитет | Источник | Кто управляет |
|---------|-----------|----------|---------------|
| **Global** | Высший | Определяется платформой/моделью | Разработчики модели / платформы (вне контроля constitution) |
| **Enterprise** | Высокий | Определяется провайдером LLM | Провайдер LLM / платформа (вне контроля constitution) |
| **User** | Средний | `~/.config/constitution/constitution.yaml` | Пользователь |
| **Project** | Низший | `{cwd}/.constitution.yaml` или `{cwd}/.claude/constitution.yaml` | Разработчик проекта |

> **Примечание**: уровни Global и Enterprise зарезервированы для правил, которые устанавливаются разработчиками модели или платформой (например, Claude Code). Constitution не создаёт, не ищет и не управляет конфигами на этих уровнях — они существуют в системе типов для совместимости с будущей платформенной инжекцией правил. Пользователи работают с уровнями **User** и **Project**.

Все найденные конфиги **загружаются и мержатся**. Флаг `--config` и `$CONSTITUTION_CONFIG` имеют уровень User.

**Правила мержа:**

- Нижний уровень **может** добавлять новые правила
- Нижний уровень **может** усилить severity (warn → block)
- Нижний уровень **НЕ может** ослабить severity (block → warn)
- Нижний уровень **НЕ может** отключить правило вышестоящего уровня
- Settings: первое непустое значение от высшего уровня побеждает
- Remote: высший уровень с `enabled: true` побеждает целиком
- Plugins: объединение по имени, при коллизии побеждает высший уровень

```
~/.config/constitution/constitution.yaml   ← правила пользователя (все проекты)
~/work/project-a/.constitution.yaml        ← доп. правила проекта (не могут ослабить user)
~/work/project-b/                          ← нет своего, используется user
```

При `constitution validate` показываются все обнаруженные источники и конфликты мержа.

### Формат конфига

```yaml
version: "1"
name: "my-constitution"

settings:
  log_level: "info"          # debug | info | warn | error
  log_file: "/tmp/constitution.log"

remote:
  enabled: false
  url: "http://localhost:8081"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000              # мс
  fallback: "local-only"     # allow | deny | local-only

plugins:
  - name: "my-plugin"
    type: "exec"             # exec | http
    path: "/usr/local/bin/my-check"
    timeout: 3000

rules:
  - id: unique-rule-id
    name: "Human-readable name"
    description: "Optional description"
    enabled: true
    priority: 1              # Меньше = выполняется раньше
    severity: block          # block | warn | audit
    hook_events: [PreToolUse]
    tool_match: [Bash]       # Опционально, regex-совместимо
    remote: false            # Делегировать на удалённый сервис
    message: "Custom message"
    check:
      type: cmd_validate     # Тип проверки
      params:                # Параметры зависят от типа
        deny_patterns:
          - { name: "Root rm", regex: "rm\\s+-rf\\s+/" }
```

### Severity (серьёзность)

| Значение | Действие |
|----------|----------|
| `block` | Блокирует действие агента. Возвращает `deny` для PreToolUse или `exit 2` для SessionStart. |
| `warn` | Разрешает действие, но добавляет предупреждение в `systemMessage`. |
| `audit` | Разрешает без вмешательства, только логирует в файл. |

### Hook Events (события)

| Событие | Когда срабатывает | Типичные проверки |
|---------|-------------------|-------------------|
| `SessionStart` | Начало сессии | `repo_access`, `skill_inject` |
| `UserPromptSubmit` | Перед обработкой промпта | `prompt_modify` |
| `PreToolUse` | Перед вызовом инструмента | `cmd_validate`, `secret_regex`, `dir_acl`, `cel` |
| `PostToolUse` | После вызова инструмента | `linter` |
| `Stop` | Агент завершает работу | `cmd_check` (тесты, сборка), `cel` |

## Типы проверок

### `cmd_validate` — Валидация bash-команд

Блокирует опасные команды по regex-паттернам.

```yaml
check:
  type: cmd_validate
  params:
    deny_patterns:
      - name: "Root deletion"
        regex: "rm\\s+-rf\\s+/"
      - name: "Drop database"
        regex: "\\bdrop\\s+database\\b"
        case_insensitive: true
    allow_patterns:           # Исключения (проверяются первыми)
      - name: "Apt exception"
        regex: "sudo\\s+apt"
```

**Как работает**: извлекает поле `command` из `tool_input`, сначала проверяет `allow_patterns` (если совпадение — пропускает), затем `deny_patterns` (если совпадение — блокирует).

### `secret_regex` — Детекция секретов

Сканирует содержимое файлов на наличие секретов перед записью.

```yaml
check:
  type: secret_regex
  params:
    scan_field: content       # Поле tool_input для сканирования
    patterns:
      - name: "AWS Access Key"
        regex: "AKIA[0-9A-Z]{16}"
      - name: "GitHub Token"
        regex: "gh[ps]_[A-Za-z0-9_]{36,}"
      - name: "Private Key"
        regex: "-----BEGIN .* PRIVATE KEY-----"
    allow_patterns:           # Исключения (ложные срабатывания)
      - "AKIAIOSFODNN7EXAMPLE"
      - "(?i)test|example|dummy"
```

**Как работает**: для `Write` сканирует поле `content`, для `Edit` — поле `new_string`. Если паттерн совпал и совпадение не попадает под `allow_patterns` — блокирует.

### `dir_acl` — Контроль доступа к директориям

Ограничивает к каким файлам и директориям агент может обращаться.

```yaml
check:
  type: dir_acl
  params:
    mode: denylist            # denylist | allowlist
    path_field: auto          # auto | file_path | path | pattern
    patterns:
      - "/etc/**"
      - "~/.ssh/**"
      - "~/.aws/**"
      - "**/.env"
      - "**/*.pem"
    allow_within_project: true  # Разрешить всё внутри CWD
```

**Поддерживаемые glob-паттерны**:
- `**` — любая вложенность директорий
- `*` — любое имя файла
- `~` — домашняя директория пользователя

**`path_field: auto`** — автоматически пробует поля `file_path` → `path` → `pattern` и использует первое найденное.

### `repo_access` — Контроль репозиториев

Разрешает или запрещает запуск агента в определённых репозиториях.

```yaml
check:
  type: repo_access
  params:
    mode: allowlist           # allowlist | denylist
    patterns:
      - "github.com/my-org/*"
      - "github.com/my-org-internal/*"
    detect_from: git_remote   # git_remote | directory
```

**Как работает**: при `SessionStart` определяет текущий репозиторий через `git remote get-url origin`, нормализует URL (SSH и HTTPS → `github.com/org/repo`), сравнивает с паттернами. Если репо не в allowlist — блокирует сессию.

### `cel` — CEL-выражения

Для сложной логики, невыразимой через простые regex-паттерны. Использует [Common Expression Language](https://github.com/google/cel-go).

```yaml
check:
  type: cel
  params:
    expression: >
      tool_input.command.contains("git push") &&
      (tool_input.command.contains("main") || tool_input.command.contains("master"))
```

**Доступные переменные**:

| Переменная | Тип | Описание |
|------------|-----|----------|
| `session_id` | `string` | ID сессии |
| `cwd` | `string` | Текущая рабочая директория |
| `hook_event_name` | `string` | Тип события |
| `tool_name` | `string` | Имя инструмента |
| `tool_input` | `map(string, dyn)` | Входные данные инструмента |
| `prompt` | `string` | Текст промпта пользователя |
| `permission_mode` | `string` | Режим разрешений |
| `last_assistant_message` | `string` | Последнее сообщение агента (Stop-события) |

**Встроенные функции**:

| Функция | Сигнатура | Описание |
|---------|-----------|----------|
| `path_match` | `(pattern, path) → bool` | Glob-матчинг пути |
| `regex_match` | `(pattern, str) → bool` | Regex-матчинг строки |
| `is_within` | `(path, base) → bool` | Проверяет что путь внутри базовой директории |

**Примеры CEL-выражений**:

```yaml
# Блокировать запись в prod-директории, если не в bypass-режиме
expression: >
  is_within(tool_input.file_path, "/prod") &&
  permission_mode != "bypassPermissions"

# Блокировать curl с подозрительными доменами
expression: >
  tool_name == "Bash" &&
  tool_input.command.contains("curl") &&
  regex_match("https?://(pastebin|hastebin|0x0)", tool_input.command)

# Блокировать запись файлов больше определённого шаблона
expression: >
  tool_name == "Write" &&
  tool_input.file_path.endsWith(".sql") &&
  tool_input.content.contains("DROP")
```

### `secret_yelp` — Yelp detect-secrets

Интеграция с [Yelp detect-secrets](https://github.com/Yelp/detect-secrets) — 28+ детекторов секретов (AWS, GitHub, GitLab, Slack, Stripe, JWT, entropy-based и др.).

**Требования**: `pip install detect-secrets`

```yaml
check:
  type: secret_yelp
  params:
    # Плагины detect-secrets (если не указаны — все по умолчанию)
    plugins:
      - name: AWSKeyDetector
      - name: GitHubTokenDetector
      - name: PrivateKeyDetector
      - name: Base64HighEntropyString
        limit: 4.5
      - name: HexHighEntropyString
        limit: 3.0
      - name: KeywordDetector
      - name: SlackDetector
      - name: StripeDetector
    # Фильтры detect-secrets
    filters:
      - path: secret_yelp.filters.gibberish.should_exclude_secret
      - path: secret_yelp.filters.allowlist.is_line_allowlisted
    # Исключения
    exclude_secrets: ["(?i)example|test|dummy"]
    exclude_lines: ["pragma: allowlist"]
    # Путь к бинарнику (опционально)
    binary: "detect-secrets"
    # Режим сканирования

```

**Как работает**: извлекает контент из `tool_input`, сканирует каждую строку через `detect-secrets scan --string` (построчное сканирование надёжнее файлового, т.к. detect-secrets применяет агрессивные фильтры при сканировании файлов). Конфиг plugins/filters из YAML динамически генерируется в JSON baseline файл. Если `detect-secrets` не установлен — `Init()` вернёт ошибку; при `severity: block` действие будет заблокировано (fail-closed).

**Доступные плагины** (28+): `AWSKeyDetector`, `ArtifactoryDetector`, `AzureStorageKeyDetector`, `Base64HighEntropyString`, `BasicAuthDetector`, `CloudantDetector`, `DiscordBotTokenDetector`, `GitHubTokenDetector`, `GitLabTokenDetector`, `HexHighEntropyString`, `IbmCloudIamDetector`, `JwtTokenDetector`, `KeywordDetector`, `MailchimpDetector`, `NpmDetector`, `OpenAIDetector`, `PrivateKeyDetector`, `SendGridDetector`, `SlackDetector`, `StripeDetector`, `TelegramBotTokenDetector`, `TwilioKeyDetector` и др.

**Совместимость**: можно использовать одновременно с `secret_regex` (regex) — они работают независимо.

### `linter` — Запуск линтеров

Запускает внешний линтер после записи/редактирования файлов.

```yaml
check:
  type: linter
  params:
    file_extensions: [".go"]  # Фильтр по расширениям
    command: "golangci-lint run --timeout=30s {file}"
    working_dir: project      # project | file
    timeout: 30000            # мс
```

**Подстановки**: `{file}` заменяется на путь к файлу.

**`working_dir`**: `project` — запуск из CWD проекта, `file` — из директории файла.

### `prompt_modify` — Модификация промпта

Добавляет контекст к промптам пользователя.

```yaml
check:
  type: prompt_modify
  params:
    system_context: |
      IMPORTANT: Never commit secrets.
      Always run tests after changes.
    prepend: "Security reminder: "
    append: ""
```

Контекст добавляется через `additionalContext` в ответе хука — агент видит его как системное сообщение.

### `skill_inject` — Инжект скиллов

Загружает контекст из файла или инлайн-текста при старте сессии.

```yaml
check:
  type: skill_inject
  params:
    context: |
      You follow ACME Corp coding standards.
    context_file: ".claude/company-context.md"
```

Если указаны оба — `context_file` имеет приоритет. Если файл не найден — fallback на `context`.

### `cmd_check` — Запуск произвольных команд

Запускает shell-команду и проверяет exit code. В отличие от `linter`, не привязан к файлу — подходит для Stop-валидации (проверка сборки, тестов).

```yaml
check:
  type: cmd_check
  params:
    command: "go test ./... -count=1"   # Shell-команда
    working_dir: project                # project (CWD) | абсолютный путь
    timeout: 120000                     # мс, default 30s
```

**Подстановки**: `{cwd}` заменяется на текущую рабочую директорию проекта.

**Как работает**: выполняет `sh -c "command"`, exit 0 → проверка пройдена, иначе — не пройдена. Вывод команды возвращается в `Message` (при ошибке) и `AdditionalContext`.

**Типичное использование** — Stop-валидация:

```yaml
- id: stop-tests
  name: "Tests Must Pass"
  enabled: true
  priority: 1
  severity: block
  hook_events: [Stop]
  message: "Tests are failing. Fix test failures before stopping."
  check:
    type: cmd_check
    params:
      command: "go test ./internal/... ./pkg/... -count=1"
      working_dir: project
      timeout: 120000
```

### `plugin` — Внешние плагины (planned)

> **Примечание**: система плагинов находится в разработке. Инфраструктура (exec/http) реализована, но check type `plugin` пока не зарегистрирован в движке. Секция `plugins` в конфигурации парсится, но правила с `type: plugin` пока не поддерживаются.

## Remote-сервис (constitutiond)

Для централизованного управления правилами и аудита.

### Запуск

```bash
constitutiond \
  --config constitution.yaml \
  --addr :8081 \
  --token "your-secret-token"
```

### API

```
POST /api/v1/evaluate    — Выполнить правила для hook input
POST /api/v1/audit       — Записать аудит-лог (→ slog structured logging)
GET  /api/v1/config      — Получить текущий конфиг
GET  /api/v1/health      — Проверка здоровья
```

### Конфигурация клиента

```yaml
remote:
  enabled: true
  url: "http://localhost:8081"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000
  fallback: "local-only"   # Что делать если remote недоступен
```

**Стратегии fallback**:

| Значение | Поведение |
|----------|-----------|
| `local-only` | Выполнять только локальные правила, пропустить remote |
| `allow` | Пропустить все remote-правила, разрешить действие |
| `deny` | Заблокировать всё, если remote недоступен |

### Маркировка правил как remote

```yaml
rules:
  - id: deep-secret-scan
    remote: true             # Это правило выполняется на remote-сервисе
    # ...
```

## Примеры конфигураций

### Минимальный (защита от секретов и опасных команд)

```yaml
version: "1"
name: "minimal"
rules:
  - id: secret-write
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

  - id: cmd-validate
    name: "Command Validation"
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
```

### Enterprise (полная защита + remote аудит)

```yaml
version: "1"
name: "enterprise"
settings:
  log_level: info
  log_file: /var/log/constitution.log
remote:
  enabled: true
  url: "https://constitution.internal.company.com"
  auth_token_env: "CONSTITUTION_TOKEN"
  timeout: 5000
  fallback: deny
rules:
  - id: repo-access
    name: "Repository Allowlist"
    enabled: true
    priority: 1
    severity: block
    hook_events: [SessionStart]
    check:
      type: repo_access
      params:
        mode: allowlist
        patterns: ["github.com/company/*"]
        detect_from: git_remote

  - id: skill-inject
    name: "Company Standards"
    enabled: true
    priority: 10
    severity: audit
    hook_events: [SessionStart]
    check:
      type: skill_inject
      params:
        context_file: ".claude/company-standards.md"

  # ... добавьте secret_regex, dir_acl, cmd_validate, linter, cel правила
```

## Тестирование

### Unit-тесты

```bash
make test           # Все тесты с race detector
make test-v         # С verbose-выводом
```

### E2E-тесты

E2E-тесты проверяют **скомпилированный бинарник** против реального `.constitution.yaml`. Каждый тест подаёт JSON на stdin и проверяет exit code + JSON output.

```bash
make e2e            # Собрать бинарник + запустить E2E-тесты
```

35 тест-кейсов по всем активным правилам:

| Группа | Тестов | Что проверяет |
|--------|--------|---------------|
| `secret-read` | 7 | Блок `.env`, `.env.*`, `credentials.json`, `.pem`, `.key` + разрешение обычных файлов |
| `secret-write` | 6 | Блок AWS key, GitHub token, RSA key, JWT + разрешение example-ключей |
| `cmd-validate` | 9 | Блок `rm -rf /`, `chmod 777`, `curl\|bash`, force push, hard reset, DROP DATABASE |
| `CEL` | 3 | Блок push в main/master + разрешение feature-веток |
| `dir-acl` | 5 | Блок `/etc/`, `/var/`, `~/.ssh/`, `~/.aws/` + разрешение проектных файлов |
| `prompt-safety` | 1 | Инъекция safety-контекста в промпты |
| `stop` | 4 | Блокировка при failing build/tests, без `VERIFIED_PRODUCTION_READY` + разрешение при success |

E2E-тесты находятся в `e2e/e2e_test.go`. Для добавления нового кейса создайте `testCase` и вызовите `run(t, tc)`.

### Smoke-тест

```bash
make smoke-test     # Быстрая проверка: rm -rf / должен быть заблокирован
```

## Разработка

```bash
make build          # Собрать бинарники в bin/
make install        # Установить глобально (go install)
make test           # Unit-тесты с race detector
make e2e            # E2E-тесты (бинарник + реальный конфиг)
make lint           # go vet
make smoke-test     # Проверить блокировку rm -rf /
make run-server     # Запустить constitutiond локально
make docker-build   # Собрать Docker-образ
make docker-run     # Запустить через docker compose
```

### Структура проекта

```
cmd/
  constitution/       CLI + hook handler (init, setup, validate, ...)
    configs/          Embedded шаблоны конфигов (go:embed)
  constitutiond/      Remote-сервис
internal/
  celenv/             CEL environment (переменные + функции)
  check/              10 типов проверок
  config/             Загрузка и валидация YAML
  engine/             Оркестрация правил
  handler/            Обработчики событий (PreToolUse, Stop, ...)
  hook/               JSON I/O + диспатчер
  plugin/             Exec + HTTP плагины
  remote/             HTTP-клиент к constitutiond
  server/             HTTP-сервер + middleware (stateless)
pkg/types/            Shared-типы (HookInput, HookOutput, Rule, ...)
e2e/                  E2E-тесты (бинарник + реальный .constitution.yaml)
configs/              Примеры конфигураций (standalone)
Dockerfile            Multi-stage build
docker-compose.yaml   Деплой сервера
```

### Написание кастомного плагина

Любой исполняемый файл, читающий JSON из stdin и пишущий JSON в stdout:

```bash
#!/bin/bash
INPUT=$(cat)
CONTENT=$(echo "$INPUT" | jq -r '.input.tool_input.content // empty')

if echo "$CONTENT" | grep -qE 'TODO|FIXME|HACK'; then
  echo '{"passed":false,"message":"Code contains TODO/FIXME/HACK markers"}'
  exit 2
fi

echo '{"passed":true,"message":"OK"}'
```

Зарегистрируйте в конфиге:

```yaml
plugins:
  - name: "no-todos"
    type: exec
    path: "/path/to/no-todos.sh"
    timeout: 3000
```

## Протокол взаимодействия с Claude Code

### Вход (stdin)

Claude Code передаёт JSON в stdin хука:

```json
{
  "session_id": "sess-abc123",
  "cwd": "/home/user/project",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf /" },
  "permission_mode": "default"
}
```

### Выход (stdout)

#### Разрешить (пустой вывод или exit 0 без stdout)

Нет вывода — действие разрешено.

#### Заблокировать (PreToolUse)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Command blocked: Root deletion"
  }
}
```

#### Предупредить

```json
{
  "systemMessage": "[Command Validation] Potentially dangerous command detected"
}
```

#### Инжектировать контекст (SessionStart / UserPromptSubmit)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "Follow ACME coding standards..."
  }
}
```

#### Заблокировать остановку (Stop)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "Stop",
    "decision": "block",
    "reason": "Tests not executed after code changes"
  }
}
```
