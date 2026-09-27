package cas

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/buildweave/buildweave/internal/fingerprint"
)

var ErrNotFound = errors.New("CAS object not found")

type Store interface {
	Put(context.Context, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
	Exists(context.Context, string) (bool, error)
	Stats() Snapshot
}
type Stats struct{ Puts, Hits, Misses atomic.Int64 }
type Snapshot struct{ Puts, Hits, Misses int64 }

func (s *Stats) Snapshot() Snapshot { return Snapshot{s.Puts.Load(), s.Hits.Load(), s.Misses.Load()} }

type FileStore struct {
	Root  string
	stats Stats
}

func NewFile(root string) (*FileStore, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &FileStore{Root: root}, nil
}
func (s *FileStore) path(d string) (string, error) {
	if !strings.HasPrefix(d, "sha256:") || len(d) != 71 {
		return "", errors.New("invalid digest")
	}
	return filepath.Join(s.Root, d[7:9], d[9:]), nil
}
func (s *FileStore) Put(ctx context.Context, b []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	d := fingerprint.Bytes(b)
	p, _ := s.path(d)
	if _, err := os.Stat(p); err == nil {
		return d, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "put-")
	if err != nil {
		return "", err
	}
	n, err := tmp.Write(b)
	if err == nil && n != len(b) {
		err = errors.New("short CAS write")
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), p)
	} else {
		os.Remove(tmp.Name())
	}
	if err == nil {
		s.stats.Puts.Add(1)
	}
	return d, err
}
func (s *FileStore) Get(ctx context.Context, d string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := s.path(d)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		s.stats.Misses.Add(1)
		return nil, ErrNotFound
	}
	if err == nil {
		s.stats.Hits.Add(1)
	}
	return b, err
}
func (s *FileStore) Exists(ctx context.Context, d string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	p, err := s.path(d)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		s.stats.Misses.Add(1)
		return false, nil
	}
	if err == nil {
		s.stats.Hits.Add(1)
	}
	return err == nil, err
}
func (s *FileStore) Stats() Snapshot { return s.stats.Snapshot() }
