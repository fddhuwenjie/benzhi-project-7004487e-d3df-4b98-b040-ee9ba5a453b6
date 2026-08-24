package treatment

import (
	"fmt"
	"sort"
	"time"

	"museum-desalination/internal/assessment"
)

func ValidateObservation(plan assessment.TreatmentPlan, o ObservationRecord) (string, error) {
	step, ok := assessment.StepFor(plan, o.Phase)
	if !ok {
		return "invalid_phase", fmt.Errorf("phase %d not found", o.Phase)
	}
	if o.SolutionConductivity < 0 || o.Temperature < -10 || o.Temperature > 60 || o.PHValue < 0 || o.PHValue > 14 {
		return "invalid_reading", fmt.Errorf("reading out of physical range")
	}
	if o.SolutionConductivity > step.ConductivityLimit {
		return "HIGH_CONDUCTIVITY", nil
	}
	if o.Temperature > step.TemperatureLimit {
		return "HIGH_TEMPERATURE", nil
	}
	if o.PHValue < step.PHMin || o.PHValue > step.PHMax {
		return "PH_OUT_OF_RANGE", nil
	}
	return "", nil
}

func SortObservations(in []ObservationRecord) []ObservationRecord {
	out := append([]ObservationRecord(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ObservedAt.Before(out[j].ObservedAt) })
	return out
}

// TrendReversal reports whether the newest observation completes two adjacent
// conductivity increases within the same phase.
func TrendReversal(observations []ObservationRecord, phase int, candidate ObservationRecord) bool {
	items := make([]ObservationRecord, 0, len(observations)+1)
	for _, o := range observations {
		if o.Phase == phase {
			items = append(items, o)
		}
	}
	items = append(items, candidate)
	items = SortObservations(items)
	if len(items) < 3 {
		return false
	}
	n := len(items)
	return items[n-2].SolutionConductivity > items[n-3].SolutionConductivity && items[n-1].SolutionConductivity > items[n-2].SolutionConductivity
}

func Coverage(observations []ObservationRecord, phase int, since time.Time) bool {
	count := 0
	for _, o := range observations {
		if o.Phase == phase && !o.ObservedAt.Before(since) {
			count++
		}
	}
	return count >= 1
}

func MissingObservationDates(observations []ObservationRecord, phase, days int, started time.Time) []string {
	if started.IsZero() || days <= 0 {
		return []string{"observation_coverage"}
	}
	seen := map[string]bool{}
	for _, o := range observations {
		if o.Phase == phase && !o.ObservedAt.Before(started) {
			seen[o.ObservedAt.UTC().Format("2006-01-02")] = true
		}
	}
	missing := make([]string, 0)
	day := time.Date(started.UTC().Year(), started.UTC().Month(), started.UTC().Day(), 0, 0, 0, 0, time.UTC)
	for i := 0; i < days; i++ {
		key := day.AddDate(0, 0, i).Format("2006-01-02")
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	return missing
}

func Summaries(observations []ObservationRecord) map[int]ObservationSummary {
	out := map[int]ObservationSummary{}
	for _, o := range SortObservations(observations) {
		s, ok := out[o.Phase]
		if !ok {
			s = ObservationSummary{Phase: o.Phase, FirstConductivity: o.SolutionConductivity, LastConductivity: o.SolutionConductivity, MinConductivity: o.SolutionConductivity, MaxConductivity: o.SolutionConductivity, MinPH: o.PHValue, MaxPH: o.PHValue, MinTemperature: o.Temperature, MaxTemperature: o.Temperature}
		}
		s.Count++
		s.LastConductivity = o.SolutionConductivity
		s.LatestConductivity, s.LatestPH, s.LatestTemperature = o.SolutionConductivity, o.PHValue, o.Temperature
		if o.AnomalyCode != "" {
			s.AnomalyCount++
			if o.AnomalyCode == "TREND_REVERSAL" || o.AnomalyCode == "REPEAT_ANOMALY" {
				s.TrendReversal = true
			}
		}
		if o.SolutionConductivity < s.MinConductivity {
			s.MinConductivity = o.SolutionConductivity
		}
		if o.SolutionConductivity > s.MaxConductivity {
			s.MaxConductivity = o.SolutionConductivity
		}
		if o.PHValue < s.MinPH {
			s.MinPH = o.PHValue
		}
		if o.PHValue > s.MaxPH {
			s.MaxPH = o.PHValue
		}
		if o.Temperature < s.MinTemperature {
			s.MinTemperature = o.Temperature
		}
		if o.Temperature > s.MaxTemperature {
			s.MaxTemperature = o.Temperature
		}
		out[o.Phase] = s
	}
	for phase, s := range out {
		if s.Count < 2 || s.LastConductivity == s.FirstConductivity {
			s.TrendDirection = "FLAT"
		} else if s.LastConductivity < s.FirstConductivity {
			s.TrendDirection = "DOWN"
		} else {
			s.TrendDirection = "UP"
		}
		out[phase] = s
	}
	return out
}

// SummariesForPlan returns summaries for every plan phase, marking uncovered phases with Count 0.
func SummariesForPlan(observations []ObservationRecord, plan assessment.TreatmentPlan) map[int]ObservationSummary {
	out := Summaries(observations)
	for _, step := range plan.PhaseSteps {
		if _, ok := out[step.Phase]; !ok {
			out[step.Phase] = ObservationSummary{Phase: step.Phase, TrendDirection: "NONE"}
		}
	}
	return out
}
