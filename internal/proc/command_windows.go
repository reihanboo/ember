package proc

import (
	"context"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func Command(command, cwd string, env map[string]string) *exec.Cmd {
	return commandWithContext(context.Background(), command, cwd, env)
}

func commandWithContext(ctx context.Context, command, cwd string, env map[string]string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       "cmd.exe /C " + command,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Dir = cwd
	cmd.Env = commandEnv(env)
	return cmd
}
