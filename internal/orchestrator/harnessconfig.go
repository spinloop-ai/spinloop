package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/spinloop-ai/spinloop/internal/catalog"
)

// HarnessConfig is what an operator's harness.yaml carries: environment
// variables added to every launch under either dispatch backend, and shell
// scripts bracketing the harness — see the agent-dispatch spec's
// "Configuring the harness with harness.yaml" and "The startup and
// shutdown scripts" requirements.
type HarnessConfig struct {
	Env map[string]string `yaml:"env"`
	// Dispatch names the backend the run uses — "bare" or "docker" — the
	// way --dispatch does. An explicit --dispatch wins over it; where the
	// flag is not given, this is the run's choice.
	Dispatch string `yaml:"dispatch"`
	Startup  string `yaml:"startup"`
	Shutdown string `yaml:"shutdown"`
}

// hasLifecycle reports whether hc names a startup or shutdown script — the
// only condition under which a launch runs the lifecycle wrapper instead
// of the harness directly.
func (hc HarnessConfig) hasLifecycle() bool {
	return hc.Startup != "" || hc.Shutdown != ""
}

// LoadHarnessConfig reads flagPath where it is given, else harness.yaml
// beside itemsPath; where neither is present, it returns a zero
// HarnessConfig — every launch proceeds exactly as it would with no
// harness.yaml at all. A present file that does not parse is an error.
func LoadHarnessConfig(flagPath, itemsPath string) (HarnessConfig, error) {
	path := flagPath
	if path == "" {
		path = filepath.Join(filepath.Dir(itemsPath), "harness.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return HarnessConfig{}, nil
		}
		return HarnessConfig{}, fmt.Errorf("reading %s: %v", path, err)
	}
	var cfg HarnessConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return HarnessConfig{}, fmt.Errorf("parsing %s: %v", path, err)
	}
	return cfg, nil
}

// CheckHarnessEnvCollision refuses a harness.yaml env entry that names the
// same variable the work list's provider token is presented under. Every
// launch under every backend resolves the same catalogue provider (see
// plumbingProvider), so this is checked once, before the command works an
// item, rather than per launch.
func CheckHarnessEnvCollision(env map[string]string) error {
	cat, err := catalog.LoadFrom(catalog.ResolveCatalogPath(""))
	if err != nil {
		return err
	}
	p := cat.Providers[plumbingProvider]
	if p == nil {
		return fmt.Errorf("the catalogue names no %q provider for the dispatch", plumbingProvider)
	}
	if p.APIKeyEnv == "" {
		return nil
	}
	if _, collide := env[p.APIKeyEnv]; collide {
		return fmt.Errorf("harness.yaml's env names %q, the variable the resolved token is presented under", p.APIKeyEnv)
	}
	return nil
}
