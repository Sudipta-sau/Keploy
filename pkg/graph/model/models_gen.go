// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

type RunTestSetResponse struct {
	Success   bool    `json:"success"`
	TestRunID string  `json:"testRunId"`
	Message   *string `json:"message,omitempty"`
}

type TestSetStatus struct {
	Status string `json:"status"`
}
