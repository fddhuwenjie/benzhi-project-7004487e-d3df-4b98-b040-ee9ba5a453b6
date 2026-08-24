package assessment

import (
	"fmt"
	"strings"
)

func ValidateSample(s Sample) error {
	if strings.TrimSpace(s.MaterialType) == "" || strings.TrimSpace(s.LocationCode) == "" || strings.TrimSpace(s.ConditionGrade) == "" {
		return fmt.Errorf("%w: material_type, location_code and condition_grade are required", ErrInvalidSample)
	}
	if s.InitialChloride < 0 || s.InitialChloride > 100000 {
		return fmt.Errorf("%w: initial_chloride out of range", ErrInvalidSample)
	}
	if s.MoistureRatio < 0 || s.MoistureRatio > 1 {
		return fmt.Errorf("%w: moisture_ratio must be between 0 and 1", ErrInvalidSample)
	}
	seen := map[string]bool{}
	for i, ref := range s.EvidenceRefs {
		r := strings.TrimSpace(ref)
		if r == "" {
			return ErrEvidenceMissing
		}
		if seen[r] {
			return fmt.Errorf("%w: duplicate evidence reference", ErrInvalidSample)
		}
		seen[r] = true
		s.EvidenceRefs[i] = r
	}
	if len(s.EvidenceRefs) == 0 {
		return ErrEvidenceMissing
	}
	return nil
}

// ValidateBatchField applies the public batch identifier rules and returns a
// stable field-specific error that transport can expose to callers.
func ValidateBatchField(field, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s_required", field)
	}
	for _, r := range trimmed {
		if r == '\u0000' || r == '\n' || r == '\r' || r == '\t' {
			return fmt.Errorf("%s_invalid_charset", field)
		}
	}
	max := 128
	if field == "title" {
		max = 200
	}
	if len([]rune(trimmed)) > max {
		return fmt.Errorf("%s_too_long", field)
	}
	return nil
}

func ValidateSamples(samples []Sample) error {
	if len(samples) == 0 {
		return fmt.Errorf("%w: at least one sample required", ErrInvalidSample)
	}
	locations := map[string]bool{}
	for i := range samples {
		s := &samples[i]
		s.LocationCode = strings.TrimSpace(s.LocationCode)
		if s.LocationCode == "" {
			return fmt.Errorf("sample[%d]: %w: location_code is required", i, ErrInvalidSample)
		}
		if locations[s.LocationCode] {
			return fmt.Errorf("sample[%d]: %w: duplicate location_code %s", i, ErrInvalidSample, s.LocationCode)
		}
		locations[s.LocationCode] = true
		if err := ValidateSample(*s); err != nil {
			return fmt.Errorf("sample[%d]: %w", i, err)
		}
	}
	return nil
}

func GeneratePlan(batchID, author string, samples []Sample, version int) (TreatmentPlan, error) {
	if err := ValidateSamples(samples); err != nil {
		return TreatmentPlan{}, err
	}
	var chloride, moisture float64
	for _, s := range samples {
		chloride += s.InitialChloride
		moisture += s.MoistureRatio
	}
	chloride /= float64(len(samples))
	moisture /= float64(len(samples))
	phMin, phMax := 6.0, 8.5
	if moisture > 0.65 {
		phMin, phMax = 6.2, 8.0
	}
	steps := []PhaseStep{
		{Phase: 1, Days: 3, Solution: "去离子水预浸", Target: chloride * 0.60, ConductivityLimit: 1200, TemperatureLimit: 24, PHMin: phMin, PHMax: phMax},
		{Phase: 2, Days: 5, Solution: "低浓度碳酸氢铵", Target: chloride * 0.25, ConductivityLimit: 800, TemperatureLimit: 22, PHMin: phMin, PHMax: phMax},
		{Phase: 3, Days: 4, Solution: "去离子水置换", Target: chloride * 0.10, ConductivityLimit: 450, TemperatureLimit: 20, PHMin: phMin, PHMax: phMax},
	}
	p := TreatmentPlan{ID: fmt.Sprintf("plan-%s-v%d", batchID, version), BatchID: batchID, Version: version, PhaseSteps: steps, SolutionProfile: "分阶段梯度脱盐", TemperatureLimit: steps[0].TemperatureLimit, ChlorideTarget: chloride * 0.05, Author: author}
	if err := ValidatePlan(p); err != nil {
		return TreatmentPlan{}, err
	}
	return p, nil
}

// ValidatePlan enforces internal consistency before a plan can be published.
func ValidatePlan(plan TreatmentPlan) error {
	if plan.Version < 1 || strings.TrimSpace(plan.Author) == "" || len(plan.PhaseSteps) == 0 {
		return ErrInvalidPlan
	}
	seen := map[int]bool{}
	for i, step := range plan.PhaseSteps {
		if step.Phase != i+1 {
			return fmt.Errorf("%w: phase order must be continuous at phase %d", ErrInvalidPlan, step.Phase)
		}
		if step.Phase < 1 || seen[step.Phase] || step.Days <= 0 || step.Target < 0 || step.ConductivityLimit <= 0 || step.TemperatureLimit < -10 || step.TemperatureLimit > 60 || step.PHMin < 0 || step.PHMax > 14 || step.PHMin >= step.PHMax {
			return fmt.Errorf("%w: phase %d parameters are inconsistent", ErrInvalidPlan, step.Phase)
		}
		if i > 0 && step.ConductivityLimit > plan.PhaseSteps[i-1].ConductivityLimit {
			return fmt.Errorf("%w: phase %d conductivity limit increases", ErrInvalidPlan, step.Phase)
		}
		if i > 0 && step.Target > plan.PhaseSteps[i-1].Target {
			return fmt.Errorf("%w: phase %d target increases", ErrInvalidPlan, step.Phase)
		}
		seen[step.Phase] = true
	}
	if plan.ChlorideTarget < 0 || plan.ChlorideTarget >= plan.PhaseSteps[0].Target {
		return fmt.Errorf("%w: final chloride target must be below first phase target", ErrInvalidPlan)
	}
	return nil
}

func StepFor(plan TreatmentPlan, phase int) (PhaseStep, bool) {
	for _, s := range plan.PhaseSteps {
		if s.Phase == phase {
			return s, true
		}
	}
	return PhaseStep{}, false
}

func DiffPlan(previous, next TreatmentPlan) []string {
	var changes []string
	for _, ns := range next.PhaseSteps {
		ps, ok := StepFor(previous, ns.Phase)
		if !ok {
			changes = append(changes, fmt.Sprintf("phase_%d_added", ns.Phase))
			continue
		}
		if ps.Days != ns.Days {
			changes = append(changes, fmt.Sprintf("phase_%d.days", ns.Phase))
		}
		if ps.Solution != ns.Solution {
			changes = append(changes, fmt.Sprintf("phase_%d.solution", ns.Phase))
		}
		if ps.ConductivityLimit != ns.ConductivityLimit {
			changes = append(changes, fmt.Sprintf("phase_%d.conductivity_limit", ns.Phase))
		}
		if ps.TemperatureLimit != ns.TemperatureLimit {
			changes = append(changes, fmt.Sprintf("phase_%d.temperature_limit", ns.Phase))
		}
		if ps.PHMin != ns.PHMin || ps.PHMax != ns.PHMax {
			changes = append(changes, fmt.Sprintf("phase_%d.ph_range", ns.Phase))
		}
	}
	if previous.ChlorideTarget != next.ChlorideTarget {
		changes = append(changes, "chloride_target")
	}
	if previous.SolutionProfile != next.SolutionProfile {
		changes = append(changes, "solution_profile")
	}
	return changes
}
