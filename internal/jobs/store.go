package jobs

import (
	"sync"
)

// Job represents the state of a single PDF process
type Job struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Progress int    `json:"progress"`
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
	s.jobs[id] = &Job{Status: "pending", Message: "", Progress: 0}
}

func (s *Store) Update(id, status, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = msg
	}
}

// UpdateProgress sets the progress (0-100) for a job.
func (s *Store) UpdateProgress(id string, p int) {
	if p < 0 {
		p = 0
	} else if p > 100 {
		p = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Progress = p
	}
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}
