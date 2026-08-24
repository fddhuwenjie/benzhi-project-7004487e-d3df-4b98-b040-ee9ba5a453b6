package transport

import (
	"errors"
	"net/http"
	"strings"

	"museum-desalination/internal/assessment"
	"museum-desalination/internal/review"
	"museum-desalination/internal/workflow"
)

func writeErr(w http.ResponseWriter, err error) {
	status, code := http.StatusBadRequest, "INVALID_REQUEST"
	if errors.Is(err, workflow.ErrNotFound) {
		status, code = http.StatusNotFound, "NOT_FOUND"
	}
	if errors.Is(err, workflow.ErrConflict) {
		status, code = http.StatusConflict, "REVISION_CONFLICT"
	}
	if errors.Is(err, workflow.ErrInvalidState) {
		status, code = http.StatusConflict, "INVALID_STATE"
	}
	if errors.Is(err, assessment.ErrEvidenceMissing) {
		code = "EVIDENCE_MISSING"
	}
	if errors.Is(err, workflow.ErrDuplicateObservation) {
		code = "DUPLICATE_OBSERVATION_TIME"
	}
	if errors.Is(err, workflow.ErrRemediationRequired) {
		code = "REMEDIATION_REQUIRED"
	}
	if errors.Is(err, workflow.ErrCatalogConflict) {
		status, code = http.StatusConflict, "CATALOG_REF_CONFLICT"
	}
	if errors.Is(err, workflow.ErrIdempotencyConflict) {
		status, code = http.StatusConflict, "IDEMPOTENCY_CONFLICT"
	}
	if errors.Is(err, workflow.ErrDeadlineExceeded) {
		status, code = http.StatusConflict, "DEADLINE_EXCEEDED"
	}
	if errors.Is(err, workflow.ErrPhaseLocked) {
		status, code = http.StatusConflict, "PHASE_LOCKED"
	}
	if errors.Is(err, workflow.ErrArchiveEvidence) {
		code = "INVALID_EVIDENCE_INDEX"
	}
	if errors.Is(err, assessment.ErrInvalidPlan) {
		code = "INVALID_PLAN_PARAMETERS"
	}
	if errors.Is(err, workflow.ErrRemediationConflict) {
		code = "REMEDIATION_OWNER_CONFLICT"
	}
	if errors.Is(err, workflow.ErrInvalidFilter) {
		code = "INVALID_FILTER"
	}
	if errors.Is(err, workflow.ErrResponsibleConflict) {
		status, code = http.StatusConflict, "RESPONSIBLE_USER_CONFLICT"
	}
	if errors.Is(err, workflow.ErrTrendNotesRequired) {
		code = "TREND_REVERSAL_NOTES_REQUIRED"
	}
	if errors.Is(err, workflow.ErrReviewEvidence) {
		code = "REVIEW_EVIDENCE_COVERAGE"
	}
	if errors.Is(err, workflow.ErrInvalidCursor) {
		code = "INVALID_AUDIT_CURSOR"
	}
	if errors.Is(err, workflow.ErrInvalidLimit) {
		code = "INVALID_AUDIT_LIMIT"
	}
	if errors.Is(err, workflow.ErrActorRequired) {
		code = "ACTOR_REQUIRED"
	}
	if errors.Is(err, workflow.ErrHandoffRequired) {
		code = "INDEPENDENT_HANDOFF_REQUIRED"
	}
	for _, field := range []string{"catalog_ref", "title", "responsible_user"} {
		if strings.HasPrefix(err.Error(), field+"_") {
			code = "INVALID_" + strings.ToUpper(field)
			writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "field": field, "message": err.Error()}})
			return
		}
	}
	var readiness *workflow.ReadinessError
	if errors.As(err, &readiness) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "READINESS_INCOMPLETE", "message": err.Error(), "missing": readiness.Missing}})
		return
	}
	if errors.Is(err, review.ErrDuplicateReviewer) {
		code = "DUPLICATE_REVIEWER"
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": err.Error()}})
}
