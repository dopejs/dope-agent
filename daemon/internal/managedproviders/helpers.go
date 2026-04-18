package managedproviders

import (
	"path/filepath"
	"strings"
)

func baseName(path string) string {
	if trimmed := strings.TrimSpace(path); trimmed != "" {
		return filepath.Base(trimmed)
	}
	return ""
}

func filepathJoin(parts ...string) string {
	return filepath.Join(parts...)
}
