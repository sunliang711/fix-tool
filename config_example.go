package fixtool

import _ "embed"

//go:embed config-example.toml
var configExampleTOML string

func ConfigExampleTOML() string {
	return configExampleTOML
}
