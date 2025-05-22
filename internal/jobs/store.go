package jobs

import (
	"sync"
)

// Job represents the state of a single PDF process
type Job struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Store holds all jobs in memory
type Store struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewStore() *Store {
	return &Store{jobs: make(map[string]*Job)}
}

func (s *Store) Create(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[id] = &Job{Status: "pending", Message: ""}
}

func (s *Store) Update(id, status, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = msg
	}
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}
