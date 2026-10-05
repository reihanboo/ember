package outdir

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Prune(keep ...string) error {
	directory, err := filepath.Abs(filepath.Join(".ember", "bin"))
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read output directory %q: %w", directory, err)
	}

	preserved := make(map[string]struct{}, len(keep)*3)
	for _, path := range keep {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve kept output %q: %w", path, err)
		}
		absolute = filepath.Clean(absolute)
		preserved[absolute] = struct{}{}
		stem := strings.TrimSuffix(absolute, filepath.Ext(absolute))
		preserved[stem+".pdb"] = struct{}{}
		preserved[stem+".ilk"] = struct{}{}
	}

	var pruneErr error
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if _, ok := preserved[filepath.Clean(path)]; ok {
			continue
		}
		if err := removePrunedPath(path); err != nil {
			pruneErr = errors.Join(pruneErr, err)
		}
		if entry.IsDir() || !isOutputName(entry.Name()) {
			continue
		}
		stem := strings.TrimSuffix(path, filepath.Ext(path))
		for _, extension := range []string{".pdb", ".ilk"} {
			sidecar := stem + extension
			if _, ok := preserved[filepath.Clean(sidecar)]; ok {
				continue
			}
			if err := removePrunedPath(sidecar); err != nil {
				pruneErr = errors.Join(pruneErr, err)
			}
		}
	}
	return pruneErr
}

func removePrunedPath(path string) error {
	if err := os.RemoveAll(path); err != nil {
		if isLockedError(err) {
			log.Printf("outdir: could not remove locked output %q: %v", path, err)
			return nil
		}
		return fmt.Errorf("remove stale output %q: %w", path, err)
	}
	return nil
}

func isOutputName(name string) bool {
	if !strings.HasPrefix(name, "app-") {
		return false
	}
	extension := filepath.Ext(name)
	return extension == "" || strings.EqualFold(extension, ".exe")
}
