# Zed

This task asks Ember's running supervisor to rebuild and restart the program. Start `ember run` for the project first, and make sure `ember` is available on Zed's `PATH`.

## Project task

Add this entry to the project's `.zed/tasks.json` file. Zed saves edited buffers before running it, then invokes `ember reload` from the worktree root.

```json
[
  {
    "label": "Ember: Reload",
    "command": "ember",
    "args": ["reload"],
    "cwd": "$ZED_WORKTREE_ROOT",
    "save": "all"
  }
]
```

Open the task picker with `task: spawn` and choose `Ember: Reload` to run it manually.

## Key binding

Add this object to the array in your `keymap.json` file. It binds `Ctrl+Alt+R` while a workspace is active:

```json
[
  {
    "context": "Workspace",
    "bindings": {
      "ctrl-alt-r": ["task::Spawn", { "task_name": "Ember: Reload" }]
    }
  }
]
```

On Windows, open the keymap file with Zed's `zed: open keymap file` command. It is usually under `%AppData%\Roaming\Zed\keymap.json`. For the task and keymap formats, see the [Zed tasks](https://zed.dev/docs/tasks) and [key bindings](https://zed.dev/docs/key-bindings) docs.
