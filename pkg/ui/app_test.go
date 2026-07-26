package ui

import "testing"

// TestScreenTypeString verifies the Stringer implementation on ScreenType,
// including the default branch for an out-of-range value.
func TestScreenTypeString(t *testing.T) {
	cases := []struct {
		name   string
		screen ScreenType
		want   string
	}{
		{name: "pods screen", screen: ScreenPods, want: "Pods"},
		{name: "deployments screen", screen: ScreenDeployments, want: "Deployments"},
		{name: "out-of-range value", screen: ScreenType(99), want: "Unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.screen.String()
			if got != tc.want {
				t.Errorf("ScreenType(%d).String() = %q, want %q", tc.screen, got, tc.want)
			}
		})
	}
}
