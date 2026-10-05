package ui

import "testing"

func TestEnableColorRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if EnableColor() {
		t.Fatal("EnableColor() = true with NO_COLOR set, want false")
	}
}
