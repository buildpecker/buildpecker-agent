package custom_error

func MalformedJSONRequestError(status int, cause error) *AppError {
	return New("MALFORMED_JSON", "The JSON request body was malformed", status, cause)
}
