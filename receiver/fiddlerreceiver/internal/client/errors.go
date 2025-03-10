package client

import "fmt"

type APIError struct {
	StatusCode int
	Message    string
	Endpoint   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Fiddler API error (%d) on %s: %s", e.StatusCode, e.Endpoint, e.Message)
}
