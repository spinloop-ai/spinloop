package main

import (
	"context"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
)

// TestViperSpinloopAliasPrecedence pins the SPINLOOP_ALIAS resolution through the
// CLI's one Viper: unset falls through, a registered name resolves to its
// Spinloop, and a name the registry does not hold errors naming the variable.
func TestViperSpinloopAliasPrecedence(t *testing.T) {
	isolateConfig(t)

	t.Setenv("SPINLOOP_ALIAS", "")
	if name, path, err := spinloopFromEnv(); name != "" || path != "" || err != nil {
		t.Errorf("unset SPINLOOP_ALIAS: got (%q, %q, %v), want all empty", name, path, err)
	}

	aliasFor(t, "q3", "gemma")
	t.Setenv("SPINLOOP_ALIAS", "q3")
	name, path, err := spinloopFromEnv()
	if err != nil {
		t.Fatalf("SPINLOOP_ALIAS=q3: %v", err)
	}
	if name != "q3" || path == "" {
		t.Errorf("SPINLOOP_ALIAS=q3: got (%q, %q), want the name and its Spinloop path", name, path)
	}

	t.Setenv("SPINLOOP_ALIAS", "ghost")
	if _, _, err := spinloopFromEnv(); err == nil || !strings.Contains(err.Error(), "SPINLOOP_ALIAS") {
		t.Errorf("unregistered SPINLOOP_ALIAS: %v, want an error naming the variable", err)
	}
}

// TestViperDefaultSpinloopNamed pins that the gate and the reader count the same
// default: the variable counts as well as a ./Spinloop, and neither alone
// invents one.
func TestViperDefaultSpinloopNamed(t *testing.T) {
	isolateConfig(t)
	t.Chdir(t.TempDir())

	t.Setenv("SPINLOOP_ALIAS", "")
	if defaultSpinloopNamed() {
		t.Error("no variable and no ./Spinloop: the gate reported a default")
	}

	t.Setenv("SPINLOOP_ALIAS", "q3")
	if !defaultSpinloopNamed() {
		t.Error("the variable names a default: the gate must count it")
	}

	t.Setenv("SPINLOOP_ALIAS", "")
	mustWrite(t, "Spinloop", "PROVIDER llamacpp\nMODEL gemma\n")
	if !defaultSpinloopNamed() {
		t.Error("./Spinloop present: the gate must count it")
	}
}

// TestViperCloudVariablesFeedThePlaneResolution pins what the CLI's Viper is
// still asked for: every SPINLOOP_CLOUD_* variable the plane resolution reads
// reaches it through cliViper rather than the raw process environment, and one
// variable overrides its own endpoint while the rest stay as the plane stated
// them. There is no cloud config file for a variable to beat: the plane's own
// endpoints are what SPINLOOP_CLOUD_* overrides, one at a time.
func TestViperCloudVariablesFeedThePlaneResolution(t *testing.T) {
	isolateConfig(t)
	t.Chdir(t.TempDir()) // no ./Spinloop, so nothing but the variables speaks

	const stated = "https://plane.example"
	const override = "https://variable.example/wins"
	stubPlaneEndpoints(t, stated)

	legs := map[string]func(cloud.Config) string{
		"SPINLOOP_CLOUD_START_URL":    func(c cloud.Config) string { return c.StartURL },
		"SPINLOOP_CLOUD_STOP_URL":     func(c cloud.Config) string { return c.StopURL },
		"SPINLOOP_CLOUD_DEPLOY_URL":   func(c cloud.Config) string { return c.DeployURL },
		"SPINLOOP_CLOUD_STATS_URL":    func(c cloud.Config) string { return c.StatsURL },
		"SPINLOOP_CLOUD_ENV_URL":      func(c cloud.Config) string { return c.EnvURL },
		"SPINLOOP_CLOUD_SEED_URL":     func(c cloud.Config) string { return c.SeedURL },
		"SPINLOOP_CLOUD_UPDATE_URL":   func(c cloud.Config) string { return c.UpdateURL },
		"SPINLOOP_CLOUD_SCHEDULE_URL": func(c cloud.Config) string { return c.ScheduleURL },
	}

	for name, get := range legs {
		t.Run(name, func(t *testing.T) {
			// A value only cliViper holds, never the process environment: a
			// resolver reading os.Getenv directly would resolve the plane's
			// endpoint and fail here.
			key := strings.ToLower(strings.TrimPrefix(name, "SPINLOOP_"))
			cliViper.Set(key, override)
			// viper.Set cannot be undone — even setting "" keeps shadowing the
			// environment — so the test leaves the instance rebuilt rather than
			// emptied. Every command rebuilds cliViper through resolve() when
			// it runs; a test that pokes overrides directly owes the suite the
			// same wipe, or the next test resolves half its endpoints.
			t.Cleanup(func() { cliViper = newCLIViper() })

			cfg, err := resolveCloudConfig(context.Background(), "dev", "")
			if err != nil {
				t.Fatalf("%s set: %v", name, err)
			}
			if got := get(cfg); got != override {
				t.Errorf("%s set: resolved %q, want %q", name, got, override)
			}
			// The variables it does not name stay as the plane said them.
			if name != "SPINLOOP_CLOUD_START_URL" && cfg.StartURL != stated {
				t.Errorf("%s set: StartURL = %q, want the plane's %q", name, cfg.StartURL, stated)
			}
		})
	}
}

// The region is half of a plane's identity, so it is read the same way: through
// the CLI's Viper, and a stated region is the plane resolved.
func TestViperCloudRegionResolvesThePlane(t *testing.T) {
	isolateConfig(t)
	t.Chdir(t.TempDir())
	stubPlaneEndpoints(t, "https://plane.example")

	cliViper.Set("cloud_region", "eu-west-2")
	t.Cleanup(func() { cliViper = newCLIViper() })

	cfg, err := resolveCloudConfig(context.Background(), "dev", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != "eu-west-2" {
		t.Errorf("region = %q, want the one Viper holds", cfg.Region)
	}
}
