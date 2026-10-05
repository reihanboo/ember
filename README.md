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

The starter config builds to a fixed `build/app.exe` path. Unless your CMake
project is set up to write to `{out}`, set `stops_running = true` for that
recipe so Windows can replace the executable.

## build recipes

These are the `[build]` and `[run]` parts to drop into `ember.toml`. The
compiler recipes write a fresh executable to `{out}`, so Ember can leave the
old program running until the replacement is ready.

### cl (MSVC)

Not verified here: `cl` is not on this machine's PATH. Run Ember from a
Developer PowerShell with the Visual Studio C++ tools enabled.

```toml
[build]
cmd = "cl /nologo /W4 /Fe{out} main.c"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

### clang-cl

Verified here with Clang 23.1.2 on Windows; it compiled and ran the hello
example.

```toml
[build]
cmd = "clang-cl /nologo /W4 /Fe{out} main.c"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

### clang

Verified here with Clang 23.1.2 on Windows; it compiled and ran the hello
example.

```toml
[build]
cmd = "clang -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

### gcc

Not verified here: GCC is not installed on this machine. This is a Linux or
WSL recipe.

```toml
[build]
cmd = "gcc -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

### zig cc

Not verified here: Zig is not installed on this machine.

```toml
[build]
cmd = "zig cc -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

### CMake with Ninja

Not verified here: neither CMake nor Ninja is installed. This version needs
an executable target named `app` that honors `EMBER_OUTPUT`. For example:

```cmake
add_executable(app main.c)

if(DEFINED EMBER_OUTPUT)
  get_filename_component(ember_output "${EMBER_OUTPUT}" ABSOLUTE
                         BASE_DIR "${CMAKE_SOURCE_DIR}")
  get_filename_component(ember_output_dir "${ember_output}" DIRECTORY)
  get_filename_component(ember_output_name "${ember_output}" NAME_WE)
  set_target_properties(app PROPERTIES
    RUNTIME_OUTPUT_DIRECTORY "${ember_output_dir}"
    OUTPUT_NAME "${ember_output_name}"
  )
endif()
```

Then use this config. Ninja is a single-configuration generator here, so the
result lands at the exact `{out}` path rather than in a config subdirectory.

```toml
[build]
cmd = "cmake -S . -B build -G Ninja -DEMBER_OUTPUT={out} && cmake --build build"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

### make

Not verified here: Make is not installed. This expects your `Makefile` to
write its executable to the `OUT` variable supplied on the command line.

```toml
[build]
cmd = "make OUT={out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

### when the output name is fixed

Windows won't let a compiler overwrite an executable that's still running.
Using `{out}` avoids that: each build gets a different file, and Ember switches
to it only after the build succeeds. If your build system always writes to the
same filename, stop the old program before building instead:

```toml
[build]
cmd = "cmake --build build"
stops_running = true

[run]
cmd = "build\\app.exe"
cwd = "."
```

With `stops_running = true`, a failed build leaves the program stopped; with
it set to `false`, a failed build leaves the old program running. The fixed
output recipe above is unverified here because CMake isn't installed.

### WSL

On WSL, file notifications can be unreliable for projects under `/mnt/c` and
other mounted Windows paths. Set `poll = true` under `[watch]`, or keep the
checkout in the WSL Linux filesystem for more reliable notifications. Use the
Linux toolchain recipes from WSL; a Windows `cl` installation isn't
automatically available to Linux shell commands.

Ember stores its state and build outputs in `.ember/`. Delete it while Ember is stopped to reset the project state.
