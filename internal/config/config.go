package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Watch WatchConfig `toml:"watch"`
	Build BuildConfig `toml:"build"`
	Run   RunConfig   `toml:"run"`
	UI    UIConfig    `toml:"ui"`
}

type WatchConfig struct {
	Paths      []string `toml:"paths"`
	Include    []string `toml:"include"`
	Ignore     []string `toml:"ignore"`
	DebounceMS int      `toml:"debounce_ms"`
	Poll       bool     `toml:"poll"`
}

type BuildConfig struct {
	Cmd          string `toml:"cmd"`
	StopsRunning bool   `toml:"stops_running"`
}

type RunConfig struct {
	Cmd           string            `toml:"cmd"`
	Cwd           string            `toml:"cwd"`
	Env           map[string]string `toml:"env"`
	KillTimeoutMS int               `toml:"kill_timeout_ms"`
}

type UIConfig struct {
	Color bool `toml:"color"`
}

type NotFoundError struct {
	StartDir string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("ember.toml not found from %q", e.StartDir)
}

func Defaults() Config {
	return Config{
		Watch: WatchConfig{
			Paths:   []string{"src", "include"},
			Include: []string{"**/*.c", "**/*.cpp", "**/*.h", "**/*.hpp", "CMakeLists.txt"},
			Ignore: []string{
				"build/**",
				".git/**",
				".ember/**",
				"**/*.obj",
				"**/*.pdb",
				"**/*.o",
				"**/*~",
				"**/*.swp",
				"**/.#*",
			},
			DebounceMS: 150,
		},
		Run: RunConfig{
			Cwd:           ".",
			Env:           map[string]string{},
			KillTimeoutMS: 2000,
		},
		UI: UIConfig{Color: true},
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}
	return cfg, nil
}

func Find(startDir string) (string, string, error) {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve start directory %q: %w", startDir, err)
	}
	start := current

	for {
		configPath := filepath.Join(current, "ember.toml")
		info, err := os.Stat(configPath)
		if err == nil && !info.IsDir() {
			return current, configPath, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("check config %q: %w", configPath, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", "", &NotFoundError{StartDir: start}
}
