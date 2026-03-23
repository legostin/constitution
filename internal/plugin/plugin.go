package plugin

import (
	"context"
	"fmt"

	"github.com/legostin/constitution/pkg/types"
)

// Plugin is an external check provider.
type Plugin interface {
	Name() string
	Execute(ctx context.Context, input *types.HookInput, params map[string]interface{}) (*types.CheckResult, error)
	Close() error
}

// Manager manages plugin lifecycle.
type Manager struct {
	plugins map[string]Plugin
}

// NewManager creates a Manager and loads plugins from config.
func NewManager(configs []types.PluginConfig) (*Manager, error) {
	m := &Manager{
		plugins: make(map[string]Plugin),
	}
	for _, cfg := range configs {
		p, err := loadPlugin(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to load plugin %q: %w", cfg.Name, err)
		}
		m.plugins[cfg.Name] = p
	}
	return m, nil
}

// Get returns a plugin by name.
func (m *Manager) Get(name string) (Plugin, bool) {
	p, ok := m.plugins[name]
	return p, ok
}

// Close releases all plugin resources.
func (m *Manager) Close() {
	for _, p := range m.plugins {
		p.Close()
	}
}

func loadPlugin(cfg types.PluginConfig) (Plugin, error) {
	switch cfg.Type {
	case "exec":
		return NewExecPlugin(cfg)
	case "http":
		return NewHTTPPlugin(cfg)
	default:
		return nil, fmt.Errorf("unknown plugin type: %s", cfg.Type)
	}
}
