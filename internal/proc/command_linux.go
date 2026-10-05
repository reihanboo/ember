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
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Dir = cwd
	cmd.Env = commandEnv(env)
	return cmd
}
