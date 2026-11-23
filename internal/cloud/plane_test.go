package cloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
)

// stubPlane replaces the stack read for one test, counting how often it is
// consulted so a test can say "resolved once" rather than trusting the memo.
func stubPlane(t *testing.T, fn func(ctx context.Context, cfg aws.Config, stack string) (ControlPlane, error)) *int32 {
	t.Helper()
	original := planeDiscovery
	var calls int32
	planeDiscovery = func(ctx context.Context, cfg aws.Config, stack string) (ControlPlane, error) {
		atomic.AddInt32(&calls, 1)
		return fn(ctx, cfg, stack)
	}
	t.Cleanup(func() { planeDiscovery = original })
	return &calls
}

// resetPlane clears the per-process resolution between cases, the way a fresh
// process starts.
func resetPlane(t *testing.T) {
	t.Helper()
	ForgetPlanes()
	t.Cleanup(ForgetPlanes)
}

// stubPlaneChain pins the AWS-chain region so a test does not read a real
// profile.
func stubPlaneChain(t *testing.T, region string) {
	t.Helper()
	original := planeChainRegion
	planeChainRegion = func(context.Context) string { return region }
	t.Cleanup(func() { planeChainRegion = original })
}

// stubPlaneAccount pins the account the failure text names.
func stubPlaneAccount(t *testing.T, account string) {
	t.Helper()
	original := planeAccount
	planeAccount = func(context.Context, aws.Config) (string, error) { return account, nil }
	t.Cleanup(func() { planeAccount = original })
}

// foundPlane is the answer a bootstrapped stack gives.
func foundPlane(region string) ControlPlane {
	return ControlPlane{
		Config: Config{
			StartURL:  fmt.Sprintf("https://start.lambda-url.%s.on.aws/", region),
			StopURL:   fmt.Sprintf("https://stop.lambda-url.%s.on.aws/", region),
			DeployURL: fmt.Sprintf("https://deploy.lambda-url.%s.on.aws/", region),
			StatsURL:  fmt.Sprintf("https://stats.lambda-url.%s.on.aws/", region),
			Region:    region,
		},
		WeightsBucket: "weights-bucket",
	}
}

func TestResolvePlane_RegionSources(t *testing.T) {
	for _, tc := range []struct {
		name     string
		explicit string
		env      map[string]string
		chain    string
		want     string
		wantErr  string
	}{
		{name: "explicit flag", explicit: "eu-west-1", chain: "us-east-1", want: "eu-west-1"},
		{name: "spinloop region variable", env: map[string]string{"SPINLOOP_CLOUD_REGION": "eu-central-1"},
			chain: "us-east-1", want: "eu-central-1"},
		{name: "aws region variable", env: map[string]string{"AWS_REGION": "us-west-2"},
			chain: "us-east-1", want: "us-west-2"},
		{name: "profile region", chain: "ap-south-1", want: "ap-south-1"},
		{name: "nothing resolves", chain: "", wantErr: "no AWS region resolves"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetPlane(t)
			stubPlaneChain(t, tc.chain)
			calls := stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
				return foundPlane(tc.want), nil
			})
			plane, err := ResolvePlane(context.Background(), PlaneRequest{
				Region:  tc.explicit,
				GetName: envMap(tc.env),
			})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to mention %q", err, tc.wantErr)
				}
				if *calls != 0 {
					t.Errorf("a command with no region contacted the stack %d times, want 0", *calls)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if plane.Config.Region != tc.want {
				t.Errorf("region = %q, want %q", plane.Config.Region, tc.want)
			}
			if !strings.Contains(plane.Config.StartURL, tc.want) {
				t.Errorf("endpoints came from another region: %q", plane.Config.StartURL)
			}
		})
	}
}

func TestResolvePlane_ResolvedOncePerProcess(t *testing.T) {
	resetPlane(t)
	stubPlaneChain(t, "eu-west-1")
	calls := stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
		return foundPlane("eu-west-1"), nil
	})
	// Four environments read one after another — a fleet fan-out, or the
	// dashboard's refresh loop — resolve the plane once.
	for _, name := range []string{"a", "b", "c", "d"} {
		cfg, err := ResolveEnvironment(context.Background(), name, PlaneRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Environment != name {
			t.Errorf("environment = %q, want %q", cfg.Environment, name)
		}
	}
	if *calls != 1 {
		t.Errorf("the stack was read %d times for four environments, want 1", *calls)
	}
}

