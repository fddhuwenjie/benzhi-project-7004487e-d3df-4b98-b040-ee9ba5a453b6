package archivefailurecommitsstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/assessment"
	"museum-desalination/internal/review"
	"museum-desalination/internal/treatment"
	"museum-desalination/internal/workflow"
)

func TestArchivePersistenceFailureDoesNotCommitWorkflowState(t *testing.T) {
	dir := t.TempDir()
	store := archive.NewStore(dir)
	wf := workflow.New(store)

	sample := assessment.Sample{
		MaterialType:    "paper",
		LocationCode:    "case-1",
		InitialChloride: 80,
		MoistureRatio:   0.3,
		ConditionGrade:  "A",
		EvidenceRefs:    []string{"lab-report"},
	}
	batch, err := wf.Create(workflow.CreateRequest{
		CatalogRef:      "archive-atomicity",
		Title:           "归档原子性复现",
		ResponsibleUser: "conservator",
		Samples:         []assessment.Sample{sample},
	}, "archive-atomicity-create")
	if err != nil {
		t.Fatal(err)
	}
	batch, err = wf.AssignTask(batch.ID, batch.Revision, 1, "operator", "conservator")
	if err != nil {
		t.Fatal(err)
	}
	started := batch.Tasks[0].StartedAt
	observations := []treatment.ObservationRecord{
		{Phase: 1, ObservedAt: started.Add(time.Hour), SolutionConductivity: 500, PHValue: 7, Temperature: 20},
		{Phase: 1, ObservedAt: started.Add(25 * time.Hour), SolutionConductivity: 490, PHValue: 7, Temperature: 20},
		{Phase: 1, ObservedAt: started.Add(49 * time.Hour), SolutionConductivity: 480, PHValue: 7, Temperature: 20},
	}
	batch, err = wf.AddObservations(batch.ID, batch.Revision, observations, "operator")
	if err != nil {
		t.Fatal(err)
	}
	batch, err = wf.Complete(batch.ID, batch.Revision, 1, "handoff-lead")
	if err != nil {
		t.Fatal(err)
	}
	batch, err = wf.AddReview(batch.ID, batch.Revision, review.ReviewDecision{
		Phase: 1, Reviewer: "reviewer-a", Decision: "approve", Findings: "通过", EvidenceRefs: []string{"lab-report"},
	}, "reviewer-a")
	if err != nil {
		t.Fatal(err)
	}
	batch, err = wf.AddReview(batch.ID, batch.Revision, review.ReviewDecision{
		Phase: 1, Reviewer: "reviewer-b", Decision: "approve", Findings: "通过", EvidenceRefs: []string{"lab-report"},
	}, "reviewer-b")
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != workflow.StatusApproved {
		t.Fatalf("precondition: status=%s", batch.Status)
	}

	// persistLocked writes this fixed temporary path before rename. Replacing it
	// with a directory deterministically makes the archive write return EISDIR.
	if err := os.Mkdir(filepath.Join(dir, "snapshot.json.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	approvedRevision := batch.Revision
	if _, err := wf.Archive(batch.ID, batch.Revision, "archivist", []string{"lab-report"}); err == nil {
		t.Fatal("expected archive persistence failure")
	}

	_, cachedArchiveExists := store.Archive(batch.ID)
	live, err := wf.Get(batch.ID)
	if err != nil {
		t.Fatal(err)
	}

	restartedStore := archive.NewStore(dir)
	if err := restartedStore.Load(); err != nil {
		t.Fatal(err)
	}
	_, persistedArchiveExists := restartedStore.Archive(batch.ID)
	restarted := workflow.New(restartedStore)
	recovered, err := restarted.Get(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cachedArchiveExists || persistedArchiveExists || live.Status != workflow.StatusApproved || live.Revision != approvedRevision || live.Archive != nil || recovered.Status != workflow.StatusApproved || recovered.Revision != approvedRevision {
		t.Fatalf("failed archive committed state: cached=%t persisted=%t live=%s/%d restarted=%s/%d archive=%+v", cachedArchiveExists, persistedArchiveExists, live.Status, live.Revision, recovered.Status, recovered.Revision, live.Archive)
	}
}
