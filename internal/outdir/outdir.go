package outdir

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

var sequence atomic.Uint64

func Next() string {
	name := "app-" + strconv.FormatUint(sequence.Add(1), 10) + executableSuffix()
	return filepath.Join(".ember", "bin", name)
}

func Substitute(cmd, out string) string {
	return strings.ReplaceAll(cmd, "{out}", out)
}