func TestResolvePlane_ConcurrentResolversShareOneRead(t *testing.T) {
	resetPlane(t)
	stubPlaneChain(t, "eu-west-1")
	release := make(chan struct{})
	var calls int32
	original := planeDiscovery
	planeDiscovery = func(context.Context, aws.Config, string) (ControlPlane, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return foundPlane("eu-west-1"), nil
	}
	t.Cleanup(func() { planeDiscovery = original })

	errs := make(chan error, 4)
	for _, name := range []string{"a", "b", "c", "d"} {
		go func(name string) {
			_, err := ResolveEnvironment(context.Background(), name, PlaneRequest{})
			errs <- err
		}(name)
	}
	// Every resolver is now waiting on the one read in flight. Closing release
	// lets it answer, and the second half of the assertion is that nobody
	// resolved a different answer.
	close(release)
	for range 4 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("the stack was read %d times by four concurrent resolvers, want 1", got)
	}
}

func TestResolvePlane_VariableOverrides(t *testing.T) {
	t.Run("one variable beats the discovered plane", func(t *testing.T) {
		resetPlane(t)
		stubPlaneChain(t, "eu-west-1")
		stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
			return foundPlane("eu-west-1"), nil
		})
		plane, err := ResolvePlane(context.Background(), PlaneRequest{
			GetName: envMap(map[string]string{"SPINLOOP_CLOUD_STOP_URL": "https://elsewhere.example.com/stop"}),
		})
		if err != nil {
			t.Fatal(err)
		}
		if plane.Config.StopURL != "https://elsewhere.example.com/stop" {
			t.Errorf("stop URL = %q, want the stated one", plane.Config.StopURL)
		}
		if !strings.Contains(plane.Config.StartURL, "eu-west-1") {
			t.Errorf("the other endpoints should stay as the stack said them: %q", plane.Config.StartURL)
		}
	})

	t.Run("a complete set needs no stack", func(t *testing.T) {
		resetPlane(t)
		stubPlaneChain(t, "eu-west-1")
		calls := stubPlane(t, func(context.Context, aws.Config, string) (ControlPlane, error) {
			return ControlPlane{}, errors.New("no stack must be read")
		})
		vars := map[string]string{
			"SPINLOOP_CLOUD_START_URL":  "https://start.example.com",
			"SPINLOOP_CLOUD_STOP_URL":   "https://stop.example.com",
			"SPINLOOP_CLOUD_DEPLOY_URL": "https://deploy.example.com",
			"SPINLOOP_CLOUD_STATS_URL":  "https://stats.example.com",
		}
		cfg, err := ResolveEnvironment(context.Background(), "ci", PlaneRequest{GetName: envMap(vars)})
		if err != nil {
			t.Fatal(err)
		}
		if *calls != 0 {
			t.Errorf("a plane stated entirely by variables read the stack %d times, want 0", *calls)
		}
		if cfg.Environment != "ci" {
			t.Errorf("environment = %q, want the name given", cfg.Environment)
		}
		if cfg.StatsURL != vars["SPINLOOP_CLOUD_STATS_URL"] {
			t.Errorf("stats URL = %q", cfg.StatsURL)
		}
	})

	t.Run("an incomplete set is an override, not a plane", func(t *testing.T) {
		resetPlane(t)
		stubPlaneChain(t, "eu-west-1")
		stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
			return foundPlane("eu-west-1"), nil
		})
		plane, err := ResolvePlane(context.Background(), PlaneRequest{
			GetName: envMap(map[string]string{"SPINLOOP_CLOUD_SEED_URL": "https://seed.example.com"}),
		})
		if err != nil {
			t.Fatal(err)
		}
		if plane.Config.StartURL == "" {
			t.Error("a partial set must not stand in for a plane: the discovered endpoints are missing")
		}
	})
}

func TestResolvePlane_UnresolvableNamesWhereItLooked(t *testing.T) {
	t.Run("the account is not bootstrapped", func(t *testing.T) {
		resetPlane(t)
		stubPlaneChain(t, "eu-west-1")
		stubPlaneAccount(t, "0")
		stubPlane(t, func(context.Context, aws.Config, string) (ControlPlane, error) {
			return ControlPlane{}, fmt.Errorf(
				"the control plane (stack %q) is not deployed in this account and region", ControlPlaneStackName)
		})
		_, err := ResolvePlane(context.Background(), PlaneRequest{})
		if err == nil {
			t.Fatal("want a failure")
		}
		for _, want := range []string{ControlPlaneStackName, "account 0", "eu-west-1", "spinloop cloud bootstrap"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the failure %q does not name %q", err, want)
			}
		}
	})

	t.Run("the credential is rejected", func(t *testing.T) {
		resetPlane(t)
		stubPlaneChain(t, "eu-west-1")
		stubPlaneAccount(t, "0")
		stubPlane(t, func(context.Context, aws.Config, string) (ControlPlane, error) {
			return ControlPlane{}, &smithy.GenericAPIError{
				Code: "ExpiredTokenException", Message: "the security token included in the request is expired",
			}
		})
		_, err := ResolvePlane(context.Background(), PlaneRequest{})
		if err == nil {
			t.Fatal("want a failure")
		}
		text := err.Error()
		if !strings.Contains(text, "expired or invalid") {
			t.Errorf("the failure %q does not say the credential was rejected", text)
		}
		if strings.Contains(text, "spinloop cloud bootstrap") {
			t.Errorf("a rejected credential is not an unbuilt control plane: %q", text)
		}
	})
}

