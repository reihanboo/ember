package ui

import (
	"os"
	"testing"
)

func TestReadKeySkipsNonTTYInput(t *testing.T) {
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer output.Close()

	key, read, err := ReadKey(input)
	if err != nil {
		t.Fatal(err)
	}
	if read {
		t.Errorf("ReadKey() read a key from non-TTY input: %q", key)
	}
	if key != 0 {
		t.Errorf("ReadKey() key = %q, want zero for non-TTY input", key)
	}
}
