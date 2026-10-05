package watcher

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/reihanboo/ember/internal/watcher/glob"
)

type Filter struct {
	root    string
	include []string
	ignore  []string
}

func NewFilter(root string, include, ignore []string) (*Filter, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root %q: %w", root, err)
	}
	return &Filter{
		root:    filepath.Clean(absoluteRoot),
		include: append([]string(nil), include...),
		ignore:  append([]string(nil), ignore...),
	}, nil
}

func (f *Filter) Allow(absPath string) bool {
	if !filepath.IsAbs(absPath) {
		return false
	}

	relative, err := filepath.Rel(f.root, filepath.Clean(absPath))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	path := filepath.ToSlash(relative)
	if path == ".ember" || glob.Match(".ember/**", path) {
		return false
	}
	for _, pattern := range f.ignore {
		if glob.Match(pattern, path) {
			return false
		}
	}
	if len(f.include) == 0 {
		return true
	}
	for _, pattern := range f.include {
		if glob.Match(pattern, path) {
			return true
		}
	}
	return false
}
