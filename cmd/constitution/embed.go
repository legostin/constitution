package main

import _ "embed"

//go:embed configs/full.yaml
var fullConfigTemplate string

//go:embed configs/minimal.yaml
var minimalConfigTemplate string

//go:embed configs/remote.yaml
var remoteConfigTemplate string

//go:embed skills/constitution/SKILL.md
var skillConstitution string

//go:embed skills/constitution-rules/SKILL.md
var skillConstitutionRules string
