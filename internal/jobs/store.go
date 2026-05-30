package jobs

import (
	"sync"
	"time"
)

// store is an in-memory job and sandbox registry. Jobs are ephemeral and are
// not persisted across restarts in the MVP.
type store struct {
	mu        sync.RWMutex
	jobs      map[string]*Job
	sandboxes map[string]string // sandbox session id -> job id
}

func newStore() *store {
	return &store{
		jobs:      make(map[string]*Job),
		sandboxes: make(map[string]string),
	}
}

func (s *store) put(j *Job) {
	s.mu.Lock()
	s.jobs[j.ID] = j
	s.mu.Unlock()
}

func (s *store) get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *store) linkSandbox(sessionID, jobID string) {
	s.mu.Lock()
	s.sandboxes[sessionID] = jobID
	s.mu.Unlock()
}

func (s *store) jobForSandbox(sessionID string) (*Job, bool) {
	s.mu.RLock()
	jobID, ok := s.sandboxes[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return s.get(jobID)
}

func (s *store) list() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

// purgeTerminalBefore removes terminal (complete/error/expired/cancelled) jobs
// created before cutoff and returns their IDs (so on-disk scratch can be
// cleaned). Live jobs and any that still own an MCP session are never removed.
func (s *store) purgeTerminalBefore(cutoff time.Time) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []string
	for id, j := range s.jobs {
		if j.CreatedAt.After(cutoff) {
			continue
		}
		switch j.Status() {
		case StatusComplete, StatusError, StatusExpired, StatusCancelled:
			if j.Session() != nil {
				continue // a live sandbox session is still attached
			}
			delete(s.jobs, id)
			removed = append(removed, id)
			for sid, jid := range s.sandboxes {
				if jid == id {
					delete(s.sandboxes, sid)
				}
			}
		}
	}
	return removed
}
