package assessment

import "testing"

func TestValidateSampleAndGeneratePlan(t *testing.T) {
	s := Sample{MaterialType: "stone", LocationCode: "A", InitialChloride: 100, MoistureRatio: .4, ConditionGrade: "B", EvidenceRefs: []string{"e1"}}
	if err := ValidateSample(s); err != nil {
		t.Fatal(err)
	}
	p, err := GeneratePlan("b1", "u1", []Sample{s}, 1)
	if err != nil || len(p.PhaseSteps) != 3 {
		t.Fatalf("plan=%+v err=%v", p, err)
	}
}

func TestMissingEvidenceRejected(t *testing.T) {
	s := Sample{MaterialType: "stone", LocationCode: "A", InitialChloride: 1, MoistureRatio: .1, ConditionGrade: "A"}
	if err := ValidateSample(s); err != ErrEvidenceMissing {
		t.Fatalf("expected evidence error, got %v", err)
	}
}
