package main

import (
	"strings"
	"testing"
)

// The retention deadline rides the stats read and is drawn beside the
// last-active line in every surface that draws that line. The control plane is
// the single home of the "is it still in the future?" judgement, so these
// renderers only ever ask "does the read carry a deadline?" and draw it when
// present, and nothing when it is empty.

const retainedDeadline = "2030-01-02T04:00:00Z"

func TestRemoteMetricsBarShowsRetainUntil(t *testing.T) {
	statsServer(t, `{
		"environment": "dev",
		"state": "running",
		"instanceType": "g6e.xlarge",
		"modelId": "unsloth/Qwen3.6-27B",
		"cpu": {"utilization": 24},
		"retainUntil": "`+retainedDeadline+`"
	}`)

	out := captureStdout(t, func() {
		if err := cmdRemoteMetrics([]string{"--format=bar"}); err != nil {
			t.Fatalf("cmdRemoteMetrics: %v", err)
		}
	})
	if !strings.Contains(out, "retain until "+retainedDeadline) {
		t.Errorf("bar format missing the retain-until line:\n%s", out)
	}
	// Beside last-active, before the bars: a fact about the endpoint, not a
	// utilisation reading.
	active := strings.Index(out, "last active")
	retain := strings.Index(out, "retain until")
	bars := strings.Index(out, "CPU")
	if !(retain >= 0 && bars >= 0 && retain < bars) {
		t.Errorf("retain-until line is not before the bars:\n%s", out)
	}
	if active >= 0 && !(retain > active) {
		t.Errorf("retain-until line is not beside (after) last-active:\n%s", out)
	}
}

func TestRemoteMetricsTableShowsRetainUntil(t *testing.T) {
	statsServer(t, `{
		"environment": "dev",
		"state": "running",
		"instanceType": "g6e.xlarge",
		"modelId": "unsloth/Qwen3.6-27B",
		"cpu": {"utilization": 24},
		"retainUntil": "`+retainedDeadline+`"
	}`)

	out := captureStdout(t, func() {
		if err := cmdRemoteMetrics([]string{"--format=table"}); err != nil {
			t.Fatalf("cmdRemoteMetrics: %v", err)
		}
	})
	// A key-value row, padded to the key column its neighbours use.
	if !strings.Contains(out, "retain until: "+retainedDeadline) {
		t.Errorf("table format missing the retain-until row:\n%s", out)
	}
}

// A stopped environment can still be retained: the deadline is the control
// plane's, not the engine's, so the line survives the non-running short-circuit
// in both formats.
func TestRemoteMetricsStoppedRetainedStillShowsRetainUntil(t *testing.T) {
	statsServer(t, `{
		"environment": "dev",
		"state": "stopped",
		"runner": "llamacpp",
		"modelId": "unsloth/Qwen3.6-27B",
		"retainUntil": "`+retainedDeadline+`"
	}`)

	for format, want := range map[string]string{
		"bar":   "retain until " + retainedDeadline,
		"table": "retain until: " + retainedDeadline,
	} {
		t.Run(format, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := cmdRemoteMetrics([]string{"--format=" + format}); err != nil {
					t.Fatalf("cmdRemoteMetrics: %v", err)
				}
			})
			if !strings.Contains(out, want) {
				t.Errorf("%s format dropped the deadline for a stopped, retained endpoint:\n%s", format, out)
			}
		})
	}
}

// No deadline on the read, no line: the renderer does not invent one, and it
// does not disturb the rest of the report.
func TestRemoteMetricsOmitsRetainUntilWhenAbsent(t *testing.T) {
	statsServer(t, `{
		"environment": "dev",
		"state": "running",
		"cpu": {"utilization": 24},
		"lastActiveAt": "2026-08-10T10:00:00Z",
		"idleSeconds": 125
	}`)

	for _, format := range []string{"bar", "table"} {
		t.Run(format, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := cmdRemoteMetrics([]string{"--format=" + format}); err != nil {
					t.Fatalf("cmdRemoteMetrics: %v", err)
				}
			})
			if strings.Contains(out, "retain until") {
				t.Errorf("%s format claimed a deadline the read does not carry:\n%s", format, out)
			}
			if !strings.Contains(out, "last active") {
				t.Errorf("%s format lost the rest of the report:\n%s", format, out)
			}
		})
	}
}

// The deadline rides the node's stats, so `fleet metrics` draws it beside
// last-active whatever the node's state. A faked daemon carrying the field
// stands in for a retained remote environment on the same render path.
func TestFleetMetricsShowsRetainUntil(t *testing.T) {
	fleetNodeWithMetrics(t, map[string]any{
		"state":       "running",
		"runner":      "llamacpp",
		"modelId":     "org/qwen",
		"cpu":         map[string]any{"utilization": 30.0},
		"retainUntil": retainedDeadline,
	})

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"metrics"}); err != nil {
			t.Fatalf("cmdFleet metrics: %v", err)
		}
	})
	if !strings.Contains(out, "retain until "+retainedDeadline) {
		t.Errorf("fleet metrics missing the retain-until line:\n%s", out)
	}
}

// A node whose read carries no deadline draws no line, so a local daemon node —
// which never has one — is simply quieter on that line.
func TestFleetMetricsOmitsRetainUntilWhenAbsent(t *testing.T) {
	fleetNodeWithMetrics(t, map[string]any{
		"state":        "running",
		"runner":       "llamacpp",
		"modelId":      "org/qwen",
		"cpu":          map[string]any{"utilization": 30.0},
		"lastActiveAt": "2026-08-10T10:00:00Z",
		"idleSeconds":  125,
	})

	out := captureStdout(t, func() {
		if err := cmdFleet([]string{"metrics"}); err != nil {
			t.Fatalf("cmdFleet metrics: %v", err)
		}
	})
	if strings.Contains(out, "retain until") {
		t.Errorf("fleet metrics claimed a deadline a local node never carries:\n%s", out)
	}
}
