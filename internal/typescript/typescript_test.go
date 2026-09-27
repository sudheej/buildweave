package typescript

import (
	"os"
	"path/filepath"
	"testing"
)

func sig(t *testing.T, s string) string {
	t.Helper()
	d := t.TempDir()
	os.Mkdir(filepath.Join(d, "src"), 0755)
	os.WriteFile(filepath.Join(d, "src", "x.ts"), []byte(s), 0644)
	x, _, e := ExportSignature(filepath.Join(d, "src"))
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func TestSignatureSemanticBoundary(t *testing.T) {
	a := sig(t, "export function calculate(x: number): number { return x * 2; }")
	b := sig(t, "export function calculate(x: number): number { const y=x*3; return y; }")
	if a != b {
		t.Fatal("implementation changed declaration")
	}
	c := sig(t, "export function calculate(x: string): number { return x.length; }")
	if a == c {
		t.Fatal("public type change missed")
	}
}
