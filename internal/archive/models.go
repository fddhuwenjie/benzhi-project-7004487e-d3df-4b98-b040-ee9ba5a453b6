package archive

import (
	"os"
	"sync"
	"time"
)

type AuditEvent struct {
	ID       string         `json:"id"`
	BatchID  string         `json:"batch_id"`
	Type     string         `json:"type"`
	Revision int            `json:"revision"`
	At       time.Time      `json:"at"`
	Actor    string         `json:"actor"`
	Detail   map[string]any `json:"detail,omitempty"`
}

type ArchiveRecord struct {
	ID             string    `json:"id"`
	BatchID        string    `json:"batch_id"`
	FinalRevision  int       `json:"final_revision"`
	RecordDigest   string    `json:"record_digest"`
	EvidenceIndex  []string  `json:"evidence_index"`
	ArchivedBy     string    `json:"archived_by"`
	ArchivedAt     time.Time `json:"archived_at"`
	DigestVerified bool      `json:"digest_verified"`
}

type Store struct {
	mu      sync.Mutex
	dir     string
	events  []AuditEvent
	records map[string]ArchiveRecord
	audit   *os.File
}
