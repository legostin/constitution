package config

import (
	"testing"

	"github.com/legostin/constitution/pkg/types"
)

func makePolicy(name string, rules ...types.Rule) *types.Policy {
	return &types.Policy{
		Version: "1",
		Name:    name,
		Rules:   rules,
	}
}

func makeRule(id string, severity types.Severity, enabled bool) types.Rule {
	return types.Rule{
		ID:         id,
		Name:       id,
		Enabled:    enabled,
		Severity:   severity,
		HookEvents: []string{"PreToolUse"},
		Check:      types.CheckConfig{Type: "cmd_validate", Params: map[string]interface{}{}},
	}
}

func TestMergePolicies_SingleLayer(t *testing.T) {
	rule := makeRule("r1", types.SeverityBlock, true)
	rule.Source = types.LevelUser
	p := makePolicy("single", rule)

	result := MergePolicies([]LayeredPolicy{
		{Policy: p, Level: types.LevelUser, Path: "user.yaml"},
	})

	if len(result.Policy.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(result.Policy.Rules))
	}
	if result.Policy.Rules[0].ID != "r1" {
		t.Error("expected rule r1")
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(result.Conflicts))
	}
}

func TestMergePolicies_TwoLayers_UniqueRules(t *testing.T) {
	r1 := makeRule("org-rule", types.SeverityBlock, true)
	r1.Source = types.LevelEnterprise
	r2 := makeRule("proj-rule", types.SeverityWarn, true)
	r2.Source = types.LevelProject

	result := MergePolicies([]LayeredPolicy{
		{Policy: makePolicy("org", r1), Level: types.LevelEnterprise},
		{Policy: makePolicy("proj", r2), Level: types.LevelProject},
	})

	if len(result.Policy.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(result.Policy.Rules))
	}
}

func TestMergePolicies_CannotWeakenSeverity(t *testing.T) {
	r1 := makeRule("shared", types.SeverityBlock, true)
	r1.Source = types.LevelEnterprise
	r2 := makeRule("shared", types.SeverityWarn, true)
	r2.Source = types.LevelProject

	result := MergePolicies([]LayeredPolicy{
		{Policy: makePolicy("org", r1), Level: types.LevelEnterprise},
		{Policy: makePolicy("proj", r2), Level: types.LevelProject},
	})

	if len(result.Policy.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(result.Policy.Rules))
	}
	if result.Policy.Rules[0].Severity != types.SeverityBlock {
		t.Errorf("severity should remain block, got %s", result.Policy.Rules[0].Severity)
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result.Conflicts))
	}
	if result.Conflicts[0].Action != "ignored" {
		t.Errorf("expected action=ignored, got %s", result.Conflicts[0].Action)
	}
}

func TestMergePolicies_CanStrengthenSeverity(t *testing.T) {
	r1 := makeRule("shared", types.SeverityWarn, true)
	r1.Source = types.LevelEnterprise
	r2 := makeRule("shared", types.SeverityBlock, true)
	r2.Source = types.LevelProject

	result := MergePolicies([]LayeredPolicy{
		{Policy: makePolicy("org", r1), Level: types.LevelEnterprise},
		{Policy: makePolicy("proj", r2), Level: types.LevelProject},
	})

	if result.Policy.Rules[0].Severity != types.SeverityBlock {
		t.Errorf("severity should be strengthened to block, got %s", result.Policy.Rules[0].Severity)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].Action != "strengthened" {
		t.Error("expected strengthened conflict")
	}
}

func TestMergePolicies_CannotDisable(t *testing.T) {
	r1 := makeRule("shared", types.SeverityBlock, true)
	r1.Source = types.LevelEnterprise
	r2 := makeRule("shared", types.SeverityBlock, false)
	r2.Source = types.LevelProject

	result := MergePolicies([]LayeredPolicy{
		{Policy: makePolicy("org", r1), Level: types.LevelEnterprise},
		{Policy: makePolicy("proj", r2), Level: types.LevelProject},
	})

	if !result.Policy.Rules[0].Enabled {
		t.Error("rule should remain enabled")
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].Field != "enabled" {
		t.Error("expected enabled conflict")
	}
}

func TestMergePolicies_PriorityCanOnlyTighten(t *testing.T) {
	r1 := makeRule("shared", types.SeverityBlock, true)
	r1.Source = types.LevelEnterprise
	r1.Priority = 5
	r2 := makeRule("shared", types.SeverityBlock, true)
	r2.Source = types.LevelProject
	r2.Priority = 2 // More urgent

	result := MergePolicies([]LayeredPolicy{
		{Policy: makePolicy("org", r1), Level: types.LevelEnterprise},
		{Policy: makePolicy("proj", r2), Level: types.LevelProject},
	})

	if result.Policy.Rules[0].Priority != 2 {
		t.Errorf("priority should tighten to 2, got %d", result.Policy.Rules[0].Priority)
	}
}

