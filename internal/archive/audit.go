package archive

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (s *Store) AppendEvent(e AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	if s.dir != "" {
		if err := os.MkdirAll(s.dir, 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(s.dir, "audit.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		b, _ := json.Marshal(e)
		if _, err = f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return s.persistLocked()
}

func (s *Store) Events(batchID string) []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AuditEvent, 0)
	for _, e := range s.events {
		if batchID == "" || e.BatchID == batchID {
			out = append(out, e)
		}
	}
	return out
}

func (s *Store) EventsBetween(batchID string, from, to *time.Time) []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AuditEvent, 0)
	for _, e := range s.events {
		if e.BatchID != batchID || from != nil && e.At.Before(*from) || to != nil && e.At.After(*to) {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At.Equal(out[j].At) {
			return out[i].ID < out[j].ID
		}
		return out[i].At.Before(out[j].At)
	})
	return out
}

func ReadAuditFile(path string) ([]AuditEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []AuditEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, scanner.Err()
}
