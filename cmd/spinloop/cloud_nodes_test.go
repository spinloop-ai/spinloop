package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spinloop-ai/spinloop/internal/cloud"
	"github.com/spinloop-ai/spinloop/internal/config"
)

// registerNode stands a control plane in that holds one cloud node, driven from
// the endpoints cfg names. It used to write a node's cloud.json into the
// per-user registry; there is no registry, so the same shape now states the plane
// outright — which is also how a machine with no permission to read a stack
// drives one — and the inventory the plane reports holds the name.
//
// A test that needs several cloud nodes, or one the plane says something
// specific about, uses stubPlane or stubPlaneNodes instead.
func registerNode(t *testing.T, name string, cfg cloud.Config) {
	t.Helper()
	// One endpoint is enough to drive a plane; the rest default to it, as they
	// do on a plane whose Lambdas all answer from the same deployment.
	base := cfg.StartURL
	if base == "" {
		base = cfg.StopURL
	}
	if base == "" {
		base = "https://plane.invalid"
	}
	for _, o := range []struct {
		key string
		url string
	}{
		{"SPINLOOP_CLOUD_START_URL", cfg.StartURL},
		{"SPINLOOP_CLOUD_STOP_URL", cfg.StopURL},
		{"SPINLOOP_CLOUD_DEPLOY_URL", cfg.DeployURL},
		{"SPINLOOP_CLOUD_STATS_URL", cfg.StatsURL},
		{"SPINLOOP_CLOUD_ENV_URL", cfg.EnvURL},
		{"SPINLOOP_CLOUD_SEED_URL", cfg.SeedURL},
		{"SPINLOOP_CLOUD_UPDATE_URL", cfg.UpdateURL},
		{"SPINLOOP_CLOUD_SCHEDULE_URL", cfg.ScheduleURL},
	} {
		t.Setenv(o.key, orDefault(o.url, base))
	}
	t.Setenv("SPINLOOP_CLOUD_REGION", orDefault(cfg.Region, "us-east-1"))
	stubAWSEnv(t)
	servePlaneInventory(t, []cloud.Node{{
		Name: name, BaseURL: cfg.BaseURL, State: "running",
	}})
	cloud.ForgetPlanes()
	t.Cleanup(cloud.ForgetPlanes)
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func stateServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"state":"running","healthy":true}`))
	}))
}

// A bare cloud node name on --node is the cloud node of that name on the plane
// this machine's credentials reach.
func TestCloud_NodeNameResolves(t *testing.T) {
	isolateConfig(t)
	server := stateServer(t)
	defer server.Close()
	registerNode(t, "prodenv", cloud.Config{StartURL: server.URL, StopURL: server.URL, Region: "eu-west-1"})

	t.Chdir(t.TempDir())
	if err := os.WriteFile("Spinloop", []byte("PROVIDER openai-compatible\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := cmdStatus([]string{"--node", "prodenv"}); err != nil {
			t.Errorf("status via --node name: %v", err)
		}
	})
	if !strings.Contains(out, "running") {
		t.Errorf("--node name should resolve on the plane, got:\n%s", out)
	}
}

// `default` is a cloud node name like any other: it works when the plane holds
// it and fails when it does not, with nothing special in either direction.
func TestCloud_DefaultIsJustAName(t *testing.T) {
	isolateConfig(t)
	server := stateServer(t)
	defer server.Close()
	registerNode(t, "default", cloud.Config{StartURL: server.URL, StopURL: server.URL, Region: "eu-west-1"})

	t.Chdir(t.TempDir()) // no ./Spinloop here
	out := captureStdout(t, func() {
		if err := cmdStatus([]string{"--node", "default"}); err != nil {
			t.Errorf("status via --node default: %v", err)
		}
	})
	if !strings.Contains(out, "running") {
		t.Errorf("a cloud node named default should be driven like any other, got:\n%s", out)
	}
}

// There is no default to fall back to: a cloud command that acts on one
// cloud node says so and lists what the plane holds, rather than picking one —
// on a shared plane, an assumed name is somebody else's instance.
func TestCloud_NoNodeIsNamedByDefault(t *testing.T) {
	isolateConfig(t)
	stubPlane(t, planeControlAnswers(`{"state":"running","healthy":true}`, http.StatusOK), "dev", "prod")
	t.Chdir(t.TempDir())

	err := cmdCloudStop(nil)
	if err == nil {
		t.Fatal("want a refusal to act without a named cloud node")
	}
	for _, want := range []string{"--node", "dev", "prod"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should name the flag and what could have been typed (%q)", err, want)
		}
	}
}

// An empty plane is still told as an empty plane, with the command that fills it.
func TestCloud_NoNodeIsNamedAndThePlaneIsEmpty(t *testing.T) {
	isolateConfig(t)
	stubPlaneEmpty(t)
	t.Chdir(t.TempDir())

	err := cmdCloudStop(nil)
	if err == nil {
		t.Fatal("want a refusal")
	}
	for _, want := range []string{"--node", "none deployed", "spinloop cloud deploy"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should say %q:\n%v", err, want, err)
		}
	}
}

// Nothing on disk configures a cloud node. A leftover cloud.json — at either of
// the paths the registry used — is unread, so a machine that has just been
// re-imaged, or a cloud node a colleague deployed, is the same case as one
// deployed here: the plane is asked, and the plane's answer is the answer.
func TestCloud_NothingOnDiskIsRead(t *testing.T) {
	isolateConfig(t)
	stubPlaneUnresolvable(t)
	server := stateServer(t)
	defer server.Close()

	home := must1(config.Dir())
	if err := os.MkdirAll(filepath.Join(home, "clouds", "default"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"start_url":"` + server.URL + `","stop_url":"` + server.URL + `","region":"eu-west-1"}`)
	if err := os.WriteFile(filepath.Join(home, "cloud.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "clouds", "default", "cloud.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	err := cmdMetrics([]string{"--node", "default"})
	if err == nil {
		t.Fatal("a file on disk must not configure a cloud node")
	}
	if strings.Contains(err.Error(), "cloud.json") {
		t.Errorf("the failure %q points at a file the tool no longer reads", err)
	}
}

// What the plane holds is what `cloud ls` says: the same list on every machine
// with access to it, including cloud nodes this machine never deployed, each
// with who put it there.
func TestCloudList(t *testing.T) {
	isolateConfig(t)

	t.Run("a plane with nothing on it says so", func(t *testing.T) {
		stubPlaneEmpty(t)
		out := captureStdout(t, func() {
			if err := cmdCloudList(nil); err != nil {
				t.Errorf("ls empty: %v", err)
			}
		})
		if !strings.Contains(out, "No nodes") {
			t.Errorf("empty ls should say so, got:\n%s", out)
		}
		if !strings.Contains(out, "us-east-1") {
			t.Errorf("ls should name the plane it looked at, got:\n%s", out)
		}
	})

	t.Run("the plane's cloud nodes, with their deployers", func(t *testing.T) {
		stubPlaneNodes(t, nil, []cloud.Node{
			{
				Name: "prod", DeployedBy: "colleague@example", DeployedAt: "2026-11-20T09:00:00Z",
				ServedName: "qwen", BaseURL: "http://198.51.100.4:8000/v1", State: "running",
			},
			{
				Name: "dev", DeployedBy: "me@example", DeployedAt: "2026-11-21T11:30:00Z",
				ModelID: "org/m", State: "stopped",
			},
			{Name: "half-read", State: "stopped", Partial: true},
		})
		out := captureStdout(t, func() {
			if err := cmdCloudList(nil); err != nil {
				t.Errorf("ls: %v", err)
			}
		})
		for _, want := range []string{
			"prod", "colleague@example", "qwen", "http://198.51.100.4:8000/v1", "running",
			"dev", "me@example", "org/m", "half-read", "2026-11-20",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("ls output missing %q:\n%s", want, out)
			}
		}
	})

	// A plane that cannot be read is a failure, not an empty list: reporting
	// nothing deployed because nothing could be asked would be the confidently
	// wrong answer that sent people re-deploying over other people's work.
	t.Run("an unreadable plane fails", func(t *testing.T) {
		stubPlaneUnresolvable(t)
		err := cmdCloudList(nil)
		if err == nil {
			t.Fatal("want an error")
		}
		if strings.Contains(err.Error(), "No nodes") {
			t.Errorf("ls printed an empty list rather than the failure:\n%v", err)
		}
	})

	t.Run("a plane too old to answer fails naming the fix", func(t *testing.T) {
		stubPlaneRefused(t, `{"error":"env is required"}`)
		err := cmdCloudList(nil)
		if err == nil {
			t.Fatal("want an error")
		}
		if !strings.Contains(err.Error(), "spinloop cloud bootstrap") {
			t.Errorf("the failure %q should name the fix", err)
		}
	})
}
