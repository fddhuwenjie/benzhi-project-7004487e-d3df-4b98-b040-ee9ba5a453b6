package workflow

import (
	"errors"
	"testing"
	"time"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/assessment"
	"museum-desalination/internal/review"
	"museum-desalination/internal/treatment"
)

func newWorkflow(t *testing.T) *Workflow { t.Helper(); return New(archive.NewStore(t.TempDir())) }
func sample() assessment.Sample {
	return assessment.Sample{MaterialType: "paper", LocationCode: "P-1", InitialChloride: 80, MoistureRatio: .3, ConditionGrade: "A", EvidenceRefs: []string{"lab"}}
}

func TestLifecycleAndRevisionConflict(t *testing.T) {
	w := newWorkflow(t)
	b, err := w.Create(CreateRequest{CatalogRef: "C", Title: "T", ResponsibleUser: "u", Samples: []assessment.Sample{sample()}}, "k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.AssignTask(b.ID, b.Revision, 1, "op", "u"); err != nil {
		t.Fatal(err)
	}
	cur, _ := w.Get(b.ID)
	start := time.Now().UTC()
	if _, err = w.AddObservations(b.ID, cur.Revision, []treatment.ObservationRecord{{Phase: 1, ObservedAt: start, SolutionConductivity: 500, PHValue: 7, Temperature: 20, Operator: "op"}, {Phase: 1, ObservedAt: start.Add(24 * time.Hour), SolutionConductivity: 490, PHValue: 7, Temperature: 20, Operator: "op"}, {Phase: 1, ObservedAt: start.Add(48 * time.Hour), SolutionConductivity: 480, PHValue: 7, Temperature: 20, Operator: "op"}}, "op"); err != nil {
		t.Fatal(err)
	}
	cur, _ = w.Get(b.ID)
	if _, err = w.Complete(b.ID, cur.Revision, 1, "u"); err != nil {
		t.Fatal(err)
	}
	cur, _ = w.Get(b.ID)
	if _, err = w.AddReview(b.ID, cur.Revision, review.ReviewDecision{Phase: 1, Reviewer: "a", Decision: "approve", EvidenceRefs: []string{"lab"}}, "a"); err != nil {
		t.Fatal(err)
	}
	cur, _ = w.Get(b.ID)
	if _, err = w.AddReview(b.ID, cur.Revision, review.ReviewDecision{Phase: 1, Reviewer: "a", Decision: "approve", EvidenceRefs: []string{"lab"}}, "a"); !errors.Is(err, review.ErrDuplicateReviewer) {
		t.Fatalf("expected duplicate reviewer, got %v", err)
	}
	if _, err = w.AddReview(b.ID, cur.Revision, review.ReviewDecision{Phase: 1, Reviewer: "b", Decision: "approve", EvidenceRefs: []string{"lab"}}, "b"); err != nil {
		t.Fatal(err)
	}
	cur, _ = w.Get(b.ID)
	final, err := w.Archive(b.ID, cur.Revision, "arch", []string{"lab"})
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != StatusArchived {
		t.Fatalf("status %s", final.Status)
	}
	if final.Archive == nil || final.Archive.FinalRevision != final.Revision {
		t.Fatalf("archive revision mismatch: archive=%+v batch=%d", final.Archive, final.Revision)
	}
	if _, err = w.Get(b.ID); err != nil {
		t.Fatal(err)
	}
}

func TestAnomalyPausesBatch(t *testing.T) {
	w := newWorkflow(t)
	b, _ := w.Create(CreateRequest{CatalogRef: "C", Title: "T", ResponsibleUser: "u", Samples: []assessment.Sample{sample()}}, "")
	b, _ = w.AssignTask(b.ID, b.Revision, 1, "op", "u")
	b, err := w.AddObservation(b.ID, b.Revision, treatment.ObservationRecord{Phase: 1, SolutionConductivity: 5000, PHValue: 7, Temperature: 20, Operator: "op"}, "op")
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != StatusPaused {
		t.Fatalf("status %s", b.Status)
	}
}

