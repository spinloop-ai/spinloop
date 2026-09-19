package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadHarnessConfig_DefaultBesideItemsFile(t *testing.T) {
	dir := t.TempDir()
	itemsPath := filepath.Join(dir, "work.yaml")
	if err := os.WriteFile(filepath.Join(dir, "harness.yaml"),
		[]byte("env:\n  FOO: bar\nstartup: echo hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hc, err := LoadHarnessConfig("", itemsPath)
	if err != nil {
		t.Fatalf("LoadHarnessConfig: %v", err)
	}
	if hc.Env["FOO"] != "bar" || hc.Startup != "echo hi" {
		t.Errorf("the default harness.yaml beside the items file should be read, got %+v", hc)
	}
}

func TestLoadHarnessConfig_ExplicitFlagOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	itemsPath := filepath.Join(dir, "work.yaml")
	if err := os.WriteFile(filepath.Join(dir, "harness.yaml"),
		[]byte("env:\n  FOO: default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	named := filepath.Join(dir, "other.yaml")
	if err := os.WriteFile(named, []byte("env:\n  FOO: named\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hc, err := LoadHarnessConfig(named, itemsPath)
	if err != nil {
		t.Fatalf("LoadHarnessConfig: %v", err)
	}
	if hc.Env["FOO"] != "named" {
		t.Errorf("the flag's file should be read in preference to the default, got %+v", hc)
	}
}

func TestLoadHarnessConfig_NeitherPresentChangesNothing(t *testing.T) {
	dir := t.TempDir()
	itemsPath := filepath.Join(dir, "work.yaml")
	hc, err := LoadHarnessConfig("", itemsPath)
	if err != nil {
		t.Fatalf("no harness.yaml at all should not be an error: %v", err)
	}
	if hc.hasLifecycle() || len(hc.Env) != 0 {
		t.Errorf("a zero HarnessConfig was expected, got %+v", hc)
	}
}

func TestLoadHarnessConfig_UnparsableFileFails(t *testing.T) {
	dir := t.TempDir()
	itemsPath := filepath.Join(dir, "work.yaml")
	if err := os.WriteFile(filepath.Join(dir, "harness.yaml"), []byte("not: [valid: yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadHarnessConfig("", itemsPath)
	if err == nil || !strings.Contains(err.Error(), "harness.yaml") {
		t.Errorf("an unparsable harness.yaml should fail naming it, got %v", err)
	}
}

func TestLoadHarnessConfig_ARelativeBaseDirResolvesAgainstHarnessYamlsOwnDirectory(t *testing.T) {
	dir := t.TempDir()
	itemsPath := filepath.Join(dir, "sub", "work.yaml")
	if err := os.MkdirAll(filepath.Dir(itemsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(itemsPath), "harness.yaml"),
		[]byte("baseDir: ../items\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hc, err := LoadHarnessConfig("", itemsPath)
	if err != nil {
		t.Fatalf("LoadHarnessConfig: %v", err)
	}
	want := filepath.Join(dir, "items")
	if hc.BaseDir != want {
		t.Errorf("baseDir = %q, want %q (resolved against harness.yaml's own directory)", hc.BaseDir, want)
	}
}

func TestLoadHarnessConfig_AnAbsoluteBaseDirIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	itemsPath := filepath.Join(dir, "work.yaml")
	abs := filepath.Join(dir, "elsewhere")
	if err := os.WriteFile(filepath.Join(dir, "harness.yaml"),
		[]byte("baseDir: "+abs+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hc, err := LoadHarnessConfig("", itemsPath)
	if err != nil {
		t.Fatalf("LoadHarnessConfig: %v", err)
	}
	if hc.BaseDir != abs {
		t.Errorf("an absolute baseDir should be unchanged, got %q, want %q", hc.BaseDir, abs)
	}
}

func TestResolveItemDir(t *testing.T) {
	for _, tc := range []struct {
		name, baseDir, dir, want string
	}{
		{"no baseDir", "", "./parser", "./parser"},
		{"relative dir joins baseDir", "/base", "parser", "/base/parser"},
		{"absolute dir is unaffected", "/base", "/elsewhere/parser", "/elsewhere/parser"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveItemDir(tc.baseDir, tc.dir); got != tc.want {
				t.Errorf("ResolveItemDir(%q, %q) = %q, want %q", tc.baseDir, tc.dir, got, tc.want)
			}
		})
	}
}

func TestCheckHarnessEnvCollision(t *testing.T) {
	if err := CheckHarnessEnvCollision(map[string]string{"OPENAI_API_KEY": "x"}); err == nil {
		t.Error("an entry naming the token's own variable should be refused")
	}
	if err := CheckHarnessEnvCollision(map[string]string{"SOME_OTHER_VAR": "x"}); err != nil {
		t.Errorf("an unrelated entry should not be refused: %v", err)
	}
}
