package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ignored = map[string]bool{".git": true, ".gradle": true, "build": true, "node_modules": true, "dist": true, ".buildweave": true}

func TreeDigest(root string) (string, error) {
	var names []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() && ignored[d.Name()] {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		f, err := os.Open(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d:%s\x00", len(name), name)
		if _, err = io.Copy(h, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func CopyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return os.MkdirAll(dst, 0755)
		}
		if d.IsDir() && ignored[d.Name()] {
			return filepath.SkipDir
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, 0644)
	})
}

func SafeRelative(name string) bool {
	return name != "" && !filepath.IsAbs(name) && !strings.HasPrefix(filepath.Clean(name), "..")
}
