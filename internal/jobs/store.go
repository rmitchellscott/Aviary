package jobs

import (
	"sync"
)

// Job represents the state of a single PDF process
type Job struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Progress  int    `json:"progress"`
	Operation string `json:"operation"` // e.g., "downloading", "compressing", "uploading"
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
	ch := make(chan *Job, 1)
	s.mu.Lock()
	s.watchers[id] = append(s.watchers[id], ch)
	job := s.jobs[id]
	s.mu.Unlock()

	if job != nil {
		// send current state
		ch <- job
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
	watchers := append([]chan *Job(nil), s.watchers[id]...)
	// lock held when called
	for _, ch := range watchers {
		select {
		case ch <- job:
		default:
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
	s.jobs[id] = &Job{Status: "pending", Message: "", Progress: 0, Operation: ""}
	s.broadcastLocked(id)
	s.mu.Unlock()
}

func (s *Store) Update(id, status, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = msg
		s.broadcastLocked(id)
	}
}

// UpdateWithOperation updates both message and operation type
func (s *Store) UpdateWithOperation(id, status, msg, operation string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Message = msg
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
