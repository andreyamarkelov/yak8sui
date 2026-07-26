package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestStatusColor is a table-driven test: every scenario is one row in the
// `cases` slice, and t.Run turns each row into its own named subtest.
func TestStatusColor(t *testing.T) {
	cases := []struct {
		name   string      // human-readable description, shown in test output
		status string      // input to statusColor
		want   tcell.Color // the color we expect back
	}{
		{name: "running is green", status: "Running", want: tcell.ColorGreen},
		{name: "succeeded is green", status: "Succeeded", want: tcell.ColorGreen},
		{name: "pending is yellow", status: "Pending", want: tcell.ColorYellow},
		{name: "failed falls through to red", status: "Failed", want: tcell.ColorRed},
		{name: "unknown status is red", status: "CrashLoopBackOff", want: tcell.ColorRed},
		{name: "empty string is red", status: "", want: tcell.ColorRed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := statusColor(tc.status)
			if got != tc.want {
				t.Errorf("statusColor(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
