package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriterError(t *testing.T) {

	rr := httptest.NewRecorder()

	writeError(rr, http.StatusNotFound, "project_not_found", "project not found")

	if rr.Code != http.StatusNotFound {
		t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	contentType := rr.Result().Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("content type: got %q, want %q", contentType, "application/json")
	}

	var got errorBody
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}

	if got.Error.Code != "project_not_found" {
		t.Errorf("error code: got %q, want %q", got.Error.Code, "project_not_found")
	}

	if got.Error.Message != "project not found" {
		t.Errorf("error message: got %q, want %q", got.Error.Message, "project not found")
	}

}
