package build

import (
	"errors"
	"testing"
)

func TestCommandHint(t *testing.T) {
	path := map[string]bool{
		"cmake": true,
		"gcc":   true,
	}
	lookPath := func(command string) (string, error) {
		if path[command] {
			return command, nil
		}
		return "", errors.New("not found")
	}
	tests := []struct {
		name    string
		command string
		windows bool
		want    string
	}{
		{name: "available command", command: "cmake --build build", want: ""},

		{name: "shell keyword", command: "if true; then make; fi", want: ""},
		{name: "missing command", command: "clang++ -o app main.cpp", want: "command not found: clang++"},
		{name: "missing cl gets developer shell hint", command: "cl /nologo main.c", windows: true, want: "cl not found on PATH; run ember from a Developer PowerShell"},
		{name: "missing cl exe gets developer shell hint", command: "cl.exe main.c", windows: true, want: "cl not found on PATH; run ember from a Developer PowerShell"},
		{name: "cl on non windows platform is generic", command: "cl main.c", want: "command not found: cl"},
		{name: "shell builtin", command: "echo built >> build.log", want: ""},
		{name: "windows shell builtin", command: "exit /b 7", windows: true, want: ""},
		{name: "quoted executable", command: `"C:\Program Files\tool.exe" --version`, windows: true, want: `command not found: C:\Program Files\tool.exe`},
		{name: "empty command", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := commandHint(test.command, lookPath, test.windows); got != test.want {
				t.Errorf("commandHint(%q) = %q, want %q", test.command, got, test.want)
			}
		})
	}
}
