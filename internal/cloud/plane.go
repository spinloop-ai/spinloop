package cloud

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// A plane is the control plane one credential in one region reaches: the
// well-known stack there, and the endpoints its outputs name. It is resolved,
// never configured — nothing on disk names a plane, so two people holding
// credentials for the same account and region resolve the same plane and drive
// the same cloud nodes, and what a command acts on is a name on that plane.
//
// The resolution is remembered for the process and nowhere else. It is derived
// state, and the difference matters exactly here: a stale authoritative file
// misroutes silently forever, whereas a stale in-memory answer produces a
// rejected call, which is a signal that heals itself (see refreshPlane).

// PlaneRequest says which plane to resolve. Both fields are optional: an empty
// Region defers to the environment, and a nil GetName means os.Getenv.
type PlaneRequest struct {
	// Region is an explicit override — the `--region` flag — in preference to
	// SPINLOOP_CLOUD_REGION and the AWS region chain.
	Region string
	// GetName reads configuration variables, so a caller whose values come
	// from somewhere richer than the process environment (Viper, with its
	// AutomaticEnv and config file) resolves the plane the way it reads
	// everything else. Nil means os.Getenv.
	GetName func(string) string
}

func (r PlaneRequest) getenv() func(string) string {
	if r.GetName == nil {
		return os.Getenv
	}
	return r.GetName
}

// planeDiscovery reads the stack outputs. A variable so the suite can resolve a
// plane without a network; DiscoverControlPlane is the real thing.
var planeDiscovery = DiscoverControlPlane

// planeChainRegion reads the region the standard AWS chain yields when no
// region is set: the region variables, then the profile's. A variable so tests
// need no AWS configuration files.
var planeChainRegion = func(ctx context.Context) string {
	cfg, err := LoadAWSConfig(ctx, "")
	if err != nil {
		return ""
	}
	return cfg.Region
}

// planeAccount names the account a failure concerns, for the failure text. A
// variable so tests need no STS call.
var planeAccount = CallerIdentity

// planeMemo holds one entry per region for the process's lifetime. A plane
// changes once per re-bootstrap, so re-reading CloudFormation per call would be
// a dashboard polling the stack to learn something that never moved; and
// because the answer is only ever in memory, a wrong one costs a rejected call
// rather than a permanent misroute.
//
// A failed resolution is never stored: a stack that lands, or a credential the
// operator refreshes, has to resolve on the next call.
var planeMemo struct {
	mu    sync.Mutex
	byTag map[string]*planeEntry
}

// planeEntry is one region's resolution in one of two states: while the stack
// read is in flight it carries an open done channel and read false; once done
// is closed, read says whether there is an answer to read. A failed read is
// dropped from the memo as well as closed, so nobody inherits it.
type planeEntry struct {
	done  chan struct{}
	plane ControlPlane
	read  bool
}

func init() {
	planeMemo.byTag = map[string]*planeEntry{}
}

// ResolvePlane resolves the plane a request names: the region it resolves to,
// the well-known stack in it, and that stack's outputs. The SPINLOOP_CLOUD_*
// variables override individual endpoints as they are read, and a complete set
// of them is a plane in its own right, resolved without contacting AWS.
//
// Where no plane resolves, the failure names what was looked for and where —
// the stack, the account, the region — and the fix, rather than reporting that
// some plane is missing. A plane that cannot be reached is never silently
// reported as an empty one.
func ResolvePlane(ctx context.Context, req PlaneRequest) (ControlPlane, error) {
	getenv := req.getenv()
	region, regionErr := ResolveRegion(ctx, req.Region, getenv)

	// A complete set of variables is a plane outright, read without contacting
	// AWS at all — which is why a caller that stated its endpoints outright is
	// not first required to state the region they are in: a Lambda Function URL
	// names its own region, and that is enough to sign for it.
	if stated, complete := planeFromVariables(getenv); complete {
		if region == "" {
			region = regionFromURL(stated.StartURL)
		}
		if region == "" {
			return ControlPlane{}, regionErr
		}
		stated.Region = region
		return ControlPlane{Config: stated}, nil
	}
	if regionErr != nil {
		return ControlPlane{}, regionErr
	}

	planeMemo.mu.Lock()
	entry, started := planeMemo.byTag[region]
	if !started {
		entry = &planeEntry{done: make(chan struct{})}
		planeMemo.byTag[region] = entry
	}
	planeMemo.mu.Unlock()

	if started {
		// Someone else is reading — or has read — this region's stack: wait for
		// their answer rather than issuing a second read for the same thing.
		select {
		case <-entry.done:
		case <-ctx.Done():
			return ControlPlane{}, ctx.Err()
		}
		if entry.read {
			return overlayVariables(entry.plane, getenv), nil
		}
		return ControlPlane{}, planeMissRetry(region)
	}

	discovered, err := discoverPlane(ctx, region)
	if err != nil {
		forgetPlane(region)
		close(entry.done)
		return ControlPlane{}, err
	}
	entry.plane = discovered
	entry.read = true
	close(entry.done)
	return overlayVariables(discovered, getenv), nil
}

