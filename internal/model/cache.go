package model

import "time"

// ActionsCache represents a GitHub Actions cache entry.
type ActionsCache struct {
	ID             int64     `json:"id"`
	Ref            string    `json:"ref"`
	Key            string    `json:"key"`
	Version        string    `json:"version"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
	CreatedAt      time.Time `json:"created_at"`
	SizeInBytes    int64     `json:"size_in_bytes"`
}

// ActionsCacheList is the response from the GitHub Actions cache list endpoint.
type ActionsCacheList struct {
	TotalCount    int            `json:"total_count"`
	ActionsCaches []ActionsCache `json:"actions_caches"`
}
