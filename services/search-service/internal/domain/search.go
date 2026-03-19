// Package domain defines core types for the search service.
package domain

import "time"

// MessageDoc is the Elasticsearch document shape for an indexed message.
type MessageDoc struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ChannelID   string    `json:"channel_id"`
	AuthorID    string    `json:"author_id"`
	Body        string    `json:"body"`
	ThreadID    string    `json:"thread_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsDeleted   bool      `json:"is_deleted"`
}

// SearchFilter holds optional query constraints.
type SearchFilter struct {
	WorkspaceID string
	ChannelID   string    // SRCH-03: search within a specific channel
	AuthorID    string    // SRCH-02: filter by user
	After       time.Time // SRCH-02: date range start
	Before      time.Time // SRCH-02: date range end
}

// SearchResult is a single hit from Elasticsearch.
type SearchResult struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ChannelID   string    `json:"channel_id"`
	AuthorID    string    `json:"author_id"`
	Body        string    `json:"body"`
	Highlight   string    `json:"highlight,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Score       float64   `json:"score"`
}

// SearchPage is a paginated list of search hits.
type SearchPage struct {
	Results []SearchResult `json:"results"`
	Total   int64          `json:"total"`
	From    int            `json:"from"`
	Size    int            `json:"size"`
}