// discoverPlane reads the well-known stack's outputs for a region. Its failures
// are phrased by planeMissError, which is where the account is named.
func discoverPlane(ctx context.Context, region string) (ControlPlane, error) {
	awsCfg, err := LoadAWSConfig(ctx, region)
	if err != nil {
		return ControlPlane{}, fmt.Errorf("resolving AWS credentials for %s: %w", region, err)
	}
	layer, err := planeDiscovery(ctx, awsCfg, ControlPlaneStackName)
	if err != nil {
		return ControlPlane{}, planeMissError(ctx, awsCfg, region, err)
	}
	// The stack names its own region, and it is the region the search was
	// made in; the two can only disagree if a stack output is wrong.
	if layer.Config.Region == "" {
		layer.Config.Region = region
	}
	return layer, nil
}

// planeMissError explains a plane that was not found. The stack not existing is
// the account not being bootstrapped, and the fix is to name it; a rejected
// credential is not an empty account, so it keeps the credential's own guidance
// rather than the bootstrap hint. Every version says where it looked.
func planeMissError(ctx context.Context, awsCfg aws.Config, region string, err error) error {
	account := ""
	if id, idErr := planeAccount(ctx, awsCfg); idErr == nil {
		account = id
	}
	switch {
	case credentialError(err):
		return fmt.Errorf(
			"AWS credentials are expired or invalid — %s: %w", credsRefreshHint(region), err)
	case strings.Contains(err.Error(), "not deployed"):
		return fmt.Errorf(
			"no cloud control plane in %s: stack %q is not deployed there — run `spinloop cloud bootstrap`, "+
				"choose another region with --region or SPINLOOP_CLOUD_REGION, or use credentials for the account that holds it",
			whereLooked(account, region), ControlPlaneStackName)
	default:
		return fmt.Errorf("reading stack %q in %s: %w", ControlPlaneStackName, whereLooked(account, region), err)
	}
}

// whereLooked phrases the place a plane was searched for, naming the account
// when the credential could say it. A failure that says only "eu-west-1" leaves
// the operator guessing which of their accounts that was.
func whereLooked(account, region string) string {
	if account == "" {
		return region
	}
	return fmt.Sprintf("account %s in %s", account, region)
}

// planeMissRetry is what a waiter sees when the reader it waited on failed: the
// same situation, one attempt later, so it points at a retry rather than
// inventing its own diagnosis.
func planeMissRetry(region string) error {
	return fmt.Errorf(
		"could not resolve the cloud control plane in %s (stack %q): another call tried first and failed — try again",
		region, ControlPlaneStackName)
}

// forgetPlane drops one region's resolution, so the next call reads the stack
// again.
func forgetPlane(region string) {
	planeMemo.mu.Lock()
	defer planeMemo.mu.Unlock()
	delete(planeMemo.byTag, region)
}

