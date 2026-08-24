package workflow

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/assessment"
	"museum-desalination/internal/review"
	"museum-desalination/internal/treatment"
)

const (
	StatusDraft      = "DRAFT"
	StatusPending    = "PENDING_PROCESSING"
	StatusProcessing = "PROCESSING"
	StatusPaused     = "PAUSED"
	StatusReview     = "PENDING_REVIEW"
	StatusApproved   = "APPROVED"
	StatusArchived   = "ARCHIVED"
)

var (
	ErrNotFound             = errors.New("batch not found")
	ErrConflict             = errors.New("revision conflict")
	ErrInvalidState         = errors.New("invalid state transition")
	ErrAlreadyDone          = errors.New("already completed")
	ErrEvidenceMissing      = assessment.ErrEvidenceMissing
	ErrDuplicateObservation = errors.New("duplicate observation time")
	ErrRemediationRequired  = errors.New("remediation note and owner are required")
	ErrCatalogConflict      = errors.New("catalog_ref already has an active batch")
	ErrIdempotencyConflict  = errors.New("idempotency key reused with different request")
	ErrDeadlineExceeded     = errors.New("observation deadline exceeded")
	ErrPhaseLocked          = errors.New("phase is locked")
	ErrArchiveEvidence      = errors.New("invalid archive evidence index")
	ErrRemediationConflict  = errors.New("remediation owner must differ from triggering operator")
	ErrInvalidFilter        = errors.New("invalid batch filter")
	ErrTrendNotesRequired   = errors.New("sample_notes required for trend reversal")
	ErrReviewEvidence       = errors.New("review evidence does not cover sample evidence")
	ErrInvalidCursor        = errors.New("invalid audit cursor")
	ErrInvalidLimit         = errors.New("invalid audit limit")
	ErrResponsibleConflict  = errors.New("responsible user already has an active batch")
	ErrActorRequired        = errors.New("completion actor is required")
	ErrHandoffRequired      = errors.New("independent handoff is required")
)

type ReadinessError struct{ Missing []string }

func (e *ReadinessError) Error() string {
	return "phase readiness incomplete: " + strings.Join(e.Missing, ", ")
}

type ProtectionBatch struct {
	ID                     string                               `json:"id"`
	CatalogRef             string                               `json:"catalog_ref"`
	Title                  string                               `json:"title"`
	Status                 string                               `json:"status"`
	CurrentPhase           int                                  `json:"current_phase"`
	Revision               int                                  `json:"revision"`
	ResponsibleUser        string                               `json:"responsible_user"`
	CreatedAt              time.Time                            `json:"created_at"`
	UpdatedAt              time.Time                            `json:"updated_at"`
	Samples                []assessment.Sample                  `json:"samples"`
	Plans                  []assessment.TreatmentPlan           `json:"plans"`
	Tasks                  []treatment.Task                     `json:"tasks"`
	Observations           []treatment.ObservationRecord        `json:"observations"`
	ObservationSummaries   map[int]treatment.ObservationSummary `json:"observation_summaries,omitempty"`
	Reviews                []review.ReviewDecision              `json:"reviews"`
	Archive                *archive.ArchiveRecord               `json:"archive,omitempty"`
	LockedPhases           map[int]bool                         `json:"locked_phases"`
	Readiness              map[int]map[string]bool              `json:"readiness,omitempty"`
	PlanRefreshRequired    bool                                 `json:"plan_refresh_required,omitempty"`
	PlanRefreshSampleIDs   []string                             `json:"plan_refresh_sample_ids,omitempty"`
	PlanRefreshFromVersion int                                  `json:"plan_refresh_from_version,omitempty"`
	PendingRemediation     *Remediation                         `json:"pending_remediation,omitempty"`
	ObservationCoverage    map[int]bool                         `json:"observation_coverage,omitempty"`
	LatestAuditAt          *time.Time                           `json:"latest_audit_at,omitempty"`
	PlanVersion            int                                  `json:"plan_version,omitempty"`
	InitialAuditEventID    string                               `json:"initial_audit_event_id,omitempty"`
	AuditEventID           string                               `json:"audit_event_id,omitempty"`
	InitialRevision        int                                  `json:"initial_revision,omitempty"`
	InitialSampleCount     int                                  `json:"initial_sample_count,omitempty"`
	PlanRefreshSummary     *PlanRefreshSummary                  `json:"plan_refresh_summary,omitempty"`
	PlanPreview            *assessment.TreatmentPlan            `json:"plan_preview,omitempty"`
	ReadinessSnapshots     map[int]ReadinessSnapshot            `json:"readiness_snapshots,omitempty"`
	ReviewSummaries        map[int]review.ReviewSummary         `json:"review_summaries,omitempty"`
}

type PlanRefreshSummary struct {
	BeforeAverageChloride float64 `json:"before_average_initial_chloride"`
	AfterAverageChloride  float64 `json:"after_average_initial_chloride"`
	BeforeAverageMoisture float64 `json:"before_average_moisture_ratio"`
	AfterAverageMoisture  float64 `json:"after_average_moisture_ratio"`
	BeforeSampleCount     int     `json:"before_sample_count"`
	AfterSampleCount      int     `json:"after_sample_count"`
	AffectedPlanVersion   int     `json:"affected_plan_version"`
}

type ReadinessSnapshot struct {
	Phase               int      `json:"phase"`
	CoverageDates       []string `json:"coverage_dates"`
	AnomalyResult       string   `json:"anomaly_result"`
	PlanVersion         int      `json:"plan_version"`
	SampleEvidenceCount int      `json:"sample_evidence_count"`
	CapturedRevision    int      `json:"captured_revision"`
}

