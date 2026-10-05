package proc

import (
	"context"
	"os/exec"
)

func Command(command, cwd string, env map[string]string) *exec.Cmd {
	return commandWithContext(context.Background(), command, cwd, env)
}

func commandWithContext(ctx context.Context, command, cwd string, env map[string]string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = cwd
	cmd.Env = commandEnv(env)
	return cmd
}
