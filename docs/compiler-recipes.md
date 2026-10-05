# compiler recipes

Use these `[build]` and `[run]` sections in `ember.toml`. Direct compiler recipes write to a fresh `{out}` path, leaving the old executable alone until the build succeeds.

## cl (msvc)

On Windows, run Ember from a Developer PowerShell with the Visual Studio C++ tools enabled.

```toml
[build]
cmd = "cl /nologo /W4 /Fe{out} main.c"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

## clang-cl

Install Clang and make sure its linker and runtime are available for the target platform.

```toml
[build]
cmd = "clang-cl /nologo /W4 /Fe{out} main.c"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

## clang

Install Clang and the C runtime/linker for your target platform.

```toml
[build]
cmd = "clang -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

## gcc

Install GCC and make sure `gcc` is on `PATH`. This recipe is for Linux or WSL.

```toml
[build]
cmd = "gcc -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

## zig cc

Install Zig and make sure `zig` is on `PATH`.

```toml
[build]
cmd = "zig cc -std=c11 -Wall -Wextra -Wpedantic -O0 main.c -o {out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```
