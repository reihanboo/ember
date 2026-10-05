package ui

import (
	"os"
	"testing"
)

func TestKeyEventForByte(t *testing.T) {
	for _, test := range []struct {
		key  byte
		want KeyEvent
		ok   bool
	}{
		{key: 'r', want: ReloadKey, ok: true},
		{key: 's', want: ToggleKey, ok: true},
		{key: 'c', want: ClearKey, ok: true},
		{key: 'q', want: QuitKey, ok: true},
		{key: 3, want: QuitKey, ok: true},
		{key: 'x'},
	} {
		got, ok := KeyEventForByte(test.key)
		if got != test.want || ok != test.ok {
			t.Errorf("KeyEventForByte(%q) = (%d, %t), want (%d, %t)", test.key, got, ok, test.want, test.ok)
		}
	}
}

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
