package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	custom_errors "github.com/pthsarmah/forge/errors"
	"io"
	"net/http"
	"strings"
)

// DecodeJSONBody reads and validates a JSON request body into dst.
// It returns a structured *AppError on validation or decoding failures.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	ct := r.Header.Get("Content-Type")
	if ct != "" {
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
		if mediaType != "application/json" {
			msg := "Content-Type header is not application/json"
			return custom_errors.MalformedJSONRequestError(http.StatusUnsupportedMediaType, errors.New(msg))
		}
	}

	// Limit body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return custom_errors.MalformedJSONRequestError(http.StatusBadRequest, errors.New(msg))

		case errors.Is(err, io.ErrUnexpectedEOF):
			return custom_errors.MalformedJSONRequestError(http.StatusBadRequest, errors.New("Request body contains badly-formed JSON"))

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf(
				"Request body contains an invalid value for the %q field (at position %d)",
				unmarshalTypeError.Field, unmarshalTypeError.Offset,
			)
			return custom_errors.MalformedJSONRequestError(http.StatusBadRequest, errors.New(msg))

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return custom_errors.MalformedJSONRequestError(http.StatusBadRequest, errors.New(msg))

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return custom_errors.MalformedJSONRequestError(http.StatusBadRequest, errors.New(msg))

		case errors.As(err, &maxBytesError):
			msg := fmt.Sprintf("Request body must not be larger than %d bytes", maxBytesError.Limit)
			return custom_errors.MalformedJSONRequestError(http.StatusRequestEntityTooLarge, errors.New(msg))

		default:
			// Unknown JSON decoding error — wrap as internal
			return custom_errors.MalformedJSONRequestError(http.StatusBadRequest, err)
		}
	}

	// Extra data after valid JSON — reject
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		msg := "Request body must only contain a single JSON object"
		return custom_errors.MalformedJSONRequestError(http.StatusBadRequest, errors.New(msg))
	}

	return nil
}
