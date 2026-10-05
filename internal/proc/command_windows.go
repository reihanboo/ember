package proc

import (
	"context"
	"os/exec"
	"syscall"
)

func Command(command, cwd string, env map[string]string) *exec.Cmd {
	return commandWithContext(context.Background(), command, cwd, env)
}

func commandWithContext(ctx context.Context, command, cwd string, env map[string]string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: "cmd.exe /C " + command}
	cmd.Dir = cwd
	cmd.Env = commandEnv(env)
	return cmd
}
