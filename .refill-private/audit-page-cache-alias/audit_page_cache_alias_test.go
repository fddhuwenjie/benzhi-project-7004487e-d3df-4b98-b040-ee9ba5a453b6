package auditpagecachealias_test

import (
	"sync"
	"testing"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/assessment"
	"museum-desalination/internal/workflow"
)

func TestConcurrentAuditPageResponsesDoNotShareState(t *testing.T) {
	wf := workflow.New(archive.NewStore(t.TempDir()))
	batch, err := wf.Create(workflow.CreateRequest{
		CatalogRef:      "audit-cache-alias",
		Title:           "审计分页缓存所有权复现",
		ResponsibleUser: "owner",
		Samples: []assessment.Sample{{
			MaterialType:    "stone",
			LocationCode:    "A-01",
			InitialChloride: 20,
			MoistureRatio:   0.2,
			ConditionGrade:  "A",
			EvidenceRefs:    []string{"lab-001"},
		}},
	}, "audit-cache-alias-key")
	if err != nil {
		t.Fatal(err)
	}

	first, err := wf.AuditPage(batch.ID, nil, nil, "", 50, "")
	if err != nil || len(first.Events) == 0 {
		t.Fatalf("prime audit page cache: events=%d err=%v", len(first.Events), err)
	}
	second, err := wf.AuditPage(batch.ID, nil, nil, "", 50, "")
	if err != nil {
		t.Fatal(err)
	}
	originalActor := first.Events[0].Actor

	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	var writers sync.WaitGroup
	writers.Add(2)
	mutate := func(page *workflow.AuditPage, actor string) {
		defer writers.Done()
		ready <- struct{}{}
		<-start
		page.Events[0].Actor = actor
	}
	go mutate(&first, "tampered-a")
	go mutate(&second, "tampered-b")
	<-ready
	<-ready
	close(start)
	writers.Wait()

	after, err := wf.AuditPage(batch.ID, nil, nil, "", 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if after.Events[0].Actor != originalActor {
		t.Fatalf("cached audit page was polluted: got actor %q, want %q", after.Events[0].Actor, originalActor)
	}
}
