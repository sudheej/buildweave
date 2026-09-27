package java

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func compileText(t *testing.T, text string) Result {
	t.Helper()
	d := t.TempDir()
	src := filepath.Join(d, "src")
	os.Mkdir(src, 0755)
	os.WriteFile(filepath.Join(src, "Calc.java"), []byte(text), 0644)
	r, e := Compile(context.Background(), src, filepath.Join(d, "out"), nil)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestABIImplementationVsAPI(t *testing.T) {
	a := compileText(t, "public class Calc { public int calculate(int x){return x*2;} }")
	b := compileText(t, "public class Calc { public int calculate(int x){return x*3;} }")
	if a.ContentDigest == b.ContentDigest {
		t.Fatal("bytecode did not change")
	}
	if a.ABIDigest != b.ABIDigest {
		t.Fatal("implementation altered ABI")
	}
	c := compileText(t, "public class Calc { public long calculate(long x){return x*3;} }")
	if a.ABIDigest == c.ABIDigest {
		t.Fatal("API change missed")
	}
}
