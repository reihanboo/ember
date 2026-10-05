# platform notes

## locked executables on windows

Windows won't let a compiler overwrite an executable that's still running. Using `{out}` avoids that. Each build writes a different file under `.ember/bin/`, and Ember switches to it only after the build succeeds. A failed build leaves the old program running.

If a build system always writes to one fixed filename, set `stops_running = true` so Ember stops the program before the build. For example:

```toml
[build]
cmd = "cmake --build build"
stops_running = true

[run]
cmd = "build\\app.exe"
cwd = "."
```

With this setting, a failed build leaves the program stopped. The starter config uses a fixed `build/app.exe` path, so enable `stops_running` unless the CMake project is configured to write to `{out}`.

## wsl

File notifications can be unreliable for projects under `/mnt/c` and other mounted Windows paths. Set `poll = true` under `[watch]`, or keep the checkout in the WSL Linux filesystem for more reliable notifications. Use the Linux compiler recipes from WSL, a Windows `cl` installation isn't automatically available to Linux shell commands.
