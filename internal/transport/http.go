package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"museum-desalination/internal/assessment"
	"museum-desalination/internal/review"
	"museum-desalination/internal/treatment"
	"museum-desalination/internal/workflow"
)

type Server struct{ WF *workflow.Workflow }

func New(wf *workflow.Workflow) *Server { return &Server{WF: wf} }
func (s *Server) Routes() http.Handler  { return http.HandlerFunc(s.ServeHTTP) }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if path == "v1/batches" && r.Method == http.MethodPost {
		s.createBatch(w, r)
		return
	}
	if path == "v1/batches" && r.Method == http.MethodGet {
		f := workflow.BatchFilter{Status: r.URL.Query().Get("status"), CurrentPhase: r.URL.Query().Get("current_phase"), ResponsibleUser: r.URL.Query().Get("responsible_user")}
		parse := func(key string) (*time.Time, error) {
			v := r.URL.Query().Get(key)
			if v == "" {
				return nil, nil
			}
			t, err := time.Parse(time.RFC3339, v)
			return &t, err
		}
		var err error
		if f.From, err = parse("from"); err != nil {
			writeErr(w, workflow.ErrInvalidFilter)
			return
		}
		if f.To, err = parse("to"); err != nil {
			writeErr(w, workflow.ErrInvalidFilter)
			return
		}
		validStatus := map[string]bool{workflow.StatusDraft: true, workflow.StatusPending: true, workflow.StatusProcessing: true, workflow.StatusPaused: true, workflow.StatusReview: true, workflow.StatusApproved: true, workflow.StatusArchived: true}
		if f.Status != "" && !validStatus[f.Status] {
			writeErr(w, workflow.ErrInvalidFilter)
			return
		}
		if f.CurrentPhase != "" {
			if n, e := strconv.Atoi(f.CurrentPhase); e != nil || n < 1 {
				writeErr(w, workflow.ErrInvalidFilter)
				return
			}
		}
		res, err := s.WF.ListFiltered(f)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	}
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "batches" {
		id := parts[2]
		if len(parts) == 3 && r.Method == http.MethodGet {
			s.getBatch(w, id)
			return
		}
		if len(parts) == 4 {
			switch parts[3] {
			case "samples":
				if r.Method == http.MethodPost {
					s.addSample(w, r, id)
					return
				}
			case "plan":
				if r.Method == http.MethodPost {
					s.plan(w, r, id)
					return
				}
			case "observations":
				if r.Method == http.MethodPost {
					s.observation(w, r, id)
					return
				}
			case "completion":
				if r.Method == http.MethodPost {
					s.completion(w, r, id)
					return
				}
			case "reviews":
				if r.Method == http.MethodPost {
					s.review(w, r, id)
					return
				}
			case "archive":
				if r.Method == http.MethodPost {
					s.archive(w, r, id)
					return
				}
			case "audit":
				if r.Method == http.MethodGet {
					s.audit(w, id, r)
					return
				}
			}
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "NOT_FOUND", "message": "route not found"}})
}

func (s *Server) createBatch(w http.ResponseWriter, r *http.Request) {
	var req workflow.CreateRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.WF.Create(req, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusCreated, b)
}
func (s *Server) getBatch(w http.ResponseWriter, id string) {
	b, err := s.WF.Get(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusOK, b)
}
func (s *Server) addSample(w http.ResponseWriter, r *http.Request, id string) {
	var raw json.RawMessage
	if err := decode(r, &raw); err != nil {
		writeErr(w, err)
		return
	}
	var reqs []assessment.Sample
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &reqs); err != nil {
			writeErr(w, err)
			return
		}
	} else {
		var one assessment.Sample
		if err := json.Unmarshal(raw, &one); err != nil {
			writeErr(w, err)
			return
		}
		reqs = []assessment.Sample{one}
	}
	if len(reqs) == 0 {
		writeErr(w, errors.New("sample is required"))
		return
	}
	b, err := s.WF.AddSamples(id, expectedRevision(r), reqs, r.Header.Get("X-User"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusCreated, b)
	return
}
func (s *Server) plan(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Author  string                    `json:"author"`
		Confirm bool                      `json:"confirm"`
		Plan    *assessment.TreatmentPlan `json:"plan"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.WF.ConfirmPlan(id, expectedRevision(r), req.Author, req.Confirm, req.Plan)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusOK, b)
}
func (s *Server) observation(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Action                  string                        `json:"action"`
		Phase                   int                           `json:"phase"`
		Assignee                string                        `json:"assignee"`
		Observation             *treatment.ObservationRecord  `json:"observation"`
		Observations            []treatment.ObservationRecord `json:"observations"`
		Operator                string                        `json:"operator"`
		RemediationNote         string                        `json:"remediation_note"`
		RemediationOwner        string                        `json:"remediation_owner"`
		RemediationEvidenceRefs []string                      `json:"remediation_evidence_refs"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Action == "assign" {
		b, err := s.WF.AssignTaskWithRemediationEvidence(id, expectedRevision(r), req.Phase, req.Assignee, req.Operator, req.RemediationNote, req.RemediationOwner, req.RemediationEvidenceRefs)
		if err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("ETag", strconv.Itoa(b.Revision))
		writeJSON(w, http.StatusOK, b)
		return
	}
	if req.Observation == nil && len(req.Observations) == 0 {
		writeErr(w, errors.New("observation is required"))
		return
	}
	obs := req.Observations
	if len(obs) == 0 {
		obs = []treatment.ObservationRecord{*req.Observation}
	}
	b, err := s.WF.AddObservations(id, expectedRevision(r), obs, req.Operator)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusCreated, b)
}
func (s *Server) completion(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Phase int    `json:"phase"`
		Actor string `json:"actor"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.WF.Complete(id, expectedRevision(r), req.Phase, req.Actor)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusOK, b)
}
func (s *Server) review(w http.ResponseWriter, r *http.Request, id string) {
	var d review.ReviewDecision
	if err := decode(r, &d); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(d.Findings) == "" {
		writeErr(w, errors.New("findings is required"))
		return
	}
	b, err := s.WF.AddReview(id, expectedRevision(r), d, d.Reviewer)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusOK, b)
}
func (s *Server) archive(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		ArchivedBy    string   `json:"archived_by"`
		EvidenceIndex []string `json:"evidence_index"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.WF.Archive(id, expectedRevision(r), req.ArchivedBy, req.EvidenceIndex)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Itoa(b.Revision))
	writeJSON(w, http.StatusOK, b)
}
func (s *Server) audit(w http.ResponseWriter, id string, r *http.Request) {
	var from, to *time.Time
	parse := func(key string) (*time.Time, error) {
		v := r.URL.Query().Get(key)
		if v == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		return &t, nil
	}
	var err error
	if from, err = parse("from"); err != nil {
		writeErr(w, errors.New("invalid time range"))
		return
	}
	if to, err = parse("to"); err != nil {
		writeErr(w, errors.New("invalid time range"))
		return
	}
	if from != nil && to != nil && from.After(*to) {
		writeErr(w, errors.New("invalid time range"))
		return
	}
	typ := r.URL.Query().Get("type")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeErr(w, workflow.ErrInvalidLimit)
			return
		}
	}
	page, err := s.WF.AuditPage(id, from, to, typ, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
