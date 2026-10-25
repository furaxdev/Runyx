// Package automation implements item 13: scheduled (cron-style) tasks,
// folder watching, and inbound webhooks, each triggering a callback the
// caller defines (e.g. "run this prompt against a model" — kept out of
// this package to avoid a dependency on internal/api/backend).
package automation

import (
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
)

// Task is one scheduled job.
type Task struct {
	ID       string
	Name     string
	CronExpr string // standard 5-field cron expression
	Run      func() // invoked on schedule
}

// Scheduler runs Tasks on their cron schedules.
type Scheduler struct {
	cron *cron.Cron
	mu   sync.Mutex
	ids  map[string]cron.EntryID
}

// NewScheduler returns a ready-to-use scheduler. Call Start to begin
// firing tasks.
func NewScheduler() *Scheduler {
	return &Scheduler{cron: cron.New(), ids: map[string]cron.EntryID{}}
}

// Start begins running scheduled tasks in the background.
func (s *Scheduler) Start() { s.cron.Start() }

// Stop halts the scheduler, waiting for any in-flight task to finish.
func (s *Scheduler) Stop() { s.cron.Stop() }

// Add schedules a task. Replaces any existing task with the same ID.
func (s *Scheduler) Add(t Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.ids[t.ID]; ok {
		s.cron.Remove(existing)
	}
	id, err := s.cron.AddFunc(t.CronExpr, t.Run)
	if err != nil {
		return fmt.Errorf("schedule task %q (%q): %w", t.Name, t.CronExpr, err)
	}
	s.ids[t.ID] = id
	return nil
}

// Remove unschedules a task by ID.
func (s *Scheduler) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.ids[id]; ok {
		s.cron.Remove(entryID)
		delete(s.ids, id)
	}
}

// IDs returns every currently scheduled task ID.
func (s *Scheduler) IDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.ids))
	for id := range s.ids {
		out = append(out, id)
	}
	return out
}