// ResolveRegion decides which region to search, in the order a caller's choices
// are expressed: an explicit override, then SPINLOOP_CLOUD_REGION, then the AWS
// region chain. Nothing is assumed past that. The region is half of a plane's
// identity, so guessing it would mean acting on a plane nobody meant to name —
// on a shared plane, somebody else's.
func ResolveRegion(ctx context.Context, explicit string, getenv func(string) string) (string, error) {
	for _, candidate := range []string{explicit, getenv("SPINLOOP_CLOUD_REGION"), getenv("AWS_REGION")} {
		if candidate != "" {
			return candidate, nil
		}
	}
	if region := planeChainRegion(ctx); region != "" {
		return region, nil
	}
	return "", errors.New(
		"cannot tell which cloud to work on: no AWS region resolves — set SPINLOOP_CLOUD_REGION or AWS_REGION, " +
			"or a region on the AWS profile your credentials belong to")
}

// requiredEndpointVars are the endpoints no cloud command can work without:
// every lifecycle call — start, stop, status, pause, restart — goes through one
// of the two. A complete set of variables is a plane outright; a partial one is
// only an override on top of a discovered plane.
//
// The rest (deploy, seed, env, stats, update, schedules) are optional in the
// same way a stack from before those Lambdas existed is optional: the command
// that needs one names the variable to set, and a Spinloop that only ever
// starts and stops a node states no more than it uses.
var requiredEndpointVars = []string{
	"SPINLOOP_CLOUD_START_URL",
	"SPINLOOP_CLOUD_STOP_URL",
}

// planeFromVariables assembles a plane outright from the variables, when they
// name every endpoint a command cannot work without. A plane stated this way
// needs no stack and no permission to read one, which is how CI and a machine
// without DescribeStacks drive one.
func planeFromVariables(getenv func(string) string) (Config, bool) {
	for _, key := range requiredEndpointVars {
		if getenv(key) == "" {
			return Config{}, false
		}
	}
	return overlayEndpoints(Config{}, getenv), true
}

// overlayVariables applies the SPINLOOP_CLOUD_* variables over a resolved
// plane, one value at a time, so a caller can point one endpoint elsewhere
// while the rest stay as the stack says. The memo keeps the discovered plane
// and overlays on the way out, so a variable set between calls takes effect
// without invalidating the stack read.
func overlayVariables(plane ControlPlane, getenv func(string) string) ControlPlane {
	plane.Config = overlayEndpoints(plane.Config, getenv)
	return plane
}

// overlayEndpoints is overlayVariables one level down, on the endpoints alone.
func overlayEndpoints(cfg Config, getenv func(string) string) Config {
	for _, o := range []struct {
		key string
		set *string
	}{
		{"SPINLOOP_CLOUD_START_URL", &cfg.StartURL},
		{"SPINLOOP_CLOUD_STOP_URL", &cfg.StopURL},
		{"SPINLOOP_CLOUD_DEPLOY_URL", &cfg.DeployURL},
		{"SPINLOOP_CLOUD_STATS_URL", &cfg.StatsURL},
		{"SPINLOOP_CLOUD_ENV_URL", &cfg.EnvURL},
		{"SPINLOOP_CLOUD_SEED_URL", &cfg.SeedURL},
		{"SPINLOOP_CLOUD_UPDATE_URL", &cfg.UpdateURL},
		{"SPINLOOP_CLOUD_SCHEDULE_URL", &cfg.ScheduleURL},
	} {
		if v := getenv(o.key); v != "" {
			*o.set = v
		}
	}
	return cfg
}

// ResolveNode resolves the cloud node named by name on the plane req
// resolves to: the plane's endpoints, its region, and the name as the
// identifier the shared lifecycle Lambdas act on.
//
// This is the only way a cloud node is resolved. Nothing on disk is consulted,
// no name resolves by a different rule from any other, and a node the
// plane does not hold is settled by the plane's inventory rather than by a call
// that happened to fail.
func ResolveNode(ctx context.Context, name string, req PlaneRequest) (Config, error) {
	if !IsNodeName(name) {
		return Config{}, fmt.Errorf(
			"%q is not a node name: a node name is a plain identifier, with no path", name)
	}
	plane, err := ResolvePlane(ctx, req)
	if err != nil {
		return Config{}, err
	}
	cfg := plane.Config
	cfg.Node = name
	return cfg, nil
}

