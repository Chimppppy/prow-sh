package client

// HealthResponse is the JSON body for GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}

// VersionResponse is the JSON body for GET /version.
type VersionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}
