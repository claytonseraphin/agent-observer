package outcome

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Statuses a call can have. They live here so the tracker and the error
// helpers can't drift apart on spelling.
const (
	StatusProcessing = "processing"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
)

// How much of a response body the proxy reads when it looks for an error.
const maxPeekBytes = 1 << 20 // 1 MB

// CallOutcome is how one call ended. FinishCall takes this one value, so it
// doesn't need to know where the outcome came from.
type CallOutcome struct {
	Status string
	Error  string
}

// OutcomeFromError is for calls that failed before any response arrived:
// connection refused, timeout, or the client giving up.
func OutcomeFromError(err error) CallOutcome {
	return CallOutcome{Status: StatusFailed, Error: describeError(err)}
}

// OutcomeFromResponse is for calls where the agent answered. It checks the HTTP
// status first, then looks inside the JSON-RPC body, because an A2A error comes
// back as HTTP 200 with an "error" object inside.
//
// It replaces resp.Body with an equivalent body, so the client still receives
// the full response.
func OutcomeFromResponse(resp *http.Response) CallOutcome {
	if resp.StatusCode >= 400 {
		return CallOutcome{Status: StatusFailed, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return CallOutcome{Status: StatusSuccess}
	}

	body, restored := peekBody(resp.Body, maxPeekBytes)
	resp.Body = restored
  //fmt.Println("Body: %v", string(body))
	return outcomeFromRPCBody(body)
}

// outcomeFromRPCBody works on plain bytes, so it can be tested without HTTP.
func outcomeFromRPCBody(body []byte) CallOutcome {
	var reply struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		// Not JSON we understand. The HTTP status was fine, so don't call it a failure.
		return CallOutcome{Status: StatusSuccess}
	}

	if reply.Error != nil {
		return CallOutcome{
			Status: StatusFailed,
			Error:  fmt.Sprintf("rpc error %d: %s", reply.Error.Code, reply.Error.Message),
		}
	}

	switch state := taskState(reply.Result); state {
	case "failed", "rejected", "canceled", "cancelled":
		return CallOutcome{Status: StatusFailed, Error: "task " + state}
	}
	return CallOutcome{Status: StatusSuccess}
}

// taskState reads the A2A task state out of a result. It accepts both a result
// that is the task itself and one wrapped in a "task" field, and it normalizes
// names like TASK_STATE_COMPLETED to "completed".
func taskState(result json.RawMessage) string {
	var r struct {
		Status struct {
			State string `json:"state"`
		} `json:"status"`
		Task struct {
			Status struct {
				State string `json:"state"`
			} `json:"status"`
		} `json:"task"`
	}
	if json.Unmarshal(result, &r) != nil {
		return ""
	}
	state := r.Status.State
	if state == "" {
		state = r.Task.Status.State
	}
	return strings.ToLower(strings.TrimPrefix(state, "TASK_STATE_"))
}

// describeError turns a proxy error into a short message for the call record.
func describeError(err error) string {
	var opErr *net.OpError
	switch {
	case errors.Is(err, context.Canceled):
		return "client canceled the request"
	case errors.Is(err, context.DeadlineExceeded):
		return "timed out"
	case errors.As(err, &opErr):
		return "unreachable: " + opErr.Error()
	default:
		return "proxy error: " + err.Error()
	}
}

// peekBody reads up to limit bytes and returns them, together with a body that
// still delivers everything (the bytes it consumed, then the unread rest).
// A body can only be read once, so without this the client would get nothing.
func peekBody(rc io.ReadCloser, limit int64) ([]byte, io.ReadCloser) {
	head, _ := io.ReadAll(io.LimitReader(rc, limit))
	restored := struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), rc), rc}
	return head, restored
}
