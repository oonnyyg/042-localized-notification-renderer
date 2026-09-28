package main

import "path/filepath"

// filepathDir returns the containing directory of a path, or "." for bare
// file names, so temp files are created on the same volume as the target.
func filepathDir(path string) string {
	dir := filepath.Dir(filepath.Clean(path))
	if dir == "" {
		return "."
	}
	return dir
}
