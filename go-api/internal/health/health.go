package health

// Status represents a simple health response payload.
type Status struct {
	Status string `json:"status"`
}

func OK() Status {
	return Status{Status: "ok"}
}
