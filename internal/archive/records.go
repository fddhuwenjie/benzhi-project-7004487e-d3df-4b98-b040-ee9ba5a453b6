package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func (s *Store) CreateArchive(batchID string, revision int, evidence []string, by string) (ArchiveRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.records[batchID]; ok {
		return existing, nil
	}
	seen := map[string]bool{}
	for i, ref := range evidence {
		evidence[i] = strings.TrimSpace(ref)
		if evidence[i] == "" || seen[evidence[i]] {
			return ArchiveRecord{}, fmt.Errorf("invalid evidence index at %d", i)
		}
		seen[evidence[i]] = true
	}
	now := time.Now().UTC()
	payload := fmt.Sprintf("%s:%d:%v", batchID, revision, evidence)
	sum := sha256.Sum256([]byte(payload))
	record := ArchiveRecord{ID: fmt.Sprintf("archive-%s-%d", batchID, revision), BatchID: batchID, FinalRevision: revision, RecordDigest: hex.EncodeToString(sum[:]), EvidenceIndex: append([]string(nil), evidence...), ArchivedBy: by, ArchivedAt: now, DigestVerified: true}
	s.records[batchID] = record
	if err := s.persistLocked(); err != nil {
		delete(s.records, batchID)
		return ArchiveRecord{}, err
	}
	return record, nil
}

func (s *Store) Archive(batchID string) (ArchiveRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[batchID]
	return record, ok
}
