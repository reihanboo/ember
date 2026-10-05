# configuration

Ember looks upward from the current working directory for `ember.toml`. `ember init` creates a starter file in the current directory and refuses to overwrite an existing one.

```toml
[watch]
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
```

- `watch.paths` lists directories to watch recursively. `include` and `ignore` are glob patterns relative to the project root.
- `watch.debounce_ms` waits for a quiet period before rebuilding. Editor saves that write and rename temporary files are handled as changes.
- `watch.poll = true` uses polling instead of filesystem events, this can help on WSL mounted filesystems.
- `build.cmd` and `run.cmd` are shell command strings. Ember uses `cmd /C` on Windows and `sh -c` on Linux.
- `{out}` in either command expands to a unique executable path under `.ember/bin/`. See the [compiler recipes](compiler-recipes.md) and [build-system recipes](build-systems.md).
- `build.stops_running` stops the current program before building. See [platform notes](platform-notes.md) for the locked-executable tradeoff.
- `run.cwd` is relative to the project root unless absolute. `run.env` adds or replaces environment variables for the program.
- `run.kill_timeout_ms` is the graceful stop period before Ember force-stops the process tree.
- `ui.color` enables colored status and log lines.

Ember keeps its control file and build outputs in `.ember/` in the project root. Delete that directory while Ember is stopped to reset the local state.
