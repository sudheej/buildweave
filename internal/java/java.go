package java

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/buildweave/buildweave/internal/fingerprint"
)

type Result struct {
	ContentDigest, ABIDigest string
	Classes                  []byte
}

func Compile(ctx context.Context, sourceRoot, outputDir string, classpath []string) (Result, error) {
	var sources []string
	err := filepath.WalkDir(sourceRoot, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && strings.HasSuffix(p, ".java") {
			sources = append(sources, p)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	sort.Strings(sources)
	if len(sources) == 0 {
		return Result{}, fmt.Errorf("no Java sources in %s", sourceRoot)
	}
	if err = os.RemoveAll(outputDir); err != nil {
		return Result{}, err
	}
	if err = os.MkdirAll(outputDir, 0755); err != nil {
		return Result{}, err
	}
	args := []string{"-g:none", "-d", outputDir}
	if len(classpath) > 0 {
		args = append(args, "-classpath", strings.Join(classpath, string(os.PathListSeparator)))
	}
	args = append(args, sources...)
	cmd := exec.CommandContext(ctx, "javac", args...)
	if b, e := cmd.CombinedOutput(); e != nil {
		return Result{}, fmt.Errorf("javac: %w\n%s", e, b)
	}
	return Inspect(ctx, outputDir)
}

func Inspect(ctx context.Context, classesDir string) (Result, error) {
	var classes []string
	var combined bytes.Buffer
	err := filepath.WalkDir(classesDir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(p, ".class") {
			return nil
		}
		rel, _ := filepath.Rel(classesDir, p)
		classes = append(classes, strings.TrimSuffix(filepath.ToSlash(rel), ".class"))
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	sort.Strings(classes)
	for _, c := range classes {
		b, err := os.ReadFile(filepath.Join(classesDir, filepath.FromSlash(c)+".class"))
		if err != nil {
			return Result{}, err
		}
		fmt.Fprintf(&combined, "%s\x00", c)
		combined.Write(b)
	}
	var abi bytes.Buffer
	for _, c := range classes {
		cmd := exec.CommandContext(ctx, "javap", "-classpath", classesDir, "-public", "-s", strings.ReplaceAll(c, "/", "."))
		out, err := cmd.Output()
		if err != nil {
			return Result{}, fmt.Errorf("javap %s: %w", c, err)
		}
		abi.WriteString(normalizeJavap(string(out)))
		abi.WriteByte(0)
	}
	return Result{ContentDigest: fingerprint.Bytes(combined.Bytes()), ABIDigest: fingerprint.Bytes(abi.Bytes()), Classes: combined.Bytes()}, nil
}
func normalizeJavap(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Compiled from ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
