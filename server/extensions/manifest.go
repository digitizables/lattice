package extensions

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest describes an installable Lattice extension.
type Manifest struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Enabled     *bool  `yaml:"enabled"`
	BindAddr    string `yaml:"bind_addr"`
	Protocol    struct {
		Min string `yaml:"min"`
	} `yaml:"protocol"`
	Entrypoint string `yaml:"entrypoint"`
	Path       string `yaml:"-"` // directory containing the manifest
}

// IsEnabled defaults to true when the field is omitted.
func (m Manifest) IsEnabled() bool {
	if m.Enabled == nil {
		return true
	}
	return *m.Enabled
}

func loadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	if m.Name == "" {
		return Manifest{}, fmt.Errorf("%s: name is required", path)
	}
	if m.Kind == "" {
		return Manifest{}, fmt.Errorf("%s: kind is required", path)
	}
	return m, nil
}
