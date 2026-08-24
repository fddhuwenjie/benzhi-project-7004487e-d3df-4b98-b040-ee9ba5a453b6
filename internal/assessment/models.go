package assessment

import "time"

type Sample struct {
	ID              string    `json:"id"`
	BatchID         string    `json:"batch_id"`
	MaterialType    string    `json:"material_type"`
	LocationCode    string    `json:"location_code"`
	InitialChloride float64   `json:"initial_chloride"`
	MoistureRatio   float64   `json:"moisture_ratio"`
	ConditionGrade  string    `json:"condition_grade"`
	EvidenceRefs    []string  `json:"evidence_refs"`
	ValidatedAt     time.Time `json:"validated_at"`
}

type PhaseStep struct {
	Phase             int     `json:"phase"`
	Days              int     `json:"days"`
	Solution          string  `json:"solution"`
	Target            float64 `json:"target"`
	ConductivityLimit float64 `json:"conductivity_limit"`
	TemperatureLimit  float64 `json:"temperature_limit"`
	PHMin             float64 `json:"ph_min"`
	PHMax             float64 `json:"ph_max"`
}

type TreatmentPlan struct {
	ID                 string      `json:"id"`
	BatchID            string      `json:"batch_id"`
	Version            int         `json:"version"`
	PhaseSteps         []PhaseStep `json:"phase_steps"`
	SolutionProfile    string      `json:"solution_profile"`
	TemperatureLimit   float64     `json:"temperature_limit"`
	ChlorideTarget     float64     `json:"chloride_target"`
	Author             string      `json:"author"`
	ApprovedAt         *time.Time  `json:"approved_at,omitempty"`
	SamplesValidatedAt *time.Time  `json:"samples_validated_at,omitempty"`
	DiffFields         []string    `json:"diff_fields,omitempty"`
}
