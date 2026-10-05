# Neovim

This recipe uses `vim.system`, available in Neovim 0.10 and newer. Start `ember run` for the project first, make sure `ember` is on Neovim's `PATH`, and open Neovim with its working directory inside the project.

## Manual reload

Put this in your Neovim config. Press `<leader>rr` to request `ember reload`; errors from the command are shown as a notification.

```lua
local function ember_reload()
  vim.system(
    { "ember", "reload" },
    { cwd = vim.fn.getcwd(), text = true },
    function(result)
      if result.code ~= 0 then
        vim.schedule(function()
          local message = result.stderr
          if not message or message == "" then
            message = ("ember reload exited with code %d"):format(result.code)
          end
          vim.notify(message, vim.log.levels.ERROR)
        end)
      end
    end
  )
end

vim.keymap.set("n", "<leader>rr", ember_reload, { desc = "Ember: reload" })
```

## Optional save hook

If you want Neovim to trigger reloads after saves, add this autocmd:

```lua
local ember_group = vim.api.nvim_create_augroup("EmberReload", { clear = true })

vim.api.nvim_create_autocmd("BufWritePost", {
  group = ember_group,
  pattern = { "*.c", "*.cc", "*.cpp", "*.h", "*.hpp" },
  callback = ember_reload,
})
```

Use the hook only when Ember's watcher isn't already watching these files; otherwise a save can trigger both the autocmd and the watcher.