package proc

import "os/exec"

func Command(command, cwd string, env map[string]string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = cwd
	cmd.Env = commandEnv(env)
	return cmd
}
