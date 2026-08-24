package archive

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *Store) SaveState(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.persistStateLocked(b)
}

func (s *Store) LoadState(dst any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(s.dir, "state.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func (s *Store) persistStateLocked(b []byte) error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, "state.json.tmp")
	final := filepath.Join(s.dir, "state.json")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}