// ForgetPlanes drops every remembered resolution, so the next call reads the
// stack again. A redeployed plane heals on its own at the first rejected call
// (see refreshPlane); this is the seam a test uses to watch a process resolve
// twice, and the one a caller uses just after it re-bootstrapped a plane.
func ForgetPlanes() {
	planeMemo.mu.Lock()
	defer planeMemo.mu.Unlock()
	planeMemo.byTag = map[string]*planeEntry{}
}

// refreshPlane re-resolves a plane after a control call was rejected or its host
// proved unreachable, and reports the config to retry with. It is the second
// half of keeping the resolution in memory rather than on disk: the answer is
// never validated on a timer, it is corrected by the failure that proves it
// wrong.
//
// It is deliberately conservative — a re-resolution that yields the same
// endpoints is not a fix, so the original failure stands rather than being
// retried once more against the host that just refused.
func refreshPlane(ctx context.Context, cfg Config, getenv func(string) string) (Config, bool) {
	forgetPlane(cfg.Region)
	plane, err := ResolvePlane(ctx, PlaneRequest{Region: cfg.Region, GetName: getenv})
	if err != nil || sameEndpoints(cfg, plane.Config) {
		// Same endpoints means the plane did not move, so the failure was about
		// something else — permission, a stopped instance, the caller's input.
		return cfg, false
	}
	plane.Config.Node = cfg.Node
	return plane.Config, true
}

// sameEndpoints compares the endpoints that decide where a call goes.
func sameEndpoints(a, b Config) bool {
	return a.StartURL == b.StartURL && a.StopURL == b.StopURL && a.DeployURL == b.DeployURL &&
		a.StatsURL == b.StatsURL && a.EnvURL == b.EnvURL && a.SeedURL == b.SeedURL &&
		a.UpdateURL == b.UpdateURL && a.ScheduleURL == b.ScheduleURL
}

// swapEndpoint rewrites a URL built from one of a plane's endpoints onto that
// plane's current endpoint for the same role, keeping whatever query the caller
// added — a start URL's retainUntil, a stop URL's action. ok is false when the
// URL was not built from an endpoint the plane knows.
func swapEndpoint(cfg, next Config, called string) (string, bool) {
	for _, pair := range [][2]string{
		{cfg.StartURL, next.StartURL},
		{cfg.StopURL, next.StopURL},
		{cfg.DeployURL, next.DeployURL},
		{cfg.StatsURL, next.StatsURL},
		{cfg.EnvURL, next.EnvURL},
		{cfg.SeedURL, next.SeedURL},
		{cfg.UpdateURL, next.UpdateURL},
		{cfg.ScheduleURL, next.ScheduleURL},
	} {
		old, newer := pair[0], pair[1]
		if old == "" || newer == "" || old == newer {
			continue
		}
		if strings.HasPrefix(called, old) {
			return newer + strings.TrimPrefix(called, old), true
		}
	}
	return "", false
}

// stalePlane reports whether a failure is the kind that means "this plane is not
// where you think it is": the host cannot be reached at all, or the call was
// rejected by an expired or invalid credential. Both are what a plane re-boot-
// strapped under a long-lived process looks like. A refusal for lacking
// permission, and every control-plane reply that explains itself, are not:
// retrying them just fails again.
func stalePlane(err error) bool {
	if err == nil {
		return false
	}
	if credentialError(err) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return unreachableHost(urlErr.Err)
	}
	return unreachableHost(err)
}

// unreachableHost reports a failure to reach a host at all — DNS, refused,
// reset, no route — as opposed to a host that answered and refused.
func unreachableHost(err error) bool {
	if err == nil {
		return false
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	text := err.Error()
	for _, marker := range []string{"no such host", "connection refused", "connection reset", "network is unreachable"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
