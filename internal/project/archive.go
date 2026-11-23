package project

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// WriteArchive exports saved source and assets directly from the project.
// Hidden files, dependency trees and caches are not design deliverables.
// Refuse links and special files rather than following or silently losing them.
func (m *Manager) WriteArchive(ctx context.Context, w io.Writer, slug string) error {
	root, err := m.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	zw := zip.NewWriter(w)
	var total int64
	var count int
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "__pycache__" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() || strings.ContainsAny(name, "\\:") {
			return fmt.Errorf("cannot export non-portable file %q", name)
		}
		f, err := root.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		count++
		if !st.Mode().IsRegular() || st.Size() > 1<<30-total || count > 10000 {
			return fmt.Errorf("export requires regular files totaling at most 1 GiB and 10,000 files")
		}
		total += st.Size()
		out, err := zw.CreateHeader(&zip.FileHeader{Name: slug + "/" + name, Method: zip.Deflate, Modified: st.ModTime()})
		if err != nil {
			return err
		}
		n, err := io.Copy(out, io.LimitReader(f, st.Size()+1))
		if err != nil {
			return err
		}
		if n != st.Size() {
			return fmt.Errorf("%q changed during export; try again", name)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return zw.Close()
}
