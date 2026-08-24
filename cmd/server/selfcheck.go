package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"museum-desalination/internal/workflow"
)

func selfCheck(handler http.Handler, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 2 * time.Second}
	done := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err == http.ErrServerClosed {
			err = nil
		}
		done <- err
	}()
	defer func() {
		_ = server.Close()
		<-done
	}()
	baseURL := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 5 * time.Second}
	var batch workflow.ProtectionBatch
	call := func(method, path string, body any, etag string, out any) error {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, baseURL+path, rd)
		req.Header.Set("Content-Type", "application/json")
		if etag != "" {
			req.Header.Set("If-Match", etag)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("%s %s", method, path)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	sample := map[string]any{"material_type": "stone", "location_code": "A-01", "initial_chloride": 120, "moisture_ratio": 0.42, "condition_grade": "B", "evidence_refs": []string{"lab-001"}}
	if err := call(http.MethodPost, "/v1/batches", map[string]any{"catalog_ref": "CAT-001", "title": "石碑脱盐", "responsible_user": "tech", "samples": []any{sample}}, "", &batch); err != nil {
		return err
	}
	etag := strconv.Itoa(batch.Revision)
	if err := call(http.MethodPost, "/v1/batches/"+batch.ID+"/observations", map[string]any{"action": "assign", "phase": 1, "assignee": "operator", "operator": "tech"}, etag, &batch); err != nil {
		return err
	}
	etag = strconv.Itoa(batch.Revision)
	base := time.Now().UTC()
	observations := []map[string]any{}
	for i := 0; i < 3; i++ {
		observations = append(observations, map[string]any{"phase": 1, "observed_at": base.Add(time.Duration(i) * 24 * time.Hour), "solution_conductivity": 500 - float64(i)*30, "ph_value": 7.1, "temperature": 21, "sample_notes": "稳定"})
	}
	if err := call(http.MethodPost, "/v1/batches/"+batch.ID+"/observations", map[string]any{"operator": "operator", "observations": observations}, etag, &batch); err != nil {
		return err
	}
	etag = strconv.Itoa(batch.Revision)
	if err := call(http.MethodPost, "/v1/batches/"+batch.ID+"/completion", map[string]any{"phase": 1, "actor": "tech"}, etag, &batch); err != nil {
		return err
	}
	etag = strconv.Itoa(batch.Revision)
	for _, reviewer := range []string{"reviewer-a", "reviewer-b"} {
		if err := call(http.MethodPost, "/v1/batches/"+batch.ID+"/reviews", map[string]any{"phase": 1, "reviewer": reviewer, "decision": "approve", "findings": "证据完整", "evidence_refs": []string{"lab-001"}}, etag, &batch); err != nil {
			return err
		}
		etag = strconv.Itoa(batch.Revision)
	}
	if err := call(http.MethodPost, "/v1/batches/"+batch.ID+"/archive", map[string]any{"archived_by": "archivist", "evidence_index": []string{"lab-001"}}, etag, &batch); err != nil {
		return err
	}
	if batch.Status != workflow.StatusArchived {
		return fmt.Errorf("expected archived, got %s", batch.Status)
	}
	return nil
}
