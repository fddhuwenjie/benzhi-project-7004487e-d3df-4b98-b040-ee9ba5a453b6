package readiness_summary_slice_alias

import (
	"testing"
	"time"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/assessment"
	"museum-desalination/internal/review"
	"museum-desalination/internal/treatment"
	"museum-desalination/internal/workflow"
)

func TestWorkflowSnapshotNestedSlicesDoNotPolluteState(t *testing.T) {
	wf := workflow.New(archive.NewStore(t.TempDir()))
	b, err := wf.Create(workflow.CreateRequest{
		CatalogRef: "nested-alias", Title: "T", ResponsibleUser: "u",
		Samples: []assessment.Sample{{
			MaterialType: "paper", LocationCode: "P-1", InitialChloride: 80,
			MoistureRatio: .3, ConditionGrade: "A", EvidenceRefs: []string{"lab"},
		}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err = wf.AssignTask(b.ID, b.Revision, 1, "op", "u")
	if err != nil {
		t.Fatal(err)
	}
	start := b.Tasks[0].StartedAt
	b, err = wf.AddObservations(b.ID, b.Revision, []treatment.ObservationRecord{
		{Phase: 1, ObservedAt: start.Add(time.Hour), SolutionConductivity: 500, PHValue: 7, Temperature: 20},
		{Phase: 1, ObservedAt: start.Add(24*time.Hour + time.Hour), SolutionConductivity: 490, PHValue: 7, Temperature: 20},
		{Phase: 1, ObservedAt: start.Add(48*time.Hour + time.Hour), SolutionConductivity: 480, PHValue: 7, Temperature: 20},
	}, "op")
	if err != nil {
		t.Fatal(err)
	}
	b, err = wf.Complete(b.ID, b.Revision, 1, "u")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.ReadinessSnapshots[1].CoverageDates) == 0 {
		t.Fatalf("readiness snapshot missing dates: %+v", b.ReadinessSnapshots)
	}
	b, err = wf.AddReview(b.ID, b.Revision, review.ReviewDecision{
		Phase: 1, Reviewer: "r1", Decision: "approve", Findings: "ok", EvidenceRefs: []string{"lab"},
	}, "r1")
	if err != nil {
		t.Fatal(err)
	}
	b, err = wf.AddReview(b.ID, b.Revision, review.ReviewDecision{
		Phase: 1, Reviewer: "r2", Decision: "approve", Findings: "ok", EvidenceRefs: []string{"lab"},
	}, "r2")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.ReviewSummaries[1].CoveredRefs) == 0 {
		t.Fatalf("review summary missing covered refs: %+v", b.ReviewSummaries)
	}
	b, err = wf.Archive(b.ID, b.Revision, "arch", []string{"lab"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Archive == nil || len(b.Archive.EvidenceIndex) == 0 {
		t.Fatalf("archive missing evidence: %+v", b.Archive)
	}

	// Nested slices in a response are caller-owned and must not alias workflow state.
	b.ReadinessSnapshots[1].CoverageDates[0] = "tampered-date"
	b.ReviewSummaries[1].CoveredRefs[0] = "tampered-ref"
	b.Archive.EvidenceIndex[0] = "tampered-archive"
	got, err := wf.Get(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReadinessSnapshots[1].CoverageDates[0] == "tampered-date" || got.ReviewSummaries[1].CoveredRefs[0] == "tampered-ref" || got.Archive.EvidenceIndex[0] == "tampered-archive" {
		t.Fatalf("nested response mutation polluted workflow state: snapshots=%+v summaries=%+v archive=%+v", got.ReadinessSnapshots, got.ReviewSummaries, got.Archive)
	}
}
