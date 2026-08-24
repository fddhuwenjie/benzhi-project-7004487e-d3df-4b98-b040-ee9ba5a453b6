package archive

import (
	"path/filepath"
	"testing"
)

func TestSnapshotAndAudit(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.AppendEvent(AuditEvent{ID: "e1", BatchID: "b1", Type: "created"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateArchive("b1", 3, []string{"x"}, "u"); err != nil {
		t.Fatal(err)
	}
	s2 := NewStore(dir)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	if len(s2.Events("b1")) != 1 {
		t.Fatalf("events not restored")
	}
	if _, ok := s2.Archive("b1"); !ok {
		t.Fatalf("archive not restored")
	}
	if _, err := ReadAuditFile(filepath.Join(dir, "audit.jsonl")); err != nil {
		t.Fatal(err)
	}
}
