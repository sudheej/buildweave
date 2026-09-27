package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/buildweave/buildweave/internal/model"
)

func find(r model.GenerationReport, id string) model.Explanation {
	for _, e := range r.Explanations {
		if e.QueryID == id {
			return e
		}
	}
	return model.Explanation{}
}
func TestJavaTypeScriptCarryoverSealAndShadow(t *testing.T) {
	if testing.Short() {
		t.Skip("uses javac and javap")
	}
	m, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	demo := filepath.Join("..", "..", "demo")
	b, _, e := m.OpenBranch("base", "test", demo)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if _, e = m.Verify(ctx, b.ID, 0, "agent-fast"); e != nil {
		t.Fatal(e)
	}
	g1, e := m.ApplyDelta(b.ID, 0, 1, []Change{{Path: "java/core/src/main/java/dev/buildweave/core/Calculator.java", Content: "package dev.buildweave.core; public class Calculator { public int calculate(int x){ return x*3; } }"}, {Path: "typescript/packages/models/src/index.ts", Content: "export function calculate(x: number): number { const result=x*3; return result; }"}})
	if e != nil {
		t.Fatal(e)
	}
	r1, e := m.Verify(ctx, b.ID, 1, "agent-fast")
	if e != nil {
		t.Fatal(e)
	}
	if x := find(r1, "java.compile(service)"); x.Decision != model.DecisionRetained {
		t.Fatalf("service compile=%s %s", x.Decision, x.Reason)
	}
	if x := find(r1, "typescript.typecheck(service)"); x.Decision != model.DecisionRetained {
		t.Fatalf("service typecheck=%s %s", x.Decision, x.Reason)
	}
	g2, e := m.ApplyDelta(b.ID, 1, 2, []Change{{Path: "java/core/src/main/java/dev/buildweave/core/Calculator.java", Content: "package dev.buildweave.core; public class Calculator { public int calculate(int x){ return x*3; } public long calculate(long x){return x*3;} }"}, {Path: "typescript/packages/models/src/index.ts", Content: "export function calculate(x: number, factor?: number): number { return x*(factor ?? 3); }"}})
	if e != nil {
		t.Fatal(e)
	}
	r2, e := m.Verify(ctx, b.ID, 2, "agent-fast")
	if e != nil {
		t.Fatal(e)
	}
	if x := find(r2, "java.compile(service)"); x.Decision == model.DecisionRetained || x.Decision == model.DecisionReused {
		t.Fatalf("Java API change did not propagate: %s", x.Decision)
	}
	if x := find(r2, "typescript.typecheck(service)"); x.Decision == model.DecisionRetained || x.Decision == model.DecisionReused {
		t.Fatalf("TS API change did not propagate: %s", x.Decision)
	}
	if e = m.Seal(b.ID, 2, "commit", "wrong"); e == nil {
		t.Fatal("sealed wrong tree")
	}
	if e = m.Seal(b.ID, 2, "commit", g2.SourceTreeDigest); e != nil {
		t.Fatal(e)
	}
	before := m.Metrics.Get("queries_recomputed_total")
	if _, e = m.Verify(ctx, b.ID, 2, "pr-required"); e != nil {
		t.Fatal(e)
	}
	if after := m.Metrics.Get("queries_recomputed_total"); after != before {
		t.Fatalf("commit identity caused %d recomputations", after-before)
	}
	ok, _, e := m.Shadow(ctx, b.ID, 2)
	if e != nil || !ok {
		t.Fatalf("shadow ok=%v err=%v", ok, e)
	}
	_ = g1
}