func TestMergePolicies_PriorityCannotLoosen(t *testing.T) {
	r1 := makeRule("shared", types.SeverityBlock, true)
	r1.Source = types.LevelEnterprise
	r1.Priority = 2
	r2 := makeRule("shared", types.SeverityBlock, true)
	r2.Source = types.LevelProject
	r2.Priority = 10 // Trying to make less urgent

	result := MergePolicies([]LayeredPolicy{
		{Policy: makePolicy("org", r1), Level: types.LevelEnterprise},
		{Policy: makePolicy("proj", r2), Level: types.LevelProject},
	})

	if result.Policy.Rules[0].Priority != 2 {
		t.Errorf("priority should stay 2, got %d", result.Policy.Rules[0].Priority)
	}
}

func TestMergePolicies_SettingsMerge(t *testing.T) {
	layers := []LayeredPolicy{
		{Policy: &types.Policy{
			Version:  "1",
			Settings: types.Settings{LogLevel: "warn"},
		}, Level: types.LevelEnterprise},
		{Policy: &types.Policy{
			Version:  "1",
			Settings: types.Settings{LogLevel: "debug", LogFile: "/tmp/proj.log"},
		}, Level: types.LevelProject},
	}

	result := MergePolicies(layers)

	if result.Policy.Settings.LogLevel != "warn" {
		t.Errorf("log_level should be 'warn' from enterprise, got %s", result.Policy.Settings.LogLevel)
	}
	if result.Policy.Settings.LogFile != "/tmp/proj.log" {
		t.Errorf("log_file should come from project, got %s", result.Policy.Settings.LogFile)
	}
}

func TestMergePolicies_RemoteMerge(t *testing.T) {
	layers := []LayeredPolicy{
		{Policy: &types.Policy{
			Version: "1",
			Remote:  types.RemoteConfig{Enabled: true, URL: "https://org.example.com"},
		}, Level: types.LevelEnterprise},
		{Policy: &types.Policy{
			Version: "1",
			Remote:  types.RemoteConfig{Enabled: true, URL: "https://project.example.com"},
		}, Level: types.LevelProject},
	}

	result := MergePolicies(layers)

	if result.Policy.Remote.URL != "https://org.example.com" {
		t.Errorf("remote URL should be from enterprise, got %s", result.Policy.Remote.URL)
	}
}

func TestMergePolicies_PluginsMerge(t *testing.T) {
	layers := []LayeredPolicy{
		{Policy: &types.Policy{
			Version: "1",
			Plugins: []types.PluginConfig{{Name: "org-plugin", Type: "exec"}},
		}, Level: types.LevelEnterprise},
		{Policy: &types.Policy{
			Version: "1",
			Plugins: []types.PluginConfig{
				{Name: "org-plugin", Type: "http"},  // collision — org wins
				{Name: "proj-plugin", Type: "exec"}, // unique — added
			},
		}, Level: types.LevelProject},
	}

	result := MergePolicies(layers)

	if len(result.Policy.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(result.Policy.Plugins))
	}
	if result.Policy.Plugins[0].Type != "exec" {
		t.Error("org-plugin should keep exec type from enterprise")
	}
}

func TestMergePolicies_FourLayers(t *testing.T) {
	rGlobal := makeRule("safety", types.SeverityBlock, true)
	rGlobal.Source = types.LevelGlobal
	rOrg := makeRule("compliance", types.SeverityBlock, true)
	rOrg.Source = types.LevelEnterprise
	rUser := makeRule("my-check", types.SeverityWarn, true)
	rUser.Source = types.LevelUser

	// Project tries to weaken global rule and adds its own
	rProjWeaken := makeRule("safety", types.SeverityAudit, false) // weaken + disable
	rProjWeaken.Source = types.LevelProject
	rProjNew := makeRule("proj-lint", types.SeverityWarn, true)
	rProjNew.Source = types.LevelProject

	result := MergePolicies([]LayeredPolicy{
		{Policy: makePolicy("global", rGlobal), Level: types.LevelGlobal},
		{Policy: makePolicy("org", rOrg), Level: types.LevelEnterprise},
		{Policy: makePolicy("user", rUser), Level: types.LevelUser},
		{Policy: makePolicy("proj", rProjWeaken, rProjNew), Level: types.LevelProject},
	})

	if len(result.Policy.Rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(result.Policy.Rules))
	}

	// safety rule should remain block + enabled
	for _, r := range result.Policy.Rules {
		if r.ID == "safety" {
			if r.Severity != types.SeverityBlock {
				t.Error("safety severity should remain block")
			}
			if !r.Enabled {
				t.Error("safety should remain enabled")
			}
		}
	}

	// Should have 2 conflicts: attempted disable + attempted weaken
	if len(result.Conflicts) != 2 {
		t.Errorf("expected 2 conflicts, got %d", len(result.Conflicts))
	}
}

func TestMergePolicies_Empty(t *testing.T) {
	result := MergePolicies(nil)
	if result.Policy == nil {
		t.Fatal("expected non-nil policy")
	}
	if len(result.Policy.Rules) != 0 {
		t.Error("expected 0 rules")
	}
}

func TestSeverityRank(t *testing.T) {
	if types.SeverityRank(types.SeverityAudit) >= types.SeverityRank(types.SeverityWarn) {
		t.Error("audit should be less strict than warn")
	}
	if types.SeverityRank(types.SeverityWarn) >= types.SeverityRank(types.SeverityBlock) {
		t.Error("warn should be less strict than block")
	}
}
