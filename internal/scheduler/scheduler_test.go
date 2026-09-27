package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSingleflightSubscribers(t *testing.T) {
	s := New()
	started := make(chan struct{})
	release := make(chan struct{})
	var runs atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)
	fn := func(context.Context) (string, error) {
		if runs.Add(1) == 1 {
			close(started)
		}
		<-release
		return "out", nil
	}
	go func() { defer wg.Done(); s.Execute(context.Background(), "fp", "g1", fn) }()
	<-started
	go func() { defer wg.Done(); s.Execute(context.Background(), "fp", "g2", fn) }()
	for len(s.Subscribers("fp")) != 2 {
	}
	s.Remove("fp", "g1")
	close(release)
	wg.Wait()
	if runs.Load() != 1 || s.Metrics().Joins != 1 {
		t.Fatalf("runs=%d joins=%d", runs.Load(), s.Metrics().Joins)
	}
}
func TestNondeterminism(t *testing.T) {
	s := New()
	if e := s.CheckIndependent("fp", "a"); e != nil {
		t.Fatal(e)
	}
	if e := s.CheckIndependent("fp", "b"); !errors.Is(e, ErrNondeterministic) {
		t.Fatalf("%v", e)
	}
}