type AuditPage struct {
	Events        []archive.AuditEvent `json:"events"`
	NextCursor    string               `json:"next_cursor,omitempty"`
	TypeCounts    map[string]int       `json:"type_counts"`
	RevisionRange map[string]int       `json:"revision_range"`
}

type Remediation struct {
	Code            string         `json:"code"`
	Phase           int            `json:"phase"`
	Raw             map[string]any `json:"raw,omitempty"`
	TriggerOperator string         `json:"trigger_operator"`
	Note            string         `json:"note"`
	Owner           string         `json:"owner"`
	EvidenceRefs    []string       `json:"evidence_refs"`
}

type BatchFilter struct {
	Status, CurrentPhase, ResponsibleUser string
	From, To                              *time.Time
}
type BatchListResult struct {
	Batches      []ProtectionBatch `json:"batches"`
	StatusCounts map[string]int    `json:"status_counts"`
	PhaseCounts  map[int]int       `json:"phase_counts"`
	NextCursor   string            `json:"next_cursor,omitempty"`
}

type CreateRequest struct {
	CatalogRef      string              `json:"catalog_ref"`
	Title           string              `json:"title"`
	ResponsibleUser string              `json:"responsible_user"`
	Samples         []assessment.Sample `json:"samples"`
}
type Workflow struct {
	mu          sync.RWMutex
	batches     map[string]*ProtectionBatch
	idempotency map[string]idempotencyRecord
	archive     *archive.Store
	seq         int
}

type persistedState struct {
	Batches map[string]*ProtectionBatch `json:"batches"`
	Seq     int                         `json:"seq"`
}

type idempotencyRecord struct {
	Fingerprint string
	Batch       ProtectionBatch
}

func New(a *archive.Store) *Workflow {
	w := &Workflow{batches: map[string]*ProtectionBatch{}, idempotency: map[string]idempotencyRecord{}, archive: a}
	var state persistedState
	if a != nil && a.LoadState(&state) == nil && state.Batches != nil {
		w.batches, w.seq = state.Batches, state.Seq
	}
	return w
}

func (w *Workflow) Create(req CreateRequest, idem string) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := assessment.ValidateBatchField("catalog_ref", req.CatalogRef); err != nil {
		return ProtectionBatch{}, err
	}
	if err := assessment.ValidateBatchField("title", req.Title); err != nil {
		return ProtectionBatch{}, err
	}
	if err := assessment.ValidateBatchField("responsible_user", req.ResponsibleUser); err != nil {
		return ProtectionBatch{}, err
	}
	req.CatalogRef = strings.TrimSpace(req.CatalogRef)
	req.Title = strings.TrimSpace(req.Title)
	req.ResponsibleUser = strings.TrimSpace(req.ResponsibleUser)
	fingerprint := requestFingerprint(req)
	if idem != "" {
		if rec, ok := w.idempotency["create:"+idem]; ok {
			if rec.Fingerprint != fingerprint {
				return ProtectionBatch{}, ErrIdempotencyConflict
			}
			return clone(rec.Batch), nil
		}
	}
	if req.CatalogRef == "" || req.Title == "" || req.ResponsibleUser == "" {
		return ProtectionBatch{}, errors.New("catalog_ref, title and responsible_user are required")
	}
	for _, existing := range w.batches {
		if existing.CatalogRef == req.CatalogRef && existing.Status != StatusArchived {
			return ProtectionBatch{}, ErrCatalogConflict
		}
		if existing.ResponsibleUser == req.ResponsibleUser && existing.Status != StatusArchived {
			return ProtectionBatch{}, ErrResponsibleConflict
		}
	}
	w.seq++
	id := fmt.Sprintf("batch-%06d", w.seq)
	now := time.Now().UTC()
	b := &ProtectionBatch{ID: id, CatalogRef: req.CatalogRef, Title: req.Title, Status: StatusDraft, CurrentPhase: 1, Revision: 1, InitialRevision: 1, ResponsibleUser: req.ResponsibleUser, CreatedAt: now, UpdatedAt: now, LockedPhases: map[int]bool{}, InitialSampleCount: len(req.Samples)}
	if len(req.Samples) > 0 {
		if err := assessment.ValidateSamples(req.Samples); err != nil {
			return ProtectionBatch{}, err
		}
	}
	for i := range req.Samples {
		s := req.Samples[i]
		s.ID = fmt.Sprintf("sample-%s-%02d", id, i+1)
		s.BatchID = id
		s.ValidatedAt = now
		b.Samples = append(b.Samples, s)
	}
	if len(b.Samples) > 0 {
		plan, err := assessment.GeneratePlan(id, req.ResponsibleUser, b.Samples, 1)
		if err != nil {
			return ProtectionBatch{}, err
		}
		b.Plans = append(b.Plans, plan)
		b.ObservationSummaries = treatment.SummariesForPlan(nil, plan)
		for phase, summary := range b.ObservationSummaries {
			summary.Revision = b.Revision
			b.ObservationSummaries[phase] = summary
		}
		b.Status = StatusPending
	}
	w.batches[id] = b
	w.auditLocked(b, "BATCH_CREATED", req.ResponsibleUser, nil)
	out := clone(*b)
	if idem != "" {
		w.idempotency["create:"+idem] = idempotencyRecord{Fingerprint: fingerprint, Batch: out}
	}
	return out, nil
}

