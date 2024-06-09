package core

import (
	"path/filepath"
	"strings"
)

func (s *Site) PathToDir(path string) (string, bool, error) {
	startingDir := filepath.SplitList(s.ContentRoot)
	minPartsNeeded := len(startingDir)
	for _, p := range strings.Split(path, "/") {
		if p == "." {
			// do nothing
		} else if p == ".." {
			if len(startingDir) == minPartsNeeded {
				// cannot go any further so return error
				return "", false, ErrContentPathInvalid
			} else {
				startingDir = startingDir[:len(startingDir)-1]
			}
		} else {
			// append to it
			startingDir = append(startingDir, p)
		}
	}
	return filepath.Join(startingDir...), true, nil
}
