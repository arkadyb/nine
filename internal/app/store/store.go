package store

import (
	"sort"
	"sync"
	"time"
)

type Store struct {
	assets sync.Map
}

type Segment struct {
	Index      int
	CDNURL     string
	ReceivedAt time.Time
}

type Manifest struct {
	Segments    []Segment
	LastUpdated time.Time
}

type manifestState struct {
	mu          sync.RWMutex
	segments    map[int]Segment
	lastUpdated time.Time
}

func New() *Store {
	return &Store{}
}

func (s *Store) UpsertSegment(assetID string, seg Segment) {
	state := s.manifestState(assetID)
	state.upsert(seg)
}

func (s *Store) GetManifest(assetID string) (Manifest, bool) {
	state, ok := s.lookup(assetID)
	if !ok {
		return Manifest{}, false
	}
	return state.manifest(), true
}

func (s *Store) manifestState(assetID string) *manifestState {
	if state, ok := s.lookup(assetID); ok {
		return state
	}

	candidate := &manifestState{
		segments: make(map[int]Segment),
	}
	actual, _ := s.assets.LoadOrStore(assetID, candidate)
	return actual.(*manifestState)
}

func (s *Store) lookup(assetID string) (*manifestState, bool) {
	value, ok := s.assets.Load(assetID)
	if !ok {
		return nil, false
	}
	return value.(*manifestState), true
}

func (a *manifestState) upsert(seg Segment) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.segments[seg.Index] = seg
	if seg.ReceivedAt.After(a.lastUpdated) {
		a.lastUpdated = seg.ReceivedAt
	}
}

func (a *manifestState) manifest() Manifest {
	a.mu.RLock()
	defer a.mu.RUnlock()

	segments := make([]Segment, 0, len(a.segments))
	for _, seg := range a.segments {
		segments = append(segments, seg)
	}
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Index < segments[j].Index
	})
	return Manifest{
		Segments:    segments,
		LastUpdated: a.lastUpdated,
	}
}
