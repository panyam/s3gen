package s3gen

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CopyStatic copies every StaticFolders entry into OutputDir at its URL path, so a built site
// can be served by any static host. Serve already serves these folders from where they are;
// this is for the files a build leaves behind.
//
// Each destination is emptied first, so files deleted from a static folder disappear from the
// output too. A folder mapped to "/" is copied over the output root without emptying it, since
// the root also holds the generated pages. A folder that doesn't exist is skipped.
//
// Rebuild calls this on a full build (a nil resource list).
func (s *Site) CopyStatic() error {
	if s.OutputDir == "" {
		return fmt.Errorf("CopyStatic needs an OutputDir")
	}
	for i := 0; i+1 < len(s.StaticFolders); i += 2 {
		urlPath, folder := s.StaticFolders[i], s.StaticFolders[i+1]
		info, err := os.Stat(folder)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("static folder %s is not a directory", folder)
		}
		rel := strings.Trim(filepath.FromSlash(urlPath), string(filepath.Separator))
		dest := filepath.Join(s.OutputDir, rel)
		if inside(dest, folder) || inside(folder, dest) {
			return fmt.Errorf("static folder %s and its output %s overlap", folder, dest)
		}
		if rel != "" {
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
		}
		if err := copyTree(folder, dest); err != nil {
			return fmt.Errorf("copying static folder %s to %s: %w", folder, dest, err)
		}
	}
	return nil
}

// inside reports whether path is dir or somewhere under it, comparing absolute paths.
func inside(path, dir string) bool {
	p, err1 := filepath.Abs(path)
	d, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil {
		return true // can't tell, so don't risk deleting anything
	}
	rel, err := filepath.Rel(d, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
