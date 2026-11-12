package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/claytonseraphin/agent-observer/internal/projects"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	detail := errorDetail{
		Code:    code,
		Message: message,
	}

	body := errorBody{
		Error: detail,
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("writing error response: %v", err)
	}

}

type errorMapping struct {
	err    error
	status int
	code   string
}

var errorMappings = []errorMapping{
	{projects.ErrProjectNotFound, http.StatusNotFound, "project_not_found"},
	{projects.ErrAgentNotFound, http.StatusNotFound, "agent_not_found"},
	{projects.ErrNameTaken, http.StatusConflict, "name_taken"},
	{projects.ErrAgentExists, http.StatusConflict, "agent_exists"},
	{projects.ErrDefaultProtected, http.StatusConflict, "default_protected"},
	{projects.ErrInvalidProjectName, http.StatusBadRequest, "invalid_name"},
	{projects.ErrInvalidAgentName, http.StatusBadRequest, "invalid_agent_name"},
	{projects.ErrReservedAgentName, http.StatusBadRequest, "reserved_agent_name"},
	{projects.ErrInvalidUpstreamURL, http.StatusBadRequest, "invalid_url"},
}

func handleError(w http.ResponseWriter, err error) {
	for _, m := range errorMappings {
		if errors.Is(err, m.err) {
			writeError(w, m.status, m.code, err.Error())
			return
		}
	}

	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal error")
}
