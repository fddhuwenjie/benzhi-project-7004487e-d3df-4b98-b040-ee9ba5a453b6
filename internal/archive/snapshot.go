package archive

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *Store) persistLocked() error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	snapshot := struct {
		Events  []AuditEvent             `json:"events"`
		Records map[string]ArchiveRecord `json:"records"`
	}{s.events, s.records}
	b, _ := json.MarshalIndent(snapshot, "", "  ")
	tmp := filepath.Join(s.dir, "snapshot.json.tmp")
	final := filepath.Join(s.dir, "snapshot.json")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	return nil
}
