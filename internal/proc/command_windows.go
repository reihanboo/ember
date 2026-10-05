package proc

import (
	"os/exec"
	"syscall"
)

func Command(command, cwd string, env map[string]string) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: "cmd.exe /C " + command}
	cmd.Dir = cwd
	cmd.Env = commandEnv(env)
	return cmd
}
