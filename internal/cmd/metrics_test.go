package cmd

import "testing"

func TestMetricsCompatibilityCommandsAreRegistered(t *testing.T) {
	for _, name := range []string{"summary", "aggregate", "insights"} {
		found := false
		for _, command := range metricsCmd.Commands() {
			if command.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fork compatibility metrics command %q is not registered", name)
		}
	}
}
