package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/workflow"
)

func TestCreateAndGetBatch(t *testing.T) {
	h := New(workflow.New(archive.NewStore(t.TempDir())))
	r := httptest.NewRequest(http.MethodPost, "/v1/batches", bytes.NewBufferString(`{"catalog_ref":"C","title":"T","responsible_user":"u","samples":[{"material_type":"stone","location_code":"A","initial_chloride":10,"moisture_ratio":0.2,"condition_grade":"A","evidence_refs":["e"]}]}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "same")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var b workflow.ProtectionBatch
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/v1/batches/"+b.ID, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("get status %d", w2.Code)
	}
}
