package graph

import (
	"errors"
	"sort"
	"sync"
)

type State string

const (
	Missing     State = "missing"
	Running     State = "running"
	Completed   State = "completed"
	Retained    State = "retained"
	Invalidated State = "invalidated"
	Failed      State = "failed"
)

type Node struct {
	ID                           string
	Dependencies                 []string
	OutputDigest, SemanticDigest string
	State                        State
}
type Graph struct {
	mu      sync.RWMutex
	nodes   map[string]*Node
	reverse map[string]map[string]bool
}

func New() *Graph { return &Graph{nodes: map[string]*Node{}, reverse: map[string]map[string]bool{}} }
func (g *Graph) Set(n Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, d := range n.Dependencies {
		if d == n.ID {
			return errors.New("self dependency")
		}
		if g.reverse[d] == nil {
			g.reverse[d] = map[string]bool{}
		}
		g.reverse[d][n.ID] = true
	}
	copy := n
	g.nodes[n.ID] = &copy
	return nil
}
func (g *Graph) Get(id string) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	if !ok {
		return Node{}, false
	}
	return *n, true
}
func (g *Graph) Invalidate(id string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(x string) {
		for d := range g.reverse[x] {
			if seen[d] {
				continue
			}
			seen[d] = true
			if n := g.nodes[d]; n != nil {
				n.State = Invalidated
			}
			out = append(out, d)
			walk(d)
		}
	}
	walk(id)
	sort.Strings(out)
	return out
}

// UpdateSemantic performs early cutoff: downstream nodes remain retained if the explicit semantic value is equal.
func (g *Graph) UpdateSemantic(id, output, semantic string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := g.nodes[id]
	if n == nil {
		return nil
	}
	same := n.SemanticDigest == semantic && semantic != ""
	n.OutputDigest = output
	n.SemanticDigest = semantic
	n.State = Completed
	if same {
		var retained []string
		for d := range g.reverse[id] {
			if dn := g.nodes[d]; dn != nil {
				dn.State = Retained
				retained = append(retained, d)
			}
		}
		sort.Strings(retained)
		return retained
	}
	var invalid []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(x string) {
		for d := range g.reverse[x] {
			if seen[d] {
				continue
			}
			seen[d] = true
			if dn := g.nodes[d]; dn != nil {
				dn.State = Invalidated
			}
			invalid = append(invalid, d)
			walk(d)
		}
	}
	walk(id)
	sort.Strings(invalid)
	return invalid
}
