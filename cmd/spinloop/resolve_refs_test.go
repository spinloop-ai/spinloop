// resolveSelection is the choke point that resolves environment references
// in a Spinloop's scalar instruction values ($FOO, ${FOO}, ${FOO:-bar}) when
// a command consumes them, leaving PARAM and ENV values to their own owners.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// Every scalar value resolves; a keyword whose value carries a malformed
// reference names itself; PARAM and ENV values stay raw in the returned copy.
func TestResolveSelection_ScalarValues(t *testing.T) {
	t.Setenv("RESOLVE_TEST_TAG", "Q4_K_M")
	sel := spinloop.Selection{
		Provider: "llamacpp",
		Model:    "org/model:$RESOLVE_TEST_TAG",
		BaseURL:  "http://${RESOLVE_TEST_HOST:-localhost}:8080/v1",
		Context:  "${RESOLVE_TEST_CTX:-32k}",
		Params:   []spinloop.Param{{Key: "hf", Value: "org/model:$RESOLVE_TEST_TAG"}},
		Env:      []spinloop.EnvVar{{Key: "SLOT", Value: "${RESOLVE_TEST_TAG}"}},
	}
	out, err := resolveSelection(sel)
	if err != nil {
		t.Fatalf("resolveSelection: %v", err)
	}
	if out.Model != "org/model:Q4_K_M" {
		t.Errorf("Model = %q, want the resolved org/model:Q4_K_M", out.Model)
	}
	if out.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("BaseURL = %q, want the defaulted localhost URL", out.BaseURL)
	}
	if out.Context != "32k" {
		t.Errorf("Context = %q, want the defaulted 32k", out.Context)
	}
	// PARAMs and ENVs belong to their own choke points: the copy carries them
	// raw, exactly as they were read.
	if out.Params[0].Value != "org/model:$RESOLVE_TEST_TAG" {
		t.Errorf("PARAM value = %q, want the raw reference", out.Params[0].Value)
	}
	if out.Env[0].Value != "${RESOLVE_TEST_TAG}" {
		t.Errorf("ENV value = %q, want the raw reference", out.Env[0].Value)
	}
	// The caller's selection is untouched: resolution returns a copy.
	if sel.Model != "org/model:$RESOLVE_TEST_TAG" {
		t.Errorf("the caller's Model = %q, want the raw reference", sel.Model)
	}
}

// A malformed reference in a scalar value names the offending keyword.
func TestResolveSelection_MalformedNamesKeyword(t *testing.T) {
	_, err := resolveSelection(spinloop.Selection{Provider: "llamacpp", Model: "org/model:${BAD"})
	if err == nil || !strings.Contains(err.Error(), "MODEL") {
		t.Errorf("a malformed MODEL reference should fail naming MODEL, got %v", err)
	}
	_, err = resolveSelection(spinloop.Selection{Provider: "llamacpp", Context: "${1FOO}"})
	if err == nil || !strings.Contains(err.Error(), "CONTEXT") {
		t.Errorf("a malformed CONTEXT reference should fail naming CONTEXT, got %v", err)
	}
}

// An ENV value carrying references resolves at the moment it is set, against
// the environment the .env pass left; its key is a name and reaches os.Setenv
// exactly as written; a malformed value fails naming the key.
func TestApplySpinloopEnv_ResolvesValues(t *testing.T) {
	dir := t.TempDir()
	dotenv := "ENVREF_TEST_DOT=from-dotenv\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatal(err)
	}
	unsetEnvOnCleanup(t, "ENVREF_TEST_DOT")
	unsetEnvOnCleanup(t, "ENVREF_TEST_SLOT")
	unsetEnvOnCleanup(t, "${FOO}")

	sel := spinloop.Selection{Env: []spinloop.EnvVar{
		{Key: "ENVREF_TEST_SLOT", Value: "${ENVREF_TEST_DOT:-fallback}-2"},
		{Key: "${FOO}", Value: "literal"},
	}}
	if err := applySpinloopEnv(sel, filepath.Join(dir, "Spinloop")); err != nil {
		t.Fatalf("applySpinloopEnv: %v", err)
	}
	if got := os.Getenv("ENVREF_TEST_SLOT"); got != "from-dotenv-2" {
		t.Errorf("ENVREF_TEST_SLOT = %q, want the .env-fed from-dotenv-2", got)
	}
	if got := os.Getenv("${FOO}"); got != "literal" {
		t.Errorf("the literal-named variable = %q, want literal", got)
	}

	bad := spinloop.Selection{Env: []spinloop.EnvVar{{Key: "ENVREF_TEST_SLOT", Value: "${BAD"}}}
	err := applySpinloopEnv(bad, filepath.Join(dir, "Spinloop"))
	if err == nil || !strings.Contains(err.Error(), "ENV ENVREF_TEST_SLOT") {
		t.Errorf("a malformed ENV reference should fail naming the key, got %v", err)
	}
}
