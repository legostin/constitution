package celenv

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// New creates a CEL environment with constitution-specific variables and functions.
func New() (*cel.Env, error) {
	return cel.NewEnv(
		// Variables available in CEL expressions
		cel.Variable("session_id", cel.StringType),
		cel.Variable("cwd", cel.StringType),
		cel.Variable("hook_event_name", cel.StringType),
		cel.Variable("tool_name", cel.StringType),
		cel.Variable("tool_input", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("prompt", cel.StringType),
		cel.Variable("permission_mode", cel.StringType),

		// Custom functions
		cel.Function("path_match",
			cel.Overload("path_match_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(pathMatch),
			),
		),
		cel.Function("regex_match",
			cel.Overload("regex_match_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(regexMatch),
			),
		),
		cel.Function("is_within",
			cel.Overload("is_within_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(isWithin),
			),
		),
	)
}

func pathMatch(pattern, path ref.Val) ref.Val {
	p, ok1 := pattern.Value().(string)
	s, ok2 := path.Value().(string)
	if !ok1 || !ok2 {
		return types.Bool(false)
	}
	matched, _ := filepath.Match(p, s)
	return types.Bool(matched)
}

func regexMatch(pattern, str ref.Val) ref.Val {
	p, ok1 := pattern.Value().(string)
	s, ok2 := str.Value().(string)
	if !ok1 || !ok2 {
		return types.Bool(false)
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return types.Bool(false)
	}
	return types.Bool(re.MatchString(s))
}

func isWithin(path, base ref.Val) ref.Val {
	p, ok1 := path.Value().(string)
	b, ok2 := base.Value().(string)
	if !ok1 || !ok2 {
		return types.Bool(false)
	}
	absP, _ := filepath.Abs(p)
	absB, _ := filepath.Abs(b)
	return types.Bool(strings.HasPrefix(absP, absB+string(filepath.Separator)) || absP == absB)
}
