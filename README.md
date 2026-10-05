# ember

Ember watches a C or C++ project, runs the build command you give it, and
restarts the program after a successful build. If a build fails, the old
program keeps running. It works with the build system you already use. Ember
doesn't manage compiler state or patch binaries.

## install

Requires Go 1.27.1. From the repo root:

```sh
go install ./cmd/ember # install to bin
go build ./cmd/ember # build a binary
```

This produces `ember.exe` on Windows and `ember` on Linux.

## quick start

From the repo root:

```sh
cd examples/hello
ember run
```

`ember.toml` uses Microsoft's `cl` compiler by default, but you can use `clang` on Windows or the GCC config on Linux/WSL.

While Ember is running, edit and save `main.c` to rebuild and restart the example. Press `q` or `Ctrl+C` to quit.

On WSL, if changes under `/mnt` aren't detected, enable polling:

```toml
[watch]
poll = true
```

## commands

Control commands require a running `ember run` for the same project.

You can run them from the project directory or any subdirectory. Ember searches upward for `ember.toml`.

| Command               | Description                                             |
| --------------------- | ------------------------------------------------------- |
| `ember run`           | Watch, build, and run the program.                      |
| `ember reload`        | Build and restart the program.                          |
| `ember build`         | Build without restarting.                               |
| `ember stop`          | Stop the program.                                       |
| `ember start`         | Start the last successful build.                        |
| `ember status`        | Show the current state and last build details.          |
| `ember init`          | Create a starter `ember.toml`.                          |
| `ember run --verbose` | Show debug logs, changed paths, build IDs, and timings. |
| `ember -h`            | Show help.                                              |

`--verbose` can also be placed before the command.

`ember init` won't overwrite an existing `ember.toml`.

### `ember run` controls

| Key            | Action                                    |
| -------------- | ----------------------------------------- |
| `r`            | Build and restart.                        |
| `s`            | Stop, or start the last successful build. |
| `c`            | Clear the terminal.                       |
| `q` / `Ctrl+C` | Quit Ember.                               |

### exit codes

Commands return 0 on success, 1 on error, and 2 when a control command can't find a running Ember instance.

## configuration

`ember.toml` uses TOML. `ember init` creates a basic config like this:

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

- `watch.paths`: directories to watch.
- `include` / `ignore`: glob patterns for files to include or ignore.
- `debounce_ms`: delay before rebuilding.
- `poll`: use polling instead of filesystem events. Useful on WSL.
- `build.cmd`: build command.
- `build.stops_running`: stop the current program before building.
- `run.cmd`: command to run.
- `run.cwd`: working directory.
- `run.env`: environment variables.
- `run.kill_timeout_ms`: time to wait before force-stopping.
- `ui.color`: enable colored output.

Commands use the system shell. `{out}` can be used in `build.cmd` or `run.cmd` for a unique output path under `.ember/bin/`.

Ember stores its state and build outputs in `.ember/`. Delete it while Ember is stopped to reset the project state.
