package scheduler

import (
	"context"
	"errors"
	"sync"
)

var ErrNondeterministic = errors.New("nondeterministic output for computation fingerprint")

type Metrics struct{ Executions, Joins, Nondeterministic int64 }
type call struct {
	done        chan struct{}
	output      string
	err         error
	cancel      context.CancelFunc
	subscribers map[string]context.CancelFunc
}
type Scheduler struct {
	mu      sync.Mutex
	running map[string]*call
	known   map[string]string
	metrics Metrics
}

func New() *Scheduler { return &Scheduler{running: map[string]*call{}, known: map[string]string{}} }

// Execute joins concurrent requests for the same fingerprint. An individual caller
// leaving only removes its subscription; shared work is cancelled only at zero subscribers.
func (s *Scheduler) Execute(ctx context.Context, fp, subscriber string, fn func(context.Context) (string, error)) (string, bool, error) {
	s.mu.Lock()
	if c, ok := s.running[fp]; ok {
		s.metrics.Joins++
		c.subscribers[subscriber] = nil
		s.mu.Unlock()
		select {
		case <-c.done:
			return c.output, true, c.err
		case <-ctx.Done():
			s.Remove(fp, subscriber)
			return "", true, ctx.Err()
		}
	}
	workCtx, cancel := context.WithCancel(context.Background())
	c := &call{done: make(chan struct{}), cancel: cancel, subscribers: map[string]context.CancelFunc{subscriber: cancel}}
	s.running[fp] = c
	s.metrics.Executions++
	s.mu.Unlock()
	out, err := fn(workCtx)
	s.mu.Lock()
	if old, ok := s.known[fp]; ok && err == nil && old != out {
		err = ErrNondeterministic
		s.metrics.Nondeterministic++
	} else if err == nil {
		s.known[fp] = out
	}
	c.output, c.err = out, err
	delete(s.running, fp)
	close(c.done)
	s.mu.Unlock()
	return out, false, err
}
func (s *Scheduler) Remove(fp, subscriber string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.running[fp]
	if !ok {
		return
	}
	delete(c.subscribers, subscriber)
	if len(c.subscribers) == 0 {
		c.cancel()
	}
}
func (s *Scheduler) Subscribers(fp string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.running[fp]
	if c == nil {
		return nil
	}
	r := make([]string, 0, len(c.subscribers))
	for x := range c.subscribers {
		r = append(r, x)
	}
	return r
}
func (s *Scheduler) Metrics() Metrics { s.mu.Lock(); defer s.mu.Unlock(); return s.metrics }
func (s *Scheduler) CheckIndependent(fp, out string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.known[fp]; ok && old != out {
		s.metrics.Nondeterministic++
		return ErrNondeterministic
	}
	s.known[fp] = out
	return nil
}
