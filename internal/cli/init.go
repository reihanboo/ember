package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
)

const starterConfig = `[watch]
paths = ["src", "include"]
include = ["**/*.c", "**/*.cpp", "**/*.h", "**/*.hpp", "CMakeLists.txt"]
ignore = ["build/**", ".git/**", ".ember/**", "**/*.obj", "**/*.pdb", "**/*.o", "**/*~", "**/*.swp", "**/.#*"]
debounce_ms = 150
poll = false

[build]
cmd = "cmake --build build"
stops_running = false

[run]
cmd = "build\\app.exe --flag"
cwd = "."
env = { FOO = "bar" }
kill_timeout_ms = 2000

[ui]
color = true
`

func initCommand() error {
	const path = "ember.toml"
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("ember.toml already exists; refusing to overwrite")
		}
		return fmt.Errorf("create %s: %w", path, err)
	}

	if _, err := io.WriteString(file, starterConfig); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
