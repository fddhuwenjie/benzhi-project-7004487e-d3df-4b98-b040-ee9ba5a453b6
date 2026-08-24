package review

import (
	"errors"
	"fmt"
	"strings"
)

func ValidateDecision(d ReviewDecision, existing []ReviewDecision) error {
	if strings.TrimSpace(d.Reviewer) == "" {
		return errors.New("reviewer is required")
	}
	if d.Decision != "approve" && d.Decision != "return" {
		return fmt.Errorf("decision must be approve or return")
	}
	if len(d.EvidenceRefs) == 0 {
		return errors.New("evidence_refs is required")
	}
	seen := map[string]bool{}
	for i, ref := range d.EvidenceRefs {
		d.EvidenceRefs[i] = strings.TrimSpace(ref)
		if d.EvidenceRefs[i] == "" || seen[d.EvidenceRefs[i]] {
			return errors.New("invalid evidence_refs")
		}
		seen[d.EvidenceRefs[i]] = true
	}
	for _, e := range existing {
		if e.Phase == d.Phase && e.Reviewer == d.Reviewer {
			return ErrDuplicateReviewer
		}
	}
	return nil
}

func Result(decisions []ReviewDecision, phase int) (string, error) {
	var found []ReviewDecision
	for i := len(decisions) - 1; i >= 0 && len(found) < 2; i-- {
		if decisions[i].Phase == phase {
			found = append(found, decisions[i])
		}
	}
	if len(found) < 2 {
		return "pending", nil
	}
	if found[0].Reviewer == found[1].Reviewer {
		return "return", ErrDuplicateReviewer
	}
	for _, d := range found {
		if d.Decision != "approve" {
			return "return", nil
		}
	}
	return "approve", nil
}
