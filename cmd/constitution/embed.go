package main

import _ "embed"

//go:embed configs/full.yaml
var fullConfigTemplate string

//go:embed configs/minimal.yaml
var minimalConfigTemplate string

//go:embed configs/remote.yaml
var remoteConfigTemplate string

//go:embed configs/autonomous.yaml
var autonomousTemplate string

//go:embed configs/plan-first.yaml
var planFirstTemplate string

//go:embed configs/ooda-loop.yaml
var oodaLoopTemplate string

//go:embed configs/strict-security.yaml
var strictSecurityTemplate string

//go:embed configs/ralph-loop.yaml
var ralphLoopTemplate string

//go:embed configs/autoproduct.yaml
var autoproductTemplate string

//go:embed skills/constitution/SKILL.md
var skillConstitution string

var workflowTemplates = map[string]string{
	"autonomous":      autonomousTemplate,
	"plan-first":      planFirstTemplate,
	"ooda-loop":       oodaLoopTemplate,
	"ralph-loop":      ralphLoopTemplate,
	"autoproduct":     autoproductTemplate,
	"strict-security": strictSecurityTemplate,
}

var skillFiles = map[string]string{
	"constitution": skillConstitution,
}
