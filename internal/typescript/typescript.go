package typescript

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/buildweave/buildweave/internal/fingerprint"
)

var exported = regexp.MustCompile(`^\s*export\s+(?:declare\s+)?(?:async\s+)?(function|class|interface|type|const|let|var|enum)\s+`)

// ExportSignature creates a deterministic .d.ts-equivalent digest for the deliberately
// small educational workspace. If tsc is available, DeclarationDigest should be preferred.
func ExportSignature(sourceRoot string) (string, string, error) {
	var files []string
	err := filepath.WalkDir(sourceRoot, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && strings.HasSuffix(p, ".ts") && !strings.HasSuffix(p, ".d.ts") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	sort.Strings(files)
	var sig bytes.Buffer
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", "", err
		}
		scan := bufio.NewScanner(bytes.NewReader(b))
		var collecting bool
		depth := 0
		for scan.Scan() {
			line := scan.Text()
			if exported.MatchString(line) {
				collecting = true
				depth = strings.Count(line, "{") - strings.Count(line, "}")
				signature := line
				if i := strings.Index(signature, "{"); i >= 0 {
					signature = signature[:i]
				}
				if kind := exported.FindStringSubmatch(line)[1]; kind == "function" {
					if i := strings.Index(line, "{"); i >= 0 {
						signature = line[:i]
					}
					if !strings.HasSuffix(strings.TrimSpace(signature), ";") {
						signature = strings.TrimSpace(signature) + ";"
					}
				}
				sig.WriteString(strings.TrimSpace(signature))
				sig.WriteByte('\n')
				if depth <= 0 {
					collecting = false
				}
			}
			if collecting {
				depth += strings.Count(line, "{") - strings.Count(line, "}")
				if depth <= 0 {
					collecting = false
				}
			}
		}
		if err := scan.Err(); err != nil {
			return "", "", err
		}
	}
	return fingerprint.Bytes(sig.Bytes()), sig.String(), nil
}

func DeclarationDigest(project, out string) (string, error) {
	if _, err := exec.LookPath("tsc"); err != nil {
		return "", err
	}
	if err := os.RemoveAll(out); err != nil {
		return "", err
	}
	cmd := exec.Command("tsc", "--project", project, "--declaration", "--emitDeclarationOnly", "--outDir", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("tsc: %w\n%s", err, b)
	}
	var all bytes.Buffer
	var names []string
	filepath.WalkDir(out, func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() && strings.HasSuffix(p, ".d.ts") {
			names = append(names, p)
		}
		return e
	})
	sort.Strings(names)
	for _, p := range names {
		b, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(out, p)
		all.WriteString(filepath.ToSlash(rel))
		all.WriteByte(0)
		all.Write(b)
	}
	return fingerprint.Bytes(all.Bytes()), nil
}
