package celenv

import (
	"testing"
)

func TestNew_CreatesEnvironment(t *testing.T) {
	env, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if env == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_CompileSimpleExpression(t *testing.T) {
	env, _ := New()

	tests := []struct {
		name string
		expr string
	}{
		{"string contains", `tool_name == "Bash"`},
		{"map access", `tool_input.command.contains("test")`},
		{"last_assistant_message", `last_assistant_message.contains("done")`},
		{"path_match", `path_match("*.go", "main.go")`},
		{"regex_match", `regex_match("^test", "testing")`},
		{"is_within", `is_within("/project/src", "/project")`},
		{"complex", `hook_event_name == "PreToolUse" && tool_name == "Bash"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.expr)
			if issues != nil && issues.Err() != nil {
				t.Fatalf("compile %q: %v", tt.expr, issues.Err())
			}
			if ast == nil {
				t.Fatal("compile returned nil AST")
			}
		})
	}
}

func TestNew_InvalidExpression(t *testing.T) {
	env, _ := New()
	_, issues := env.Compile("$$invalid$$")
	if issues == nil || issues.Err() == nil {
		t.Fatal("expected compile error for invalid expression")
	}
}

func TestPathMatch(t *testing.T) {
	env, _ := New()

	tests := []struct {
		expr   string
		vars   map[string]interface{}
		expect bool
	}{
		{`path_match("*.go", "main.go")`, nil, true},
		{`path_match("*.go", "main.py")`, nil, false},
		{`path_match("src/*", "src/file.go")`, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ast, _ := env.Compile(tt.expr)
			prg, _ := env.Program(ast)
			out, _, err := prg.Eval(defaultVars(tt.vars))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if out.Value().(bool) != tt.expect {
				t.Errorf("got %v, want %v", out.Value(), tt.expect)
			}
		})
	}
}

func TestRegexMatch(t *testing.T) {
	env, _ := New()

	tests := []struct {
		expr   string
		expect bool
	}{
		{`regex_match("^test", "testing")`, true},
		{`regex_match("^test", "no match")`, false},
		{`regex_match("[invalid", "text")`, false}, // invalid regex → false
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ast, _ := env.Compile(tt.expr)
			prg, _ := env.Program(ast)
			out, _, err := prg.Eval(defaultVars(nil))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if out.Value().(bool) != tt.expect {
				t.Errorf("got %v, want %v", out.Value(), tt.expect)
			}
		})
	}
}

func TestIsWithin(t *testing.T) {
	env, _ := New()

	tests := []struct {
		expr   string
		expect bool
	}{
		{`is_within("/project/src/main.go", "/project")`, true},
		{`is_within("/project", "/project")`, true},
		{`is_within("/other/path", "/project")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ast, _ := env.Compile(tt.expr)
			prg, _ := env.Program(ast)
			out, _, err := prg.Eval(defaultVars(nil))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if out.Value().(bool) != tt.expect {
				t.Errorf("got %v, want %v", out.Value(), tt.expect)
			}
		})
	}
}

func defaultVars(extra map[string]interface{}) map[string]interface{} {
	vars := map[string]interface{}{
		"session_id":              "",
		"cwd":                    "",
		"hook_event_name":        "",
		"tool_name":              "",
		"tool_input":             map[string]interface{}{},
		"prompt":                 "",
		"permission_mode":        "",
		"last_assistant_message": "",
	}
	for k, v := range extra {
		vars[k] = v
	}
	return vars
}