func TestResolveEnvironment_RejectsNonNames(t *testing.T) {
	resetPlane(t)
	stubPlaneChain(t, "eu-west-1")
	calls := stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
		return foundPlane("eu-west-1"), nil
	})
	for _, value := range []string{"", "./cloud.json", "a/b", "name.json"} {
		if _, err := ResolveEnvironment(context.Background(), value, PlaneRequest{}); err == nil {
			t.Errorf("%q was accepted as an environment name", value)
		}
	}
	if *calls != 0 {
		t.Errorf("a rejected name contacted the plane %d times, want 0", *calls)
	}
}

func TestResolveEnvironment_WritesNothingAnywhere(t *testing.T) {
	home := isolateConfig(t)
	resetPlane(t)
	stubPlaneChain(t, "eu-west-1")
	stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
		return foundPlane("eu-west-1"), nil
	})
	for _, name := range []string{"prod", "staging"} {
		if _, err := ResolveEnvironment(context.Background(), name, PlaneRequest{}); err != nil {
			t.Fatal(err)
		}
	}
	var found []string
	err := filepath.WalkDir(home, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("resolving a plane wrote %v; a resolution is not a record", found)
	}
}

func TestResolvePlane_OptionalEndpointsTravelWithThePlane(t *testing.T) {
	t.Run("a plane deployed before an endpoint existed still resolves", func(t *testing.T) {
		resetPlane(t)
		stubPlaneChain(t, "eu-west-1")
		stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
			return foundPlane("eu-west-1"), nil
		})
		cfg, err := ResolveEnvironment(context.Background(), "default", PlaneRequest{GetName: noEnv})
		if err != nil {
			t.Fatalf("a plane with no seed, env, update or schedule endpoint must still resolve: %v", err)
		}
		if cfg.SeedURL != "" || cfg.EnvURL != "" || cfg.UpdateURL != "" || cfg.ScheduleURL != "" {
			t.Errorf("absent optional endpoints came back set: %+v", cfg)
		}
	})

	t.Run("a stated endpoint overrides the stack's", func(t *testing.T) {
		resetPlane(t)
		stubPlaneChain(t, "eu-west-1")
		stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
			plane := foundPlane("eu-west-1")
			plane.Config.SeedURL = "http://from-the-stack"
			return plane, nil
		})
		cfg, err := ResolveEnvironment(context.Background(), "default", PlaneRequest{
			GetName: envMap(map[string]string{"SPINLOOP_CLOUD_SEED_URL": "http://stated"}),
		})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SeedURL != "http://stated" {
			t.Errorf("SeedURL = %q, want the stated endpoint", cfg.SeedURL)
		}
	})
}

func TestSwapEndpoint_KeepsTheQueryTheCallerAdded(t *testing.T) {
	cfg := Config{StartURL: "https://old.example.com/start", StopURL: "https://old.example.com/stop"}
	next := Config{StartURL: "https://new.example.com/start", StopURL: "https://new.example.com/stop"}

	called := cfg.StartURL + "?retainUntil=2026-01-01T00%3A00%3A00Z"
	got, ok := swapEndpoint(cfg, next, called)
	if !ok {
		t.Fatal("the start endpoint was not swapped")
	}
	if got != "https://new.example.com/start?retainUntil=2026-01-01T00%3A00%3A00Z" {
		t.Errorf("swapped URL = %q", got)
	}
	if _, ok := swapEndpoint(cfg, next, "https://somewhere.else/entirely"); ok {
		t.Error("a URL from neither plane was swapped")
	}
	if _, ok := swapEndpoint(cfg, cfg, called); ok {
		t.Error("an unmoved plane reported a swap")
	}
}

