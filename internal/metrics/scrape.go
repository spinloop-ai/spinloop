package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// scrapeClient is a package variable so tests can substitute it. The timeout
// mirrors the Lambda's curl --max-time 5.
var scrapeClient = &http.Client{Timeout: 5 * time.Second}

// ScrapeTarget names where a running engine's Prometheus metrics live: the
// engine's own base URL, the metric dialect it speaks, and the API key it
// gates /metrics behind (empty for an ungated engine).
type ScrapeTarget struct {
	BaseURL string
	Engine  string
	APIKey  string
}

// ScrapeTokenStats fetches the engine's /metrics and parses its token and
// request counters. An unreachable endpoint, a non-200 reply, or output with
// no recognisable metrics all return an error — the caller omits engine stats
// and carries on, per the engine-metrics spec.
func ScrapeTokenStats(ctx context.Context, target ScrapeTarget) (*TokenStats, error) {
	u, err := url.Parse(strings.TrimSuffix(target.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("engine base URL %q: %w", target.BaseURL, err)
	}
	// BASEURL conventionally ends in /v1 (the OpenAI-style API root), but
	// /metrics is served at the server root.
	u.Path = strings.TrimSuffix(u.Path, "/v1") + "/metrics"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if target.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+target.APIKey)
	}
	resp, err := scrapeClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("engine metrics returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	tokens := ParseTokenStats(string(body), target.Engine)
	if tokens == nil {
		return nil, fmt.Errorf("no %s metrics in scrape", target.Engine)
	}
	return tokens, nil
}

// CheckEngineReady asks whether the engine at target can currently serve
// requests, distinct from ScrapeTokenStats's assumption that it already can:
// llama.cpp binds its port before it has finished loading weights, and
// answers /health with a non-200 status until it has. The probe is
// deliberately unauthenticated — an engine that requires a key correctly
// answers 401, and that counts as ready: the point is whether the process is
// up and serving, not whether this caller is authorised. Any other status, a
// request error (including a still-refused connection while the process
// binds its port), or a malformed base URL all mean not ready.
func CheckEngineReady(ctx context.Context, target ScrapeTarget) bool {
	u, err := url.Parse(strings.TrimSuffix(target.BaseURL, "/"))
	if err != nil {
		return false
	}
	// BASEURL conventionally ends in /v1; /health is served at the server root.
	u.Path = strings.TrimSuffix(u.Path, "/v1") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false
	}
	resp, err := scrapeClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
}
