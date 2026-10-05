# Lite XL

Save this as `ember.lua` in Lite XL's plugins directory. It registers `ember:reload` and binds it to `Ctrl+Alt+R`.

```lua
local core = require "core"
local command = require "core.command"
local keymap = require "core.keymap"
local process = require "process"

local reload_process

command.add(nil, {
  ["ember:reload"] = function()
    if reload_process and reload_process:running() then return end
    local project = core.root_project()
    if not project then return end
    reload_process = process.start({ "ember", "reload" }, { cwd = project.path })
    if not reload_process then return end
    core.add_thread(function()
      while reload_process and reload_process:running() do
        coroutine.yield(0.1)
      end
      reload_process = nil
    end)
  end
})

keymap.add({ ["ctrl+alt+r"] = "ember:reload" })
```

Ember must already be running in the project for `ember reload` to reach its supervisor.
