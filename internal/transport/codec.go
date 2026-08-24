package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func expectedRevision(r *http.Request) int {
	value := r.Header.Get("If-Match")
	if value == "" {
		return 0
	}
	value = strings.Trim(value, "\"")
	revision, _ := strconv.Atoi(value)
	return revision
}
