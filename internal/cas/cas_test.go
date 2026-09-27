package cas

import (
	"context"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	s, e := NewFile(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	d, e := s.Put(context.Background(), []byte("hello"))
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.Get(context.Background(), d)
	if e != nil || string(b) != "hello" {
		t.Fatalf("%q %v", b, e)
	}
	ok, e := s.Exists(context.Background(), d)
	if e != nil || !ok {
		t.Fatal("missing")
	}
}