func requestFingerprint(req CreateRequest) string {
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func clone(b ProtectionBatch) ProtectionBatch {
	b.Samples = append([]assessment.Sample(nil), b.Samples...)
	b.Plans = append([]assessment.TreatmentPlan(nil), b.Plans...)
	b.Tasks = append([]treatment.Task(nil), b.Tasks...)
	b.Observations = append([]treatment.ObservationRecord(nil), b.Observations...)
	if b.ObservationSummaries != nil {
		b.ObservationSummaries = map[int]treatment.ObservationSummary{}
		for k, v := range b.ObservationSummaries {
			b.ObservationSummaries[k] = v
		}
	}
	b.Reviews = append([]review.ReviewDecision(nil), b.Reviews...)
	b.PlanRefreshSampleIDs = append([]string(nil), b.PlanRefreshSampleIDs...)
	if b.PendingRemediation != nil {
		r := *b.PendingRemediation
		r.EvidenceRefs = append([]string(nil), r.EvidenceRefs...)
		b.PendingRemediation = &r
	}
	if b.ObservationCoverage != nil {
		coverage := b.ObservationCoverage
		b.ObservationCoverage = map[int]bool{}
		for k, v := range coverage {
			b.ObservationCoverage[k] = v
		}
	}
	if b.LockedPhases != nil {
		b.LockedPhases = map[int]bool{}
		for k, v := range b.LockedPhases {
			b.LockedPhases[k] = v
		}
	}
	if b.Readiness != nil {
		readiness := b.Readiness
		b.Readiness = make(map[int]map[string]bool, len(readiness))
		for phase, checks := range readiness {
			b.Readiness[phase] = checks
		}
	}
	if b.PlanRefreshSummary != nil {
		s := *b.PlanRefreshSummary
		b.PlanRefreshSummary = &s
	}
	if b.PlanPreview != nil {
		p := *b.PlanPreview
		p.PhaseSteps = append([]assessment.PhaseStep(nil), p.PhaseSteps...)
		b.PlanPreview = &p
	}
	if b.ReadinessSnapshots != nil {
		original := b.ReadinessSnapshots
		b.ReadinessSnapshots = map[int]ReadinessSnapshot{}
		for phase, snap := range original {
			snap.CoverageDates = append([]string(nil), snap.CoverageDates...)
			b.ReadinessSnapshots[phase] = snap
		}
	}
	if b.ReviewSummaries != nil {
		original := b.ReviewSummaries
		b.ReviewSummaries = map[int]review.ReviewSummary{}
		for phase, summary := range original {
			summary.CoveredRefs = append([]string(nil), summary.CoveredRefs...)
			summary.MissingRefs = append([]string(nil), summary.MissingRefs...)
			summary.EvidenceDiff = append([]string(nil), summary.EvidenceDiff...)
			b.ReviewSummaries[phase] = summary
		}
	}
	return b
}

func (w *Workflow) Get(id string) (ProtectionBatch, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	b, ok := w.batches[id]
	if !ok {
		return ProtectionBatch{}, ErrNotFound
	}
	return clone(*b), nil
}
func (w *Workflow) List() []ProtectionBatch {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]ProtectionBatch, 0, len(w.batches))
	for _, b := range w.batches {
		item := clone(*b)
		item.ObservationCoverage = map[int]bool{}
		if len(item.Plans) > 0 {
			item.PlanVersion = item.Plans[len(item.Plans)-1].Version
			for _, step := range item.Plans[len(item.Plans)-1].PhaseSteps {
				var started time.Time
				for _, task := range item.Tasks {
					if task.Phase == step.Phase {
						started = task.StartedAt
					}
				}
				item.ObservationCoverage[step.Phase] = treatment.Coverage(item.Observations, step.Phase, started)
			}
		}
		events := w.archive.Events(item.ID)
		if len(events) > 0 {
			latest := events[0].At
			for _, e := range events[1:] {
				if e.At.After(latest) {
					latest = e.At
				}
			}
			item.LatestAuditAt = &latest
		}
		out = append(out, item)
	}
	return out
}

func (w *Workflow) ListFiltered(f BatchFilter) (BatchListResult, error) {
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return BatchListResult{}, ErrInvalidFilter
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]ProtectionBatch, 0)
	for _, b := range w.batches {
		if f.Status != "" && b.Status != f.Status || f.CurrentPhase != "" && fmt.Sprint(b.CurrentPhase) != f.CurrentPhase || f.ResponsibleUser != "" && b.ResponsibleUser != f.ResponsibleUser {
			continue
		}
		if f.From != nil && b.UpdatedAt.Before(*f.From) || f.To != nil && b.UpdatedAt.After(*f.To) {
			continue
		}
		item := clone(*b)
		item.ObservationCoverage = map[int]bool{}
		if len(item.Plans) > 0 {
			item.PlanVersion = item.Plans[len(item.Plans)-1].Version
			for _, step := range item.Plans[len(item.Plans)-1].PhaseSteps {
				var started time.Time
				for _, task := range item.Tasks {
					if task.Phase == step.Phase {
						started = task.StartedAt
					}
				}
				item.ObservationCoverage[step.Phase] = treatment.Coverage(item.Observations, step.Phase, started)
			}
		}
		events := w.archive.Events(item.ID)
		if len(events) > 0 {
			latest := events[0].At
			for _, e := range events[1:] {
				if e.At.After(latest) {
					latest = e.At
				}
			}
			item.LatestAuditAt = &latest
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].Revision < out[j].Revision
		}
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	res := BatchListResult{Batches: out, StatusCounts: map[string]int{}, PhaseCounts: map[int]int{}}
	for _, b := range out {
		res.StatusCounts[b.Status]++
		res.PhaseCounts[b.CurrentPhase]++
	}
	return res, nil
}

