package main

import _ "embed"

//go:embed configs/full.yaml
var fullConfigTemplate string

//go:embed configs/minimal.yaml
var minimalConfigTemplate string

//go:embed configs/remote.yaml
var remoteConfigTemplate string
