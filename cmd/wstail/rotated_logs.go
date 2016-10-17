package main

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// newRotatedLogs creates a new instance of rotatedLogs from directory dir
func newRotatedLogs(dir string, names []string) rotatedLogs {
	var logs rotatedLogs
	for _, name := range names {
		baseName := filepath.Base(name)
		if baseName == rotatedLogUncompressed {
			logs.Main = filepath.Join(dir, name)
		}
		if strings.HasPrefix(baseName, "messages.") && strings.HasSuffix(baseName, ".gz") {
			logs.Compressed = append(logs.Compressed, filepath.Join(dir, name))
		}
	}
	sort.Sort(naturalSortOrder(logs.Compressed))
	return logs
}

// rotatedLogs defines a set of files managed by savelog command
type rotatedLogs struct {
	// Main defines a completed not yet compressed log file
	Main string
	// Compressed lists all compressed log files
	Compressed []string
}

// naturalSortOrder defines a sort helper to sort filenames in
// the natural order of their index. The filenames are assumed to
// be of the form:
//
// <name>.<index>.<suffix>
type naturalSortOrder []string

func (r naturalSortOrder) Len() int {
	return len(r)
}

func (r naturalSortOrder) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r naturalSortOrder) Less(i, j int) bool {
	parts := strings.SplitN(filepath.Base(r[i]), ".", 3)
	if len(parts) != 3 {
		return false
	}
	index := parts[1]
	if len(index) > 0 {
		i, _ = strconv.Atoi(index)
	}
	parts = strings.SplitN(filepath.Base(r[j]), ".", 3)
	if len(parts) != 3 {
		return false
	}
	index = parts[1]
	if len(index) > 0 {
		j, _ = strconv.Atoi(index)
	}

	// From old to new
	return i > j
}
