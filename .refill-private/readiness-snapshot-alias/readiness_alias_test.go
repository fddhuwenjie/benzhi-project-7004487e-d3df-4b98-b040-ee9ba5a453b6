package readiness_snapshot_alias_test

import (
	"testing"
	"time"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/assessment"
	"museum-desalination/internal/treatment"
	"museum-desalination/internal/workflow"
)

func TestReadinessSnapshotMutationDoesNotPolluteWorkflow(t *testing.T) {
	store := archive.NewStore(t.TempDir())
	wf := workflow.New(store)
	batch, err := wf.Create(workflow.CreateRequest{
		CatalogRef:      "alias-case",
		Title:           "阶段锁定快照隔离复现",
		ResponsibleUser: "planner",
		Samples: []assessment.Sample{{
			MaterialType:    "paper",
			LocationCode:    "zone-a",
			InitialChloride: 80,
			MoistureRatio:   0.3,
			ConditionGrade:  "A",
			EvidenceRefs:    []string{"lab-1"},
		}},
	}, "alias-create")
	if err != nil {
		t.Fatal(err)
	}

	batch, err = wf.AssignTask(batch.ID, batch.Revision, 1, "operator", "planner")
	if err != nil {
		t.Fatal(err)
	}
	started := batch.Tasks[0].StartedAt
	observations := []treatment.ObservationRecord{
		{Phase: 1, ObservedAt: started.Add(time.Minute), SolutionConductivity: 500, PHValue: 7, Temperature: 20},
		{Phase: 1, ObservedAt: started.Add(24*time.Hour + time.Minute), SolutionConductivity: 450, PHValue: 7, Temperature: 20},
		{Phase: 1, ObservedAt: started.Add(48*time.Hour + time.Minute), SolutionConductivity: 400, PHValue: 7, Temperature: 20},
	}
	batch, err = wf.AddObservations(batch.ID, batch.Revision, observations, "operator")
	if err != nil {
		t.Fatal(err)
	}
	batch, err = wf.Complete(batch.ID, batch.Revision, 1, "supervisor")
	if err != nil {
		t.Fatal(err)
	}
	if !batch.Readiness[1]["observation_coverage"] {
		t.Fatal("precondition: completed phase is not ready")
	}

	batch.Readiness[1]["observation_coverage"] = false
	current, err := wf.Get(batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !current.Readiness[1]["observation_coverage"] {
		t.Fatalf("returned readiness snapshot mutated workflow state: status=%s locked=%v readiness=%v", current.Status, current.LockedPhases[1], current.Readiness[1])
	}
}