func TestStalePlane_DistinguishesAMovedPlaneFromARefusal(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"unreachable host", errors.New("Get \"https://plane.example.com\": dial tcp: lookup plane.example.com: no such host"), true},
		{"connection refused", errors.New("Get \"https://198.51.100.9\": dial tcp: connect: connection refused"), true},
		{"expired credential", &smithy.GenericAPIError{Code: "ExpiredTokenException"}, true},
		{"lacking permission", &smithy.GenericAPIError{Code: "AccessDenied"}, false},
		{"a reply that explains itself", errors.New("stop returned HTTP 409: instance is already stopping"), false},
		{"no failure", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stalePlane(tc.err); got != tc.want {
				t.Errorf("stalePlane(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRefreshPlane_HealsAPlaneThatMoved(t *testing.T) {
	resetPlane(t)
	stubPlaneChain(t, "eu-west-1")
	stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
		return foundPlane("eu-west-1"), nil
	})
	stale, err := ResolveEnvironment(context.Background(), "prod", PlaneRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// The plane is re-bootstrapped: the Lambdas have new URLs.
	var moved int32
	stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
		atomic.AddInt32(&moved, 1)
		plane := foundPlane("eu-west-1")
		plane.Config.StartURL = "https://fresh.lambda-url.eu-west-1.on.aws/"
		return plane, nil
	})

	fresh, changed := refreshPlane(context.Background(), stale, noEnv)
	if !changed {
		t.Fatal("a re-bootstrapped plane was reported unchanged")
	}
	if atomic.LoadInt32(&moved) != 1 {
		t.Errorf("the stack was read %d times, want 1", moved)
	}
	if fresh.StartURL != "https://fresh.lambda-url.eu-west-1.on.aws/" {
		t.Errorf("start URL = %q, want the current one", fresh.StartURL)
	}
	if fresh.Environment != "prod" {
		t.Errorf("the retry must still act on prod, got %q", fresh.Environment)
	}
	if called, ok := swapEndpoint(stale, fresh, stale.StartURL); !ok || called != fresh.StartURL {
		t.Errorf("swapEndpoint = %q, %v", called, ok)
	}

	// Unchanged endpoints are not a fix: the failure stands, unretried.
	if _, changed := refreshPlane(context.Background(), fresh, noEnv); changed {
		t.Error("an unmoved plane reported a retry")
	}
}

func TestRevalidate_RetriesOnceAgainstTheMovedPlane(t *testing.T) {
	resetPlane(t)
	stubPlaneChain(t, "eu-west-1")
	stubPlaneAccount(t, "0")

	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("env") != "prod" {
			t.Errorf("the retried call lost its environment: %q", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"state":"running","base_url":"http://198.51.100.10:8000/v1"}`)
	}))
	defer live.Close()

	// A server that is up when resolved and gone by the time it is called: the
	// picture a long-lived process has of a plane that moved under it.
	gone := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	goneURL := gone.URL
	gone.Close()

	planeFor := func(url string) ControlPlane {
		return ControlPlane{Config: Config{
			StartURL: url + "/start", StopURL: url + "/stop", DeployURL: url + "/deploy", Region: "eu-west-1",
		}}
	}
	var reads int32
	stubPlane(t, func(context.Context, aws.Config, string) (ControlPlane, error) {
		if atomic.AddInt32(&reads, 1) == 1 {
			return planeFor(goneURL), nil
		}
		return planeFor(live.URL), nil
	})

	stubAWSEnv(t)
	cfg, err := ResolveEnvironment(context.Background(), "prod", PlaneRequest{})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := Status(context.Background(), cfg)
	if err != nil {
		t.Fatalf("the first call should have healed at the failure: %v", err)
	}
	if resp.State != "running" {
		t.Errorf("state = %q", resp.State)
	}
	if atomic.LoadInt32(&reads) != 2 {
		t.Errorf("the plane was resolved %d times, want 2 (once, then once more at the failure)", reads)
	}
}

func TestRevalidate_DoesNotRetryAnOrdinaryFailure(t *testing.T) {
	resetPlane(t)
	stubPlaneChain(t, "eu-west-1")
	stubPlaneAccount(t, "0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error":"instance is already stopping"}`)
	}))
	defer server.Close()

	stubPlane(t, func(_ context.Context, _ aws.Config, _ string) (ControlPlane, error) {
		return ControlPlane{Config: Config{
			StartURL: server.URL + "/start", StopURL: server.URL + "/stop",
			DeployURL: server.URL + "/deploy", Region: "eu-west-1",
		}}, nil
	})
	stubAWSEnv(t)
	cfg, err := ResolveEnvironment(context.Background(), "prod", PlaneRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Stop(context.Background(), cfg); err == nil {
		t.Fatal("a 409 should be reported")
	}
	// One resolution only: nothing about a refusal says the plane moved.
	if _, err := Stop(context.Background(), cfg); err == nil {
		t.Fatal("a 409 should be reported")
	}
}
