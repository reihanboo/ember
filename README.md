# ember

Ember watches a C or C++ project, runs the build command you give it, and restarts the program after a successful build. If a build fails, the old program keeps running. It works with the build system you already use. Ember doesn't manage compiler state or patch binaries.

## quick start

Requires Go 1.27.1. From the repository root:

```sh
go install ./cmd/ember
cd examples/hello
ember run
```

The Windows example uses `cl`, so start it from a Developer PowerShell with the Visual Studio C++ tools enabled. Linux and WSL users can switch the sample to GCC with `cp ember.gcc.toml ember.toml`. Press `q` or Ctrl+C to quit.

## documentation map

- [Getting started](docs/getting-started.md): install, build, and run the hello example
- [Commands and keys](docs/commands.md): CLI reference, run controls, and exit codes
- [Configuration](docs/configuration.md): `ember.toml` fields and defaults
- [Compiler recipes](docs/compiler-recipes.md): cl, clang-cl, clang, GCC, and Zig
- [Build-system recipes](docs/build-systems.md): CMake with Ninja and Make
- [Platform notes](docs/platform-notes.md): locked executables and WSL
- [Zed](docs/editors/zed.md): reload the running project from a task or key binding
- [Neovim](docs/editors/nvim.md): reload from a keymap or optional save hook
