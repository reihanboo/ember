package proc

import (
	"os"
	"runtime"
	"strings"
)

func commandEnv(overrides map[string]string) []string {
	environment := os.Environ()
	positions := make(map[string]int, len(environment)+len(overrides))
	for index, entry := range environment {
		separator := strings.IndexByte(entry, '=')
		if separator <= 0 {
			continue
		}
		positions[environmentKey(entry[:separator])] = index
	}

	for key, value := range overrides {
		entry := key + "=" + value
		lookupKey := environmentKey(key)
		if index, exists := positions[lookupKey]; exists {
			environment[index] = entry
			continue
		}
		positions[lookupKey] = len(environment)
		environment = append(environment, entry)
	}
	return environment
}

func environmentKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
