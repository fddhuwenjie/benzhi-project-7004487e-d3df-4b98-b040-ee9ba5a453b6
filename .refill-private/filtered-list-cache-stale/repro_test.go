package filtered_list_cache_stale_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/transport"
	"museum-desalination/internal/workflow"
)

func TestFilteredBatchCacheInvalidatesAfterStateTransition(t *testing.T) {
	handler := transport.New(workflow.New(archive.NewStore(t.TempDir()))).Routes()

	create := httptest.NewRequest(http.MethodPost, "/v1/batches", bytes.NewBufferString(`{
		"catalog_ref":"CACHE-EDGE-001",
		"title":"筛选缓存状态推进复现",
		"responsible_user":"cache-owner",
		"samples":[{
			"material_type":"stone",
			"location_code":"A-01",
			"initial_chloride":80,
			"moisture_ratio":0.3,
			"condition_grade":"A",
			"evidence_refs":["lab-cache-001"]
		}]
	}`))
	create.Header.Set("Content-Type", "application/json")
	createdResponse := httptest.NewRecorder()
	handler.ServeHTTP(createdResponse, create)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createdResponse.Code, createdResponse.Body.String())
	}
	var batch workflow.ProtectionBatch
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}

	pendingBefore := getFiltered(t, handler, workflow.StatusPending)
	if len(pendingBefore.Batches) != 1 || pendingBefore.Batches[0].ID != batch.ID {
		t.Fatalf("pending cache was not primed: %+v", pendingBefore.Batches)
	}

	assign := httptest.NewRequest(http.MethodPost, "/v1/batches/"+batch.ID+"/observations", bytes.NewBufferString(`{
		"action":"assign",
		"phase":1,
		"assignee":"operator-a",
		"operator":"cache-owner"
	}`))
	assign.Header.Set("Content-Type", "application/json")
	assign.Header.Set("If-Match", createdResponse.Header().Get("ETag"))
	assignedResponse := httptest.NewRecorder()
	handler.ServeHTTP(assignedResponse, assign)
	if assignedResponse.Code != http.StatusOK {
		t.Fatalf("assign status=%d body=%s", assignedResponse.Code, assignedResponse.Body.String())
	}

	processing := getFiltered(t, handler, workflow.StatusProcessing)
	if len(processing.Batches) != 1 || processing.Batches[0].ID != batch.ID {
		t.Fatalf("state transition was not committed: %+v", processing.Batches)
	}
	pendingAfter := getFiltered(t, handler, workflow.StatusPending)
	if len(pendingAfter.Batches) != 0 {
		t.Fatalf("TestFilteredBatchCacheInvalidatesAfterStateTransition: stale PENDING_PROCESSING result still contains %s after PROCESSING transition", pendingAfter.Batches[0].ID)
	}
}

func getFiltered(t *testing.T, handler http.Handler, status string) workflow.BatchListResult {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/batches?status="+status, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var result workflow.BatchListResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
