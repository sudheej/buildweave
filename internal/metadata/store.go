package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/buildweave/buildweave/internal/model"
)

var ErrStaleGeneration = errors.New("stale expected generation")
var ErrNotFound = errors.New("metadata record not found")

type Data struct {
	Branches    map[string]model.BranchSession              `json:"branches"`
	Generations map[string]map[int]model.RevisionGeneration `json:"generations"`
	Values      map[string]model.QueryValue                 `json:"values"`
	Reports     map[string]model.GenerationReport           `json:"reports"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data Data
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, data: Data{map[string]model.BranchSession{}, map[string]map[int]model.RevisionGeneration{}, map[string]model.QueryValue{}, map[string]model.GenerationReport{}}}
	b, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(b, &s.data)
	}
	if os.IsNotExist(err) {
		err = nil
	}
	return s, err
}
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
func key(branch string, g int) string { return fmt.Sprintf("%s:%d", branch, g) }
func (s *Store) OpenBranch(b model.BranchSession, g model.RevisionGeneration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Branches[b.ID]; ok {
		return errors.New("branch exists")
	}
	s.data.Branches[b.ID] = b
	s.data.Generations[b.ID] = map[int]model.RevisionGeneration{0: g}
	return s.save()
}
func (s *Store) Branch(id string) (model.BranchSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.data.Branches[id]
	if !ok {
		return b, ErrNotFound
	}
	return b, nil
}
func (s *Store) Generation(id string, g int) (model.RevisionGeneration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Generations[id][g]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (s *Store) AddGeneration(id string, expected int, g model.RevisionGeneration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data.Branches[id]
	if !ok {
		return ErrNotFound
	}
	if b.LatestGeneration != expected {
		return fmt.Errorf("%w: expected %d, current %d", ErrStaleGeneration, expected, b.LatestGeneration)
	}
	if g.Generation != expected+1 {
		return errors.New("generation must increment by one")
	}
	s.data.Generations[id][g.Generation] = g
	b.LatestGeneration = g.Generation
	b.UpdatedAt = time.Now().UTC()
	s.data.Branches[id] = b
	return s.save()
}
func (s *Store) Seal(id string, g int, commit, digest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data.Generations[id][g]
	if !ok {
		return ErrNotFound
	}
	if v.SourceTreeDigest != digest {
		return fmt.Errorf("tree digest mismatch: generation=%s supplied=%s", v.SourceTreeDigest, digest)
	}
	v.Sealed = true
	v.CommitSHA = commit
	s.data.Generations[id][g] = v
	return s.save()
}
func (s *Store) PutValue(v model.QueryValue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Values[v.ComputationFingerprint] = v
	return s.save()
}
func (s *Store) Value(fp string) (model.QueryValue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Values[fp]
	return v, ok
}
func (s *Store) PutReport(r model.GenerationReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Reports[key(r.BranchID, r.Generation)] = r
	return s.save()
}
func (s *Store) Report(id string, g int) (model.GenerationReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.data.Reports[key(id, g)]
	if !ok {
		return r, ErrNotFound
	}
	return r, nil
}
func (s *Store) Branches() []model.BranchSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := make([]model.BranchSession, 0, len(s.data.Branches))
	for _, b := range s.data.Branches {
		v = append(v, b)
	}
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v
}
