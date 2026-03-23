package check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/legostin/constitution/pkg/types"
)

// SkillInject loads context from a file or inline text at session start.
type SkillInject struct {
	contextText string
	contextFile string
}

func (s *SkillInject) Name() string { return "skill_inject" }

func (s *SkillInject) Init(params map[string]interface{}) error {
	if ct, ok := params["context"].(string); ok {
		s.contextText = ct
	}
	if cf, ok := params["context_file"].(string); ok {
		s.contextFile = cf
	}
	if s.contextText == "" && s.contextFile == "" {
		return fmt.Errorf("skill_inject: either context or context_file is required")
	}
	return nil
}

func (s *SkillInject) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	var content string

	if s.contextFile != "" {
		filePath := s.contextFile
		if !filepath.IsAbs(filePath) && input.CWD != "" {
			filePath = filepath.Join(input.CWD, filePath)
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			// File not found is not an error — just skip
			if s.contextText != "" {
				content = s.contextText
			} else {
				return &types.CheckResult{
					Passed:  true,
					Message: fmt.Sprintf("context file not found: %s", filePath),
				}, nil
			}
		} else {
			content = string(data)
		}
	}

	if content == "" && s.contextText != "" {
		content = s.contextText
	}

	if content == "" {
		return &types.CheckResult{Passed: true}, nil
	}

	return &types.CheckResult{
		Passed:            true,
		Message:           "skill context injected",
		AdditionalContext: content,
	}, nil
}