func (w *Workflow) checkLocked(id string, expected int) (*ProtectionBatch, error) {
	b, ok := w.batches[id]
	if !ok {
		return nil, ErrNotFound
	}
	if expected > 0 && b.Revision != expected {
		return nil, ErrConflict
	}
	return b, nil
}
func (w *Workflow) bump(b *ProtectionBatch) { b.Revision++; b.UpdatedAt = time.Now().UTC() }
func (w *Workflow) auditLocked(b *ProtectionBatch, typ, actor string, detail map[string]any) {
	e := archive.AuditEvent{ID: fmt.Sprintf("event-%s-%d-%d", b.ID, b.Revision, time.Now().UTC().UnixNano()), BatchID: b.ID, Type: typ, Revision: b.Revision, At: time.Now().UTC(), Actor: actor, Detail: detail}
	_ = w.archive.AppendEvent(e)
	if typ == "BATCH_CREATED" {
		b.InitialAuditEventID = e.ID
		b.AuditEventID = e.ID
	}
	_ = w.archive.SaveState(persistedState{Batches: w.batches, Seq: w.seq})
}

func (w *Workflow) AddSample(id string, expected int, s assessment.Sample, actor string) (ProtectionBatch, error) {
	return w.AddSamples(id, expected, []assessment.Sample{s}, actor)
}

func (w *Workflow) AddSamples(id string, expected int, samples []assessment.Sample, actor string) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(samples) == 0 {
		return ProtectionBatch{}, errors.New("sample is required")
	}
	b, err := w.checkLocked(id, expected)
	if err != nil {
		return ProtectionBatch{}, err
	}
	if b.Status != StatusDraft && b.Status != StatusPending && b.Status != StatusApproved {
		return ProtectionBatch{}, ErrInvalidState
	}
	beforeCount := len(b.Samples)
	var beforeChloride, beforeMoisture float64
	for _, existing := range b.Samples {
		beforeChloride += existing.InitialChloride
		beforeMoisture += existing.MoistureRatio
	}
	locations := map[string]bool{}
	for _, existing := range b.Samples {
		locations[strings.TrimSpace(existing.LocationCode)] = true
	}
	validated := make([]assessment.Sample, 0, len(samples))
	for i, s := range samples {
		s.LocationCode = strings.TrimSpace(s.LocationCode)
		if locations[s.LocationCode] {
			return ProtectionBatch{}, fmt.Errorf("location_code already exists: %s", s.LocationCode)
		}
		if err := assessment.ValidateSample(s); err != nil {
			if errors.Is(err, assessment.ErrEvidenceMissing) {
				w.auditLocked(b, "EVIDENCE_VALIDATION_FAILED", actor, map[string]any{"sample_count": len(samples)})
			}
			return ProtectionBatch{}, err
		}
		locations[s.LocationCode] = true
		s.ID = fmt.Sprintf("sample-%s-%d", id, len(b.Samples)+i+1)
		s.BatchID = id
		s.ValidatedAt = time.Now().UTC()
		validated = append(validated, s)
	}
	b.Samples = append(b.Samples, validated...)
	if len(b.Plans) == 0 {
		plan, planErr := assessment.GeneratePlan(id, b.ResponsibleUser, b.Samples, 1)
		if planErr != nil {
			return ProtectionBatch{}, planErr
		}
		b.Plans = append(b.Plans, plan)
		b.ObservationSummaries = treatment.SummariesForPlan(nil, plan)
	} else {
		b.PlanRefreshRequired = true
		b.PlanRefreshFromVersion = b.Plans[len(b.Plans)-1].Version
		b.PlanRefreshSampleIDs = make([]string, 0, len(validated))
		for _, s := range validated {
			b.PlanRefreshSampleIDs = append(b.PlanRefreshSampleIDs, s.ID)
		}
		var afterChloride, afterMoisture float64
		for _, sample := range b.Samples {
			afterChloride += sample.InitialChloride
			afterMoisture += sample.MoistureRatio
		}
		b.PlanRefreshSummary = &PlanRefreshSummary{BeforeAverageChloride: beforeChloride / float64(beforeCount), AfterAverageChloride: afterChloride / float64(len(b.Samples)), BeforeAverageMoisture: beforeMoisture / float64(beforeCount), AfterAverageMoisture: afterMoisture / float64(len(b.Samples)), BeforeSampleCount: beforeCount, AfterSampleCount: len(b.Samples), AffectedPlanVersion: b.PlanRefreshFromVersion}
	}
	b.Status = StatusPending
	w.bump(b)
	ids := make([]string, 0, len(validated))
	for _, s := range validated {
		ids = append(ids, s.ID)
	}
	w.auditLocked(b, "SAMPLE_VALIDATED", actor, map[string]any{"sample_ids": ids, "sample_count": len(ids), "plan_refresh_summary": b.PlanRefreshSummary})
	return clone(*b), nil
}

func (w *Workflow) RegeneratePlan(id string, expected int, author string) (ProtectionBatch, error) {
	return w.RegeneratePlanWithConfirm(id, expected, author, true)
}

func (w *Workflow) RegeneratePlanWithConfirm(id string, expected int, author string, confirm bool) (ProtectionBatch, error) {
	return w.ConfirmPlan(id, expected, author, confirm, nil)
}

