package metadata

import (
	"errors"
	"github.com/buildweave/buildweave/internal/model"
	"testing"
	"time"
)

func TestOptimisticGenerationAndSeal(t *testing.T) {
	s, e := Open(t.TempDir() + "/m.json")
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	b := model.BranchSession{ID: "b", LatestGeneration: 0, CreatedAt: now}
	g := model.RevisionGeneration{BranchID: "b", SourceTreeDigest: "sha256:a"}
	if e = s.OpenBranch(b, g); e != nil {
		t.Fatal(e)
	}
	p := 0
	g1 := model.RevisionGeneration{BranchID: "b", Generation: 1, ParentGeneration: &p, SourceTreeDigest: "sha256:b"}
	if e = s.AddGeneration("b", 0, g1); e != nil {
		t.Fatal(e)
	}
	if e = s.AddGeneration("b", 0, g1); !errors.Is(e, ErrStaleGeneration) {
		t.Fatalf("wanted stale: %v", e)
	}
	if e = s.Seal("b", 1, "abc", "wrong"); e == nil {
		t.Fatal("wrong tree sealed")
	}
	if e = s.Seal("b", 1, "abc", "sha256:b"); e != nil {
		t.Fatal(e)
	}
}
