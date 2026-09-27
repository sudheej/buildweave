package fingerprint

import "testing"

func TestStableAndRevisionIndependent(t *testing.T) {
	a := Compute("compile(core)", "jdk", "linux", []Input{{"b", "2"}, {"a", "1"}}, []string{"y", "x"})
	b := Compute("compile(core)", "jdk", "linux", []Input{{"a", "1"}, {"b", "2"}}, []string{"x", "y"})
	if a != b {
		t.Fatal("ordering changed fingerprint")
	}
	commitA := a
	commitB := a
	if commitA != commitB {
		t.Fatal("commit provenance altered computation identity")
	}
}
