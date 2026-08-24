package review

import (
	"errors"
	"time"
)

var ErrDuplicateReviewer = errors.New("duplicate reviewer")

type ReviewDecision struct {
	ID           string    `json:"id"`
	BatchID      string    `json:"batch_id"`
	Phase        int       `json:"phase"`
	Reviewer     string    `json:"reviewer"`
	Decision     string    `json:"decision"`
	Findings     string    `json:"findings"`
	EvidenceRefs []string  `json:"evidence_refs"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

type ReviewSummary struct {
	Phase        int      `json:"phase"`
	Collected    int      `json:"collected"`
	Required     int      `json:"required"`
	CoveredRefs  []string `json:"covered_refs"`
	MissingRefs  []string `json:"missing_refs"`
	EvidenceDiff []string `json:"evidence_diff,omitempty"`
}
