# getting started

## install

Requires Go 1.27.1. From the repository root, run `go install ./cmd/ember` to install Ember in Go's bin directory, make sure that directory is on your `PATH`. To build a binary beside the source instead, run:

```sh
go build ./cmd/ember
```

This produces `ember.exe` on Windows and `ember` on Linux.

## hello example

From the repository root:

```sh
cd examples/hello
ember run
```

The checked-in `ember.toml` uses Microsoft's `cl`. Start Ember from a Developer PowerShell with the Visual Studio C++ tools enabled. If you have Clang installed on Windows, select its sample config first:

```sh
cp ember.clang.toml ember.toml
ember run
```

On Linux or WSL, select the GCC config instead, GCC must be on `PATH`:

```sh
cp ember.gcc.toml ember.toml
ember run
```

The example prints a counter once a second. Edit and save `main.c` to rebuild and restart it. Press `q` or Ctrl+C to quit. See the [WSL notes](platform-notes.md#wsl) if file changes aren't detected under `/mnt`.
