package types

// ConfigLevel represents the authority tier a config came from.
// Lower numeric value = higher authority.
type ConfigLevel int

const (
	LevelGlobal     ConfigLevel = 0 // Reserved for model/platform developers (not managed by constitution)
	LevelEnterprise ConfigLevel = 1 // Reserved for LLM provider/platform (not managed by constitution)
	LevelUser       ConfigLevel = 2 // Personal user config (managed by constitution)
	LevelProject    ConfigLevel = 3 // Project-local config (managed by constitution)
)

// String returns a human-readable name for the config level.
func (l ConfigLevel) String() string {
	switch l {
	case LevelGlobal:
		return "global"
	case LevelEnterprise:
		return "enterprise"
	case LevelUser:
		return "user"
	case LevelProject:
		return "project"
	default:
		return "unknown"
	}
}

// Severity controls what happens when a rule matches.
type Severity string

const (
	SeverityBlock Severity = "block" // Block the action
	SeverityWarn  Severity = "warn"  // Allow but add system message warning
	SeverityAudit Severity = "audit" // Allow silently, log for audit
)

// SeverityRank returns a numeric rank for severity comparison.
// Higher rank = stricter. Used by merge logic to detect weakening attempts.
func SeverityRank(s Severity) int {
	switch s {
	case SeverityAudit:
		return 0
	case SeverityWarn:
		return 1
	case SeverityBlock:
		return 2
	default:
		return -1
	}
}

// Rule is a single check configuration loaded from YAML.
type Rule struct {
	ID          string      `yaml:"id" json:"id"`
	Name        string      `yaml:"name" json:"name"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Enabled     bool        `yaml:"enabled" json:"enabled"`
	Priority    int         `yaml:"priority" json:"priority"`
	Severity    Severity    `yaml:"severity" json:"severity"`
	HookEvents  []string    `yaml:"hook_events" json:"hook_events"`
	ToolMatch   []string    `yaml:"tool_match,omitempty" json:"tool_match,omitempty"`
	Check       CheckConfig `yaml:"check" json:"check"`
	Remote      bool        `yaml:"remote,omitempty" json:"remote,omitempty"`
	Message     string      `yaml:"message,omitempty" json:"message,omitempty"`
	Source      ConfigLevel `yaml:"-" json:"source,omitempty"`      // Set during merge, not from YAML
	SourceFile  string      `yaml:"-" json:"source_file,omitempty"` // Set during merge, not from YAML
}

// CheckConfig defines which check to run and its parameters.
type CheckConfig struct {
	Type   string                 `yaml:"type" json:"type"`
	Params map[string]interface{} `yaml:"params" json:"params"`
}

// PluginConfig defines an external plugin.
type PluginConfig struct {
	Name    string                 `yaml:"name" json:"name"`
	Type    string                 `yaml:"type" json:"type"`       // "exec" | "http"
	Path    string                 `yaml:"path,omitempty" json:"path,omitempty"`
	URL     string                 `yaml:"url,omitempty" json:"url,omitempty"`
	Timeout int                    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Config  map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

// RemoteConfig configures connection to the constitutiond service.
type RemoteConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	URL          string `yaml:"url" json:"url"`
	AuthToken    string `yaml:"auth_token,omitempty" json:"auth_token,omitempty"`
	AuthTokenEnv string `yaml:"auth_token_env,omitempty" json:"auth_token_env,omitempty"`
	Timeout      int    `yaml:"timeout" json:"timeout"`
	Fallback     string `yaml:"fallback" json:"fallback"` // "allow" | "deny" | "local-only"
}

// Settings holds global behavioral settings.
type Settings struct {
	LogLevel string `yaml:"log_level" json:"log_level"`
	LogFile  string `yaml:"log_file,omitempty" json:"log_file,omitempty"`
}

// Policy is the top-level configuration.
type Policy struct {
	Version  string         `yaml:"version" json:"version"`
	Name     string         `yaml:"name" json:"name"`
	Settings Settings       `yaml:"settings,omitempty" json:"settings,omitempty"`
	Remote   RemoteConfig   `yaml:"remote,omitempty" json:"remote,omitempty"`
	Plugins  []PluginConfig `yaml:"plugins,omitempty" json:"plugins,omitempty"`
	Rules    []Rule         `yaml:"rules" json:"rules"`
}
