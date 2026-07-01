package main

import "testing"

func TestRecordRetryConvergenceTracksRepeatedFailures(t *testing.T) {
	stats := executionStats{}
	stats.recordRetryConvergence("R1", 1, "undefined symbol", "undefined symbol")
	if stats.convergenceSamples != 0 || stats.convergenceAlerts != 0 {
		t.Fatalf("unexpected counts before threshold: %#v", stats)
	}

	stats.recordRetryConvergence("R1", 2, "undefined symbol", "undefined symbol")
	if stats.convergenceSamples != 1 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected one alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}

	stats.recordRetryConvergence("R1", 3, "undefined symbol", "type mismatch")
	if stats.convergenceSamples != 2 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected convergence sample without alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}
}
