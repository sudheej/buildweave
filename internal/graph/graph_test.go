package graph

import (
	"reflect"
	"testing"
)

func TestReverseInvalidationAndEarlyCutoff(t *testing.T) {
	g := New()
	g.Set(Node{ID: "abi", SemanticDigest: "same", State: Completed})
	g.Set(Node{ID: "service", Dependencies: []string{"abi"}, State: Completed})
	g.Set(Node{ID: "app", Dependencies: []string{"service"}, State: Completed})
	if got := g.Invalidate("abi"); !reflect.DeepEqual(got, []string{"app", "service"}) {
		t.Fatalf("%v", got)
	}
	g.Set(Node{ID: "service", Dependencies: []string{"abi"}, State: Completed})
	got := g.UpdateSemantic("abi", "new-bytecode", "same")
	if !reflect.DeepEqual(got, []string{"service"}) {
		t.Fatalf("not retained: %v", got)
	}
}
