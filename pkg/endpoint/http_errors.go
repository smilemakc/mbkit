package endpoint

import (
	"errors"
	"net/http"

	"github.com/smilemakc/mbkit/pkg/policy"
)

type HTTPErrorPayload struct {
	Error string `json:"error"`
}

// ClassifyHTTPError maps domain errors to HTTP status codes and payloads.
func ClassifyHTTPError(err error) (int, any) {
	if err == nil {
		return http.StatusInternalServerError, HTTPErrorPayload{Error: ""}
	}

	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, policy.ErrBadRequest):
		status = http.StatusBadRequest
	case errors.Is(err, policy.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, policy.ErrForbidden), errors.Is(err, policy.ErrPermission):
		status = http.StatusForbidden
	case errors.Is(err, policy.ErrNotFound):
		status = http.StatusNotFound
	}

	return status, HTTPErrorPayload{Error: err.Error()}
}