func (w *Workflow) ConfirmPlan(id string, expected int, author string, confirm bool, requested *assessment.TreatmentPlan) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := w.checkLocked(id, expected)
	if err != nil {
		return ProtectionBatch{}, err
	}
	if strings.TrimSpace(author) == "" {
		return ProtectionBatch{}, assessment.ErrAuthorRequired
	}
	if len(b.Plans) > 0 && b.Plans[len(b.Plans)-1].ApprovedAt != nil && !b.PlanRefreshRequired && requested == nil {
		return clone(*b), nil
	}
	if b.Status == StatusApproved && !b.PlanRefreshRequired && requested == nil {
		return clone(*b), nil
	}
	if b.Status != StatusPending && b.Status != StatusDraft && !(requested != nil && b.Status == StatusApproved) {
		return ProtectionBatch{}, ErrInvalidState
	}
	if err := assessment.ValidateSamples(b.Samples); err != nil {
		w.auditLocked(b, "EVIDENCE_VALIDATION_FAILED", author, map[string]any{"sample_count": len(b.Samples)})
		return ProtectionBatch{}, err
	}
	for _, sample := range b.Samples {
		if sample.ValidatedAt.IsZero() {
			return ProtectionBatch{}, ErrEvidenceMissing
		}
	}
	if requested == nil && len(b.Plans) > 0 && !b.PlanRefreshRequired && b.Plans[len(b.Plans)-1].ApprovedAt == nil {
		plan := b.Plans[len(b.Plans)-1]
		if confirm {
			now := time.Now().UTC()
			plan.ApprovedAt = &now
			latest := time.Time{}
			for _, s := range b.Samples {
				if s.ValidatedAt.After(latest) {
					latest = s.ValidatedAt
				}
			}
			plan.SamplesValidatedAt = &latest
			plan.Author = author
			b.Plans[len(b.Plans)-1] = plan
			b.Status = StatusPending
			w.bump(b)
			w.auditLocked(b, "PLAN_VERSIONED", author, map[string]any{"version": plan.Version, "changed_fields": []string{}, "sample_count": len(b.Samples), "evidence_check": "passed", "approved_at": now})
			return clone(*b), nil
		}
		b.PlanPreview = &plan
		out := clone(*b)
		b.PlanPreview = nil
		return out, nil
	}
	var plan assessment.TreatmentPlan
	if requested != nil {
		plan = *requested
		plan.ID = fmt.Sprintf("plan-%s-v%d", id, len(b.Plans)+1)
		plan.BatchID, plan.Version, plan.Author = id, len(b.Plans)+1, author
		plan.ApprovedAt = nil
		if err := assessment.ValidatePlan(plan); err != nil {
			return ProtectionBatch{}, err
		}
	} else {
		var err error
		plan, err = assessment.GeneratePlan(id, author, b.Samples, len(b.Plans)+1)
		if err != nil {
			return ProtectionBatch{}, err
		}
	}
	if len(b.Plans) > 0 {
		plan.DiffFields = assessment.DiffPlan(b.Plans[len(b.Plans)-1], plan)
		if requested != nil && len(plan.DiffFields) == 0 && b.Plans[len(b.Plans)-1].ChlorideTarget == plan.ChlorideTarget && b.Plans[len(b.Plans)-1].SolutionProfile == plan.SolutionProfile {
			return clone(*b), nil
		}
		if len(plan.DiffFields) == 0 {
			plan.DiffFields = []string{"no_changes"}
		}
	}
	if err := assessment.ValidatePlan(plan); err != nil {
		return ProtectionBatch{}, err
	}
	if !confirm {
		preview := plan
		preview.ApprovedAt = nil
		b.PlanPreview = &preview
		out := clone(*b)
		b.PlanPreview = nil
		return out, nil
	}
	if confirm {
		now := time.Now().UTC()
		plan.ApprovedAt = &now
		latest := now
		for _, s := range b.Samples {
			if s.ValidatedAt.After(latest) {
				latest = s.ValidatedAt
			}
		}
		plan.SamplesValidatedAt = &latest
	}
	b.Plans = append(b.Plans, plan)
	b.PlanPreview = nil
	b.PlanRefreshRequired = false
	b.PlanRefreshSampleIDs = nil
	b.PlanRefreshFromVersion = 0
	if confirm {
		b.Status = StatusPending
	}
	w.bump(b)
	w.auditLocked(b, "PLAN_VERSIONED", author, map[string]any{"version": plan.Version, "changed_fields": plan.DiffFields, "sample_count": len(b.Samples), "evidence_check": "passed", "evidence_validated_at": time.Now().UTC()})
	return clone(*b), nil
}

func (w *Workflow) AssignTask(id string, expected int, phase int, assignee string, actor string) (ProtectionBatch, error) {
	return w.assignTask(id, expected, phase, assignee, actor, "", "", nil)
}

func (w *Workflow) AssignTaskWithRemediation(id string, expected int, phase int, assignee, actor, note, owner string) (ProtectionBatch, error) {
	return w.assignTask(id, expected, phase, assignee, actor, note, owner, nil)
}

func (w *Workflow) AssignTaskWithRemediationEvidence(id string, expected int, phase int, assignee, actor, note, owner string, evidence []string) (ProtectionBatch, error) {
	return w.assignTask(id, expected, phase, assignee, actor, note, owner, evidence)
}

