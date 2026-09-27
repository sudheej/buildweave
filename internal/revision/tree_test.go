package revision

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTreeDigestDeterministicAndIgnoresOutputs(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "b"), []byte("2"), 0644)
	os.WriteFile(filepath.Join(d, "a"), []byte("1"), 0644)
	x, e := TreeDigest(d)
	if e != nil {
		t.Fatal(e)
	}
	os.Mkdir(filepath.Join(d, "build"), 0755)
	os.WriteFile(filepath.Join(d, "build", "noise"), []byte("x"), 0644)
	y, _ := TreeDigest(d)
	if x != y {
		t.Fatal("build output affected source tree")
	}
	os.WriteFile(filepath.Join(d, "a"), []byte("changed"), 0644)
	z, _ := TreeDigest(d)
	if z == x {
		t.Fatal("source change not detected")
	}
}
