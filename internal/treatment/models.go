package treatment

import "time"

type ObservationRecord struct {
	ID                   string    `json:"id"`
	BatchID              string    `json:"batch_id"`
	Phase                int       `json:"phase"`
	ObservedAt           time.Time `json:"observed_at"`
	SolutionConductivity float64   `json:"solution_conductivity"`
	PHValue              float64   `json:"ph_value"`
	Temperature          float64   `json:"temperature"`
	SampleNotes          string    `json:"sample_notes"`
	AnomalyCode          string    `json:"anomaly_code,omitempty"`
	Operator             string    `json:"operator"`
}

type ObservationSummary struct {
	Phase              int     `json:"phase"`
	Revision           int     `json:"revision"`
	Count              int     `json:"count"`
	FirstConductivity  float64 `json:"first_conductivity"`
	LastConductivity   float64 `json:"last_conductivity"`
	TrendDirection     string  `json:"trend_direction"`
	AnomalyCount       int     `json:"anomaly_count"`
	TrendReversal      bool    `json:"trend_reversal"`
	LatestConductivity float64 `json:"latest_conductivity"`
	LatestPH           float64 `json:"latest_ph"`
	LatestTemperature  float64 `json:"latest_temperature"`
	MinConductivity    float64 `json:"min_conductivity"`
	MaxConductivity    float64 `json:"max_conductivity"`
	MinPH              float64 `json:"min_ph"`
	MaxPH              float64 `json:"max_ph"`
	MinTemperature     float64 `json:"min_temperature"`
	MaxTemperature     float64 `json:"max_temperature"`
}

type Task struct {
	BatchID   string    `json:"batch_id"`
	Phase     int       `json:"phase"`
	Assignee  string    `json:"assignee"`
	DueAt     time.Time `json:"due_at"`
	StartedAt time.Time `json:"started_at"`
}