func (w *Workflow) assignTask(id string, expected int, phase int, assignee string, actor string, remediationNote, remediationOwner string, remediationEvidence []string) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := w.checkLocked(id, expected)
	if err != nil {
		return ProtectionBatch{}, err
	}
	if b.Status != StatusPending && b.Status != StatusPaused {
		return ProtectionBatch{}, ErrInvalidState
	}
	if strings.TrimSpace(assignee) == "" {
		return ProtectionBatch{}, errors.New("assignee is required")
	}
	if b.LockedPhases[phase] {
		w.auditLocked(b, "PHASE_LOCKED", actor, map[string]any{"phase": phase, "reason": "task assignment rejected"})
		return ProtectionBatch{}, ErrPhaseLocked
	}
	if b.Status == StatusPaused {
		if strings.TrimSpace(remediationNote) == "" || strings.TrimSpace(remediationOwner) == "" || len(remediationEvidence) == 0 {
			return ProtectionBatch{}, ErrRemediationRequired
		}
		for _, ref := range remediationEvidence {
			if strings.TrimSpace(ref) == "" {
				return ProtectionBatch{}, ErrRemediationRequired
			}
		}
		if b.CurrentPhase != phase {
			return ProtectionBatch{}, ErrInvalidState
		}
		if b.PendingRemediation != nil {
			if strings.TrimSpace(remediationOwner) == strings.TrimSpace(b.PendingRemediation.TriggerOperator) {
				return ProtectionBatch{}, ErrRemediationConflict
			}
			b.PendingRemediation.Note, b.PendingRemediation.Owner = remediationNote, remediationOwner
			b.PendingRemediation.EvidenceRefs = append([]string(nil), remediationEvidence...)
		}
	}
	plan := b.Plans[len(b.Plans)-1]
	step, ok := assessment.StepFor(plan, phase)
	if !ok {
		return ProtectionBatch{}, errors.New("phase not found")
	}
	for _, task := range b.Tasks {
		if task.Phase == phase && b.Status != StatusPaused {
			return ProtectionBatch{}, errors.New("phase already has a current task")
		}
	}
	now := time.Now().UTC()
	b.Tasks = append(b.Tasks, treatment.Task{BatchID: id, Phase: phase, Assignee: assignee, StartedAt: now, DueAt: now.Add(time.Duration(step.Days) * 24 * time.Hour)})
	b.CurrentPhase = phase
	b.Status = StatusProcessing
	w.bump(b)
	detail := map[string]any{"phase": phase, "assignee": assignee}
	if remediationNote != "" {
		detail["remediation_note"], detail["remediation_owner"] = remediationNote, remediationOwner
		detail["evidence_refs"] = remediationEvidence
		if b.PendingRemediation == nil {
			kept := b.Reviews[:0]
			for _, decision := range b.Reviews {
				if decision.Phase != phase {
					kept = append(kept, decision)
				}
			}
			b.Reviews = kept
		}
	}
	w.auditLocked(b, "TASK_ASSIGNED", actor, detail)
	return clone(*b), nil
}

func (w *Workflow) AddObservation(id string, expected int, o treatment.ObservationRecord, actor string) (ProtectionBatch, error) {
	return w.AddObservations(id, expected, []treatment.ObservationRecord{o}, actor)
}

func (w *Workflow) AddObservations(id string, expected int, observations []treatment.ObservationRecord, actor string) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := w.checkLocked(id, expected)
	if err != nil {
		return ProtectionBatch{}, err
	}
	for _, o := range observations {
		if b.LockedPhases[o.Phase] {
			return ProtectionBatch{}, ErrPhaseLocked
		}
	}
	if b.Status != StatusProcessing {
		return ProtectionBatch{}, ErrInvalidState
	}
	plan := b.Plans[len(b.Plans)-1]
	validated := make([]treatment.ObservationRecord, 0, len(observations))
	seen := map[string]bool{}
	for i, o := range observations {
		var task *treatment.Task
		for j := range b.Tasks {
			if b.Tasks[j].Phase == o.Phase {
				task = &b.Tasks[j]
			}
		}
		if task == nil {
			return ProtectionBatch{}, errors.New("phase task not assigned")
		}
		if o.ObservedAt.IsZero() {
			o.ObservedAt = time.Now().UTC()
		}
		if o.ObservedAt.Before(task.StartedAt) {
			return ProtectionBatch{}, errors.New("observed_at before task start")
		}
		if o.ObservedAt.After(task.DueAt) {
			return ProtectionBatch{}, ErrDeadlineExceeded
		}
		key := fmt.Sprintf("%d:%s", o.Phase, o.ObservedAt.UTC().Format(time.RFC3339Nano))
		if seen[key] {
			return ProtectionBatch{}, ErrDuplicateObservation
		}
		for _, existing := range b.Observations {
			if existing.Phase == o.Phase && existing.ObservedAt.Equal(o.ObservedAt) {
				return ProtectionBatch{}, ErrDuplicateObservation
			}
		}
		anomaly, e := treatment.ValidateObservation(plan, o)
		if e != nil {
			return ProtectionBatch{}, e
		}
		if treatment.TrendReversal(append(append([]treatment.ObservationRecord(nil), b.Observations...), validated...), o.Phase, o) {
			if strings.TrimSpace(o.SampleNotes) == "" {
				return ProtectionBatch{}, ErrTrendNotesRequired
			}
			anomaly = "TREND_REVERSAL"
		}
		if anomaly != "" {
			count := 0
			for _, existing := range b.Observations {
				if existing.Phase == o.Phase && (existing.AnomalyCode == anomaly || existing.AnomalyCode == "REPEAT_ANOMALY" || (anomaly == "TREND_REVERSAL" && existing.AnomalyCode == "TREND_REVERSAL")) {
					count++
				}
			}
			for _, prior := range validated {
				if prior.Phase == o.Phase && prior.AnomalyCode == anomaly {
					count++
				}
			}
			if count > 0 {
				anomaly = "REPEAT_ANOMALY"
			}
		}
		o.ID = fmt.Sprintf("obs-%s-%d", id, len(b.Observations)+i+1)
		o.BatchID, o.Operator, o.AnomalyCode = id, actor, anomaly
		seen[key] = true
		validated = append(validated, o)
	}
	b.Observations = treatment.SortObservations(append(b.Observations, validated...))
	b.ObservationSummaries = treatment.SummariesForPlan(b.Observations, plan)
	w.bump(b)
	for phase, summary := range b.ObservationSummaries {
		summary.Revision = b.Revision
		b.ObservationSummaries[phase] = summary
	}
	var anomaly treatment.ObservationRecord
	for _, o := range validated {
		if o.AnomalyCode != "" {
			if anomaly.AnomalyCode == "" || o.AnomalyCode == "REPEAT_ANOMALY" {
				anomaly = o
			}
		}
	}
	if anomaly.AnomalyCode != "" {
		b.Status = StatusPaused
		b.PendingRemediation = &Remediation{Code: anomaly.AnomalyCode, Phase: anomaly.Phase, TriggerOperator: actor, Raw: map[string]any{"conductivity": anomaly.SolutionConductivity, "temperature": anomaly.Temperature, "ph": anomaly.PHValue}}
		typ := "ANOMALY_PAUSED"
		if anomaly.AnomalyCode == "REPEAT_ANOMALY" {
			typ = "ANOMALY_ESCALATED"
		}
		if anomaly.AnomalyCode == "TREND_REVERSAL" {
			typ = "TREND_REVERSAL"
		}
		w.auditLocked(b, typ, actor, map[string]any{"code": anomaly.AnomalyCode, "phase": anomaly.Phase, "conductivity": anomaly.SolutionConductivity, "temperature": anomaly.Temperature, "ph": anomaly.PHValue})
	} else {
		if b.PendingRemediation != nil {
			b.PendingRemediation = nil
			w.auditLocked(b, "REMEDIATION_VERIFIED", actor, map[string]any{"phase": validated[0].Phase})
		}
		w.auditLocked(b, "OBSERVATION_RECORDED", actor, map[string]any{"count": len(validated)})
	}
	return clone(*b), nil
}

