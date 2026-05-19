package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

func Load(path string) (Scenario, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Scenario{}, fmt.Errorf("scenario file is required")
	}
	// 第一版只支持 YAML，避免在场景语言未稳定前扩展多套解析路径。
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
	default:
		return Scenario{}, fmt.Errorf("unsupported scenario file %q: only YAML is supported", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, fmt.Errorf("read scenario file %s: %w", path, err)
	}
	var scenario Scenario
	if err := yaml.Unmarshal(data, &scenario); err != nil {
		return Scenario{}, fmt.Errorf("parse scenario file %s: %w", path, err)
	}
	if err := scenario.Validate(); err != nil {
		return Scenario{}, err
	}
	return scenario, nil
}
