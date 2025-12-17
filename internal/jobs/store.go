package jobs

import (
	"sync"
	"time"
)

// Job represents the state of a single PDF process
type Job struct {
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Data      map[string]string `json:"data,omitempty"`
	Progress  int               `json:"progress"`
	Operation string            `json:"operation"` // e.g., "downloading", "compressing", "uploading"
}

// Store holds all jobs in memory
type Store struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	watchers map[string][]chan *Job
}

func NewStore() *Store {
	return &Store{jobs: make(map[string]*Job), watchers: make(map[string][]chan *Job)}
}

// Subscribe returns a channel that receives job updates for the given id.
// The returned function should be called to unsubscribe when done.
func (s *Store) Subscribe(id string) (<-chan *Job, func()) {
	ch := make(chan *Job, 256)
	s.mu.Lock()
	s.watchers[id] = append(s.watchers[id], ch)
	job := s.jobs[id]
	s.mu.Unlock()

	if job != nil {
		// send current state (as a copy to prevent mutation issues)
		jobCopy := &Job{
			Status:    job.Status,
			Message:   job.Message,
			Data:      make(map[string]string),
			Progress:  job.Progress,
			Operation: job.Operation,
		}
		// Copy the data map
		for k, v := range job.Data {
			jobCopy.Data[k] = v
		}
		ch <- jobCopy
	}

	return ch, func() {
		s.mu.Lock()
		watchers := s.watchers[id]
		for i, c := range watchers {
			if c == ch {
				s.watchers[id] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		close(ch)
	}
}

func (s *Store) broadcastLocked(id string) {
	job := s.jobs[id]
	jobCopy := &Job{
		Status:    job.Status,
		Message:   job.Message,
		Data:      make(map[string]string),
		Progress:  job.Progress,
		Operation: job.Operation,
	}
	for k, v := range job.Data {
		jobCopy.Data[k] = v
	}

	isTerminal := job.Status == "success" || job.Status == "error"
	watchers := append([]chan *Job(nil), s.watchers[id]...)

	for _, ch := range watchers {
		if isTerminal {
			select {
			case ch <- jobCopy:
			case <-time.After(5 * time.Second):
			}
		} else {
			select {
			case ch <- jobCopy:
			default:
			}
		}
	}
}

func (s *Store) broadcast(id string) {
	s.mu.Lock()
	s.broadcastLocked(id)
	s.mu.Unlock()
}

func (s *Store) Create(id string) {
	s.mu.Lock()
	s.jobs[id] = &Job{Status: "pending", Message: "", Data: nil, Progress: 0, Operation: ""}
	s.broadcastLocked(id)
	s.mu.Unlock()
}

func (s *Store) Update(id, status, msg string, data map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = msg
		j.Data = data
		s.broadcastLocked(id)
	}
}

// UpdateWithOperation updates message, optional data, and operation type
func (s *Store) UpdateWithOperation(id, status, msg string, data map[string]string, operation string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = msg
		j.Data = data
		j.Operation = operation
		s.broadcastLocked(id)
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
		s.broadcastLocked(id)
	}
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}
