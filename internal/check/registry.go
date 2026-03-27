package check

import (
	"fmt"

	"github.com/legostin/constitution/pkg/types"
)

// Factory creates a new instance of a Check.
type Factory func() types.Check

// Registry maps check type names to their factories.
type Registry struct {
	factories map[string]Factory
}

// NewRegistry creates a Registry with all built-in checks registered.
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[string]Factory),
	}
	r.registerBuiltins()
	return r
}

// Register adds a check factory to the registry.
func (r *Registry) Register(name string, factory Factory) {
	r.factories[name] = factory
}

// Get creates a new instance of the named check.
func (r *Registry) Get(name string) (types.Check, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown check type: %s", name)
	}
	return factory(), nil
}

func (r *Registry) registerBuiltins() {
	r.Register("secret_regex", func() types.Check { return &SecretDetect{} })
	r.Register("dir_acl", func() types.Check { return &DirACL{} })
	r.Register("cmd_validate", func() types.Check { return &CmdValidate{} })
	r.Register("repo_access", func() types.Check { return &RepoAccess{} })
	r.Register("cel", func() types.Check { return &CELCheck{} })
	r.Register("linter", func() types.Check { return &Linter{} })
	r.Register("secret_yelp", func() types.Check { return &DetectSecrets{} })
	r.Register("prompt_modify", func() types.Check { return &PromptModify{} })
	r.Register("skill_inject", func() types.Check { return &SkillInject{} })
	r.Register("cmd_check", func() types.Check { return &CmdCheck{} })
}
