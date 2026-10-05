package outdir

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestNextNumbersOutputPaths(t *testing.T) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	first := Next()
	second := Next()
	if want := filepath.Join(".ember", "bin", "app-1"+suffix); first != want {
		t.Errorf("first Next() = %q, want %q", first, want)
	}
	if want := filepath.Join(".ember", "bin", "app-2"+suffix); second != want {
		t.Errorf("second Next() = %q, want %q", second, want)
	}
}

func TestSubstituteReplacesEveryOutputPath(t *testing.T) {
	cmd := "compiler -o {out} && linker {out}"
	if got, want := Substitute(cmd, "app.bin"), "compiler -o app.bin && linker app.bin"; got != want {
		t.Errorf("Substitute() = %q, want %q", got, want)
	}
}

func TestSubstituteWithoutOutputPath(t *testing.T) {
	cmd := "cmake --build build"
	if got := Substitute(cmd, "app.bin"); got != cmd {
		t.Errorf("Substitute() = %q, want unchanged command %q", got, cmd)
	}
}
