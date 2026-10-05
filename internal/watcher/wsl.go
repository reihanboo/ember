package watcher

import "strings"

func isWSLMount(root string) bool {
	root = strings.ReplaceAll(root, `\`, "/")
	return strings.HasPrefix(root, "/mnt/")
}