func TestSampleRefreshAndPlanConfirmation(t *testing.T) {
	w := newWorkflow(t)
	b, _ := w.Create(CreateRequest{CatalogRef: "refresh", Title: "T", ResponsibleUser: "u", Samples: []assessment.Sample{sample()}}, "")
	b, err := w.ConfirmPlan(b.ID, b.Revision, "u", true, nil)
	if err != nil || b.Plans[0].ApprovedAt == nil {
		t.Fatalf("confirm: %+v %v", b.Plans, err)
	}
	added := sample()
	added.LocationCode, added.InitialChloride = "P-2", 120
	b, err = w.AddSample(b.ID, b.Revision, added, "u")
	if err != nil || !b.PlanRefreshRequired || b.PlanRefreshFromVersion != 1 {
		t.Fatalf("refresh marker: %+v %v", b, err)
	}
	oldRevision := b.Revision
	if _, err = w.AddSample(b.ID, b.Revision, added, "u"); err == nil {
		t.Fatal("duplicate location accepted")
	}
	cur, _ := w.Get(b.ID)
	if cur.Revision != oldRevision {
		t.Fatalf("revision changed: %d", cur.Revision)
	}
	b, err = w.ConfirmPlan(b.ID, b.Revision, "u", true, nil)
	if err != nil || b.Plans[len(b.Plans)-1].Version != 2 || len(b.Plans[len(b.Plans)-1].DiffFields) == 0 || b.PlanRefreshRequired {
		t.Fatalf("refreshed plan: %+v %v", b, err)
	}
}

func TestObservationBatchIsAtomic(t *testing.T) {
	w := newWorkflow(t)
	b, _ := w.Create(CreateRequest{CatalogRef: "atomic", Title: "T", ResponsibleUser: "u", Samples: []assessment.Sample{sample()}}, "")
	b, _ = w.AssignTask(b.ID, b.Revision, 1, "op", "u")
	start := b.Tasks[0].StartedAt.Add(time.Minute)
	b, err := w.AddObservation(b.ID, b.Revision, treatment.ObservationRecord{Phase: 1, ObservedAt: start, SolutionConductivity: 500, PHValue: 7, Temperature: 20}, "op")
	if err != nil {
		t.Fatal(err)
	}
	revision := b.Revision
	_, err = w.AddObservations(b.ID, b.Revision, []treatment.ObservationRecord{{Phase: 1, ObservedAt: start.Add(time.Hour), SolutionConductivity: 400, PHValue: 7, Temperature: 20}, {Phase: 1, ObservedAt: start, SolutionConductivity: 390, PHValue: 7, Temperature: 20}}, "op")
	if !errors.Is(err, ErrDuplicateObservation) {
		t.Fatalf("expected duplicate, got %v", err)
	}
	cur, _ := w.Get(b.ID)
	if cur.Revision != revision || len(cur.Observations) != 1 {
		t.Fatalf("partial write: revision=%d count=%d", cur.Revision, len(cur.Observations))
	}
}

func TestRemediationRequiresIndependentOwnerAndEvidence(t *testing.T) {
	w := newWorkflow(t)
	b, _ := w.Create(CreateRequest{CatalogRef: "remediation", Title: "T", ResponsibleUser: "u", Samples: []assessment.Sample{sample()}}, "")
	b, _ = w.AssignTask(b.ID, b.Revision, 1, "op", "u")
	b, _ = w.AddObservation(b.ID, b.Revision, treatment.ObservationRecord{Phase: 1, SolutionConductivity: 5000, PHValue: 7, Temperature: 20}, "op")
	if _, err := w.AssignTaskWithRemediationEvidence(b.ID, b.Revision, 1, "op2", "u", "更换溶液", "op", []string{"retest"}); !errors.Is(err, ErrRemediationConflict) {
		t.Fatalf("owner conflict: %v", err)
	}
	b, err := w.AssignTaskWithRemediationEvidence(b.ID, b.Revision, 1, "op2", "u", "更换溶液", "lead", []string{"retest"})
	if err != nil {
		t.Fatal(err)
	}
	b, err = w.AddObservation(b.ID, b.Revision, treatment.ObservationRecord{Phase: 1, SolutionConductivity: 400, PHValue: 7, Temperature: 20}, "op2")
	if err != nil || b.Status != StatusProcessing || b.PendingRemediation != nil {
		t.Fatalf("retest: status=%s remediation=%+v err=%v", b.Status, b.PendingRemediation, err)
	}
}

func TestListFiltered(t *testing.T) {
	w := newWorkflow(t)
	b, _ := w.Create(CreateRequest{CatalogRef: "list", Title: "T", ResponsibleUser: "u", Samples: []assessment.Sample{sample()}}, "")
	b, _ = w.AssignTask(b.ID, b.Revision, 1, "op", "u")
	b, _ = w.AddObservation(b.ID, b.Revision, treatment.ObservationRecord{Phase: 1, SolutionConductivity: 5000, PHValue: 7, Temperature: 20}, "op")
	result, err := w.ListFiltered(BatchFilter{Status: StatusPaused, CurrentPhase: "1"})
	if err != nil || len(result.Batches) != 1 || result.StatusCounts[StatusPaused] != 1 || result.Batches[0].PlanVersion != 1 {
		t.Fatalf("filter: %+v %v", result, err)
	}
}
