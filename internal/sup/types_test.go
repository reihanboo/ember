package sup

import "testing"

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{state: Idle, want: "Idle"},
		{state: Building, want: "Building"},
		{state: Running, want: "Running"},
		{state: BuildFailed, want: "BuildFailed"},
		{state: State(255), want: "Unknown"},
	}

	for _, test := range tests {
		if got := test.state.String(); got != test.want {
			t.Errorf("State(%d).String() = %q, want %q", test.state, got, test.want)
		}
	}
}
