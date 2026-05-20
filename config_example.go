package fixtool

import _ "embed"

//go:embed config/config-example.toml
var configExampleTOML string

//go:embed config/default.toml
var defaultConfigTOML string

func ConfigExampleTOML() string {
	return configExampleTOML
}

func DefaultConfigTOML() string {
	return defaultConfigTOML
}