func (w *Workflow) Complete(id string, expected int, phase int, actor string) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := w.checkLocked(id, expected)
	if err != nil {
		return ProtectionBatch{}, err
	}
	if strings.TrimSpace(actor) == "" {
		w.auditLocked(b, "READINESS_INCOMPLETE", actor, map[string]any{"phase": phase, "missing": []string{"actor"}})
		return ProtectionBatch{}, ErrActorRequired
	}
	if b.LockedPhases[phase] {
		w.auditLocked(b, "PHASE_LOCKED", actor, map[string]any{"phase": phase, "reason": "already locked"})
		return ProtectionBatch{}, ErrPhaseLocked
	}
	if b.Status != StatusProcessing {
		return ProtectionBatch{}, ErrInvalidState
	}
	var started time.Time
	var due time.Time
	assignee := ""
	for _, task := range b.Tasks {
		if task.Phase == phase {
			started = task.StartedAt
			due = task.DueAt
			assignee = task.Assignee
		}
	}
	if assignee != "" && assignee == actor {
		w.auditLocked(b, "READINESS_INCOMPLETE", actor, map[string]any{"phase": phase, "missing": []string{"independent_handoff"}})
		return ProtectionBatch{}, ErrHandoffRequired
	}
	step, ok := assessment.StepFor(b.Plans[len(b.Plans)-1], phase)
	if !ok {
		return ProtectionBatch{}, errors.New("phase not found")
	}
	missingDates := treatment.MissingObservationDates(b.Observations, phase, step.Days, started)
	checks := map[string]bool{
		"observation_coverage": len(missingDates) == 0,
		"task_completed":       !started.IsZero(),
		"within_deadline":      due.IsZero() || !time.Now().UTC().After(due),
		"anomaly_remediated":   b.PendingRemediation == nil,
		"evidence_index":       len(b.Samples) > 0,
	}
	if b.Readiness == nil {
		b.Readiness = map[int]map[string]bool{}
	}
	b.Readiness[phase] = checks
	var missing []string
	for name, ok := range checks {
		if !ok {
			missing = append(missing, name)
		}
	}
	for _, day := range missingDates {
		missing = append(missing, "observation_date:"+day)
	}
	if len(missing) > 0 {
		w.auditLocked(b, "READINESS_INCOMPLETE", actor, map[string]any{"phase": phase, "missing": missing})
		return ProtectionBatch{}, &ReadinessError{Missing: missing}
	}
	b.LockedPhases[phase] = true
	b.Status = StatusReview
	if b.ReadinessSnapshots == nil {
		b.ReadinessSnapshots = map[int]ReadinessSnapshot{}
	}
	dates := make([]string, 0)
	seenDates := map[string]bool{}
	for _, o := range b.Observations {
		if o.Phase == phase && !o.ObservedAt.Before(started) {
			date := o.ObservedAt.UTC().Format("2006-01-02")
			if !seenDates[date] {
				dates = append(dates, date)
				seenDates[date] = true
			}
		}
	}
	evidenceCount := 0
	for _, sample := range b.Samples {
		evidenceCount += len(sample.EvidenceRefs)
	}
	b.ReadinessSnapshots[phase] = ReadinessSnapshot{Phase: phase, CoverageDates: dates, AnomalyResult: "clear", PlanVersion: b.Plans[len(b.Plans)-1].Version, SampleEvidenceCount: evidenceCount, CapturedRevision: b.Revision + 1}
	w.bump(b)
	w.auditLocked(b, "PHASE_LOCKED", actor, map[string]any{"phase": phase, "readiness": checks})
	return clone(*b), nil
}

