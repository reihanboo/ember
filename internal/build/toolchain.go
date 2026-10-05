package build

import "strings"

func commandHint(command string, lookPath func(string) (string, error), windows bool) string {
	token := firstCommandToken(command)
	if token == "" || shellBuiltin(token, windows) {
		return ""
	}
	if lookPath != nil {
		if _, err := lookPath(token); err == nil {
			return ""
		}
	}
	if windows && (strings.EqualFold(token, "cl") || strings.EqualFold(token, "cl.exe")) {
		return "cl not found on PATH; run ember from a Developer PowerShell"
	}
	return "command not found: " + token
}

func firstCommandToken(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if command[0] == '\'' || command[0] == '"' {
		quote := command[0]
		if end := strings.IndexByte(command[1:], quote); end >= 0 {
			return command[1 : end+1]
		}
		return strings.Trim(command, string(quote))
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func shellBuiltin(command string, windows bool) bool {
	if windows {
		switch strings.ToLower(command) {
		case "assoc", "break", "call", "cd", "chdir", "cls", "color", "copy", "date", "del", "dir", "echo", "endlocal", "erase", "exit", "for", "ftype", "goto", "if", "md", "mkdir", "mklink", "move", "path", "pause", "popd", "prompt", "pushd", "rd", "rem", "ren", "rename", "rmdir", "set", "setlocal", "shift", "start", "time", "title", "type", "ver", "verify", "vol":
			return true
		}
		return false
	}
	switch command {
	case "!", ".", ":", "alias", "bg", "break", "builtin", "case", "cd", "command", "continue", "do", "done", "echo", "elif", "else", "esac", "eval", "exec", "exit", "export", "false", "fc", "fg", "fi", "for", "getopts", "hash", "if", "jobs", "kill", "local", "printf", "pwd", "read", "readonly", "return", "select", "set", "shift", "source", "test", "then", "time", "times", "trap", "true", "type", "typeset", "ulimit", "umask", "unalias", "unset", "until", "wait", "while":
		return true
	}
	return false
}
