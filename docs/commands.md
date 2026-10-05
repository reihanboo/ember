# commands and keys

`reload`, `build`, `stop`, `start`, and `status` need a live `ember run` for the same project. Ember searches upward from the current directory for `ember.toml`. `ember init` instead writes to the current directory.

| Command | Description |
| --- | --- |
| `ember run` | Watch, build, and run the program. |
| `ember reload` | Build and restart the program now. |
| `ember build` | Build without restarting. |
| `ember stop` | Stop the program. |
| `ember start` | Start the last successfully built program. |
| `ember status` | Show the current state and last build details. |
| `ember init` | Create a starter `ember.toml` in the current directory. |
| `ember run --verbose` | Show changed paths, build IDs, and timings. |
| `ember --version` | Show the release version. |
| `ember -h` | Show the command list. |

`--verbose` is global and can also go before the command: `ember --verbose run`.

Inside `ember run`:

| Key | Action |
| --- | --- |
| `r` | Build and restart. |
| `s` | Stop, or start the last successful build. |
| `c` | Clear the terminal. |
| `q` or Ctrl+C | Quit Ember. |

`ember init` won't overwrite an existing config. Commands return 0 on success, 1 on error, and 2 when a control command can't find a running Ember instance.