func (w *Workflow) AddReview(id string, expected int, d review.ReviewDecision, actor string) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := w.checkLocked(id, expected)
	if err != nil {
		return ProtectionBatch{}, err
	}
	if b.Status != StatusReview {
		return ProtectionBatch{}, ErrInvalidState
	}
	if err := review.ValidateDecision(d, b.Reviews); err != nil {
		return ProtectionBatch{}, err
	}
	allowed := map[string]bool{}
	for _, sample := range b.Samples {
		for _, ref := range sample.EvidenceRefs {
			allowed[strings.TrimSpace(ref)] = true
		}
	}
	covered := make([]string, 0)
	for _, ref := range d.EvidenceRefs {
		if !allowed[ref] {
			return ProtectionBatch{}, fmt.Errorf("%w: %s", ErrReviewEvidence, ref)
		}
		covered = append(covered, ref)
	}
	if len(b.Reviews) > 0 {
		prior := map[string]bool{}
		returned := false
		for _, prev := range b.Reviews {
			if prev.Phase == d.Phase && prev.Decision == "return" {
				returned = true
				for _, ref := range prev.EvidenceRefs {
					prior[ref] = true
				}
			}
		}
		if returned {
			newEvidence := false
			var evidenceDiff []string
			for _, ref := range d.EvidenceRefs {
				if !prior[ref] {
					newEvidence = true
					evidenceDiff = append(evidenceDiff, ref)
				}
			}
			if !newEvidence {
				return ProtectionBatch{}, errors.New("new evidence is required after remediation")
			}
			if b.ReviewSummaries == nil {
				b.ReviewSummaries = map[int]review.ReviewSummary{}
			}
			summary := b.ReviewSummaries[d.Phase]
			summary.EvidenceDiff = evidenceDiff
			b.ReviewSummaries[d.Phase] = summary
		}
	}
	d.ID = fmt.Sprintf("review-%s-%d", id, len(b.Reviews)+1)
	d.BatchID = id
	d.ReviewedAt = time.Now().UTC()
	b.Reviews = append(b.Reviews, d)
	if b.ReviewSummaries == nil {
		b.ReviewSummaries = map[int]review.ReviewSummary{}
	}
	summary := review.ReviewSummary{Phase: d.Phase, Required: 2, CoveredRefs: covered}
	for _, existing := range b.Reviews {
		if existing.Phase == d.Phase {
			summary.Collected++
		}
	}
	for ref := range allowed {
		found := false
		for _, item := range b.Reviews {
			if item.Phase != d.Phase {
				continue
			}
			for _, evidence := range item.EvidenceRefs {
				if evidence == ref {
					found = true
				}
			}
		}
		if found {
			summary.CoveredRefs = appendUnique(summary.CoveredRefs, ref)
		} else {
			summary.MissingRefs = append(summary.MissingRefs, ref)
		}
	}
	b.ReviewSummaries[d.Phase] = summary
	result, _ := review.Result(b.Reviews, b.CurrentPhase)
	if result == "approve" {
		b.Status = StatusApproved
	} else if result == "return" {
		b.Status = StatusReview
		b.LockedPhases[d.Phase] = false
		w.auditLocked(b, "REVIEW_RETURNED", actor, map[string]any{"phase": d.Phase, "reviewer": d.Reviewer, "findings": d.Findings})
	}
	w.bump(b)
	w.auditLocked(b, "REVIEW_RECORDED", actor, map[string]any{"reviewer": d.Reviewer, "decision": d.Decision})
	return clone(*b), nil
}

func (w *Workflow) Archive(id string, expected int, actor string, evidence []string) (ProtectionBatch, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := w.checkLocked(id, expected)
	if err != nil {
		return ProtectionBatch{}, err
	}
	if existing, ok := w.archive.Archive(id); ok && b.Status == StatusArchived {
		b.Archive = &existing
		return clone(*b), nil
	}
	if b.Status != StatusApproved {
		return ProtectionBatch{}, ErrInvalidState
	}
	if len(evidence) == 0 {
		return ProtectionBatch{}, ErrArchiveEvidence
	}
	allowed := map[string]bool{}
	for _, s := range b.Samples {
		for _, ref := range s.EvidenceRefs {
			allowed[strings.TrimSpace(ref)] = true
		}
	}
	for _, d := range b.Reviews {
		for _, ref := range d.EvidenceRefs {
			allowed[strings.TrimSpace(ref)] = true
		}
	}
	seen := map[string]bool{}
	for i, ref := range evidence {
		evidence[i] = strings.TrimSpace(ref)
		if evidence[i] == "" || seen[evidence[i]] || !allowed[evidence[i]] {
			return ProtectionBatch{}, ErrArchiveEvidence
		}
		seen[evidence[i]] = true
	}
	finalRevision := b.Revision + 1
	r, err := w.archive.CreateArchive(id, finalRevision, evidence, actor)
	if err != nil {
		return ProtectionBatch{}, err
	}
	b.Archive = &r
	b.Status = StatusArchived
	w.bump(b)
	w.auditLocked(b, "ARCHIVED", actor, map[string]any{"digest": r.RecordDigest})
	return clone(*b), nil
}

func (w *Workflow) Audit(id string) ([]archive.AuditEvent, error) {
	if _, err := w.Get(id); err != nil {
		return nil, err
	}
	return w.archive.Events(id), nil
}

func (w *Workflow) AuditBetween(id string, from, to *time.Time) ([]archive.AuditEvent, error) {
	if _, err := w.Get(id); err != nil {
		return nil, err
	}
	return w.archive.EventsBetween(id, from, to), nil
}

func (w *Workflow) AuditPage(id string, from, to *time.Time, typ string, limit int, cursor string) (AuditPage, error) {
	if _, err := w.Get(id); err != nil {
		return AuditPage{}, err
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return AuditPage{}, ErrInvalidLimit
	}
	start := 0
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil || n < 0 {
			return AuditPage{}, ErrInvalidCursor
		}
		start = n
	}
	events := w.archive.EventsBetween(id, from, to)
	if typ != "" {
		filtered := events[:0]
		for _, event := range events {
			if event.Type == typ {
				filtered = append(filtered, event)
			}
		}
		events = filtered
	}
	if start > len(events) {
		return AuditPage{}, ErrInvalidCursor
	}
	end := start + limit
	if end > len(events) {
		end = len(events)
	}
	page := AuditPage{Events: append([]archive.AuditEvent(nil), events[start:end]...), TypeCounts: map[string]int{}, RevisionRange: map[string]int{}}
	for _, event := range events {
		page.TypeCounts[event.Type]++
		if min, ok := page.RevisionRange["min"]; !ok || event.Revision < min {
			page.RevisionRange["min"] = event.Revision
		}
		if max, ok := page.RevisionRange["max"]; !ok || event.Revision > max {
			page.RevisionRange["max"] = event.Revision
		}
	}
	if end < len(events) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}
