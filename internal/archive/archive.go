package archive

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func NewStore(dir string) *Store { return &Store{dir: dir, records: map[string]ArchiveRecord{}} }

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(s.dir, "snapshot.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var snap struct {
		Events  []AuditEvent             `json:"events"`
		Records map[string]ArchiveRecord `json:"records"`
	}
	if err := json.Unmarshal(b, &snap); err != nil {
		return err
	}
	s.events = snap.Events
	if snap.Records != nil {
		s.records = snap.Records
	}
	return nil
}
