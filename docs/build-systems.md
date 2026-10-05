# build-system recipes

## cmake with ninja

Install CMake and Ninja. This recipe uses `{out}` when the executable target is named `app` and its output path is configured from `EMBER_OUTPUT`. In `CMakeLists.txt`, declare the target and set its runtime output path like this:

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

Then use this config. Ninja is a single-configuration generator here, so the output lands at the exact `{out}` path.

```toml
[build]
cmd = "cmake -S . -B build -G Ninja -DEMBER_OUTPUT={out} && cmake --build build"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```

## make

Install Make. This expects the `Makefile` to use the `OUT` variable as the executable's output path.

```toml
[build]
cmd = "make OUT={out}"
stops_running = false

[run]
cmd = "{out}"
cwd = "."
```
