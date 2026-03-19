// Package service contains the search service business logic.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/relay-im/relay/services/search-service/internal/domain"
	"github.com/rs/zerolog"
)

const indexName = "relay_messages"

// SearchService indexes messages from NATS and answers search queries via Elasticsearch.
type SearchService struct {
	esURL  string
	client *http.Client
	nc     *nats.Conn
	log    zerolog.Logger
}

// New creates a SearchService.
func New(esURL string, nc *nats.Conn, log zerolog.Logger) *SearchService {
	return &SearchService{
		esURL:  strings.TrimRight(esURL, "/"),
		client: &http.Client{Timeout: 10 * time.Second},
		nc:     nc,
		log:    log,
	}
}

// EnsureIndex creates the messages index with appropriate mappings if it does not exist.
func (s *SearchService) EnsureIndex(ctx context.Context) error {
	mapping := `{
  "mappings": {
    "properties": {
      "id":           { "type": "keyword" },
      "workspace_id": { "type": "keyword" },
      "channel_id":   { "type": "keyword" },
      "author_id":    { "type": "keyword" },
      "body":         { "type": "text", "analyzer": "standard" },
      "thread_id":    { "type": "keyword" },
      "created_at":   { "type": "date" },
      "updated_at":   { "type": "date" },
      "is_deleted":   { "type": "boolean" }
    }
  }
}`

	url := fmt.Sprintf("%s/%s", s.esURL, indexName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(mapping))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer resp.Body.Close()
	// 200 = created; 400 with resource_already_exists_exception is fine.
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// IndexMessage upserts a message document into Elasticsearch.
func (s *SearchService) IndexMessage(ctx context.Context, doc domain.MessageDoc) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s/_doc/%s", s.esURL, indexName, doc.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("index message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("index message: status %d: %s", resp.StatusCode, b)
	}
	return nil
}

// DeleteMessage marks a message as deleted in the index (soft-delete via update).
func (s *SearchService) DeleteMessage(ctx context.Context, id string) error {
	script := `{"doc": {"is_deleted": true}}`
	url := fmt.Sprintf("%s/%s/_update/%s", s.esURL, indexName, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(script))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Search runs a full-text query with optional filters.
func (s *SearchService) Search(ctx context.Context, query string, filter domain.SearchFilter, from, size int) (*domain.SearchPage, error) {
	if size <= 0 || size > 100 {
		size = 20
	}

	must := []any{
		map[string]any{"match": map[string]any{"body": map[string]any{"query": query}}},
		map[string]any{"term": map[string]any{"is_deleted": false}},
		map[string]any{"term": map[string]any{"workspace_id": filter.WorkspaceID}},
	}

	if filter.ChannelID != "" {
		must = append(must, map[string]any{"term": map[string]any{"channel_id": filter.ChannelID}})
	}
	if filter.AuthorID != "" {
		must = append(must, map[string]any{"term": map[string]any{"author_id": filter.AuthorID}})
	}
	if !filter.After.IsZero() || !filter.Before.IsZero() {
		r := map[string]any{}
		if !filter.After.IsZero() {
			r["gte"] = filter.After.Format(time.RFC3339)
		}
		if !filter.Before.IsZero() {
			r["lte"] = filter.Before.Format(time.RFC3339)
		}
		must = append(must, map[string]any{"range": map[string]any{"created_at": r}})
	}

	esQuery := map[string]any{
		"from": from,
		"size": size,
		"query": map[string]any{
			"bool": map[string]any{"must": must},
		},
		"highlight": map[string]any{
			"fields": map[string]any{
				"body": map[string]any{
					"fragment_size":       150,
					"number_of_fragments": 1,
					"pre_tags":            []string{"<mark>"},
					"post_tags":           []string{"</mark>"},
				},
			},
		},
		"sort": []any{
			map[string]any{"_score": "desc"},
			map[string]any{"created_at": "desc"},
		},
	}

	body, err := json.Marshal(esQuery)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/_search", s.esURL, indexName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("es search: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return s.parseResponse(raw, from, size)
}

// ── NATS consumer ─────────────────────────────────────────────────────────────

// StartConsumer subscribes to message NATS events and indexes them.
func (s *SearchService) StartConsumer(ctx context.Context) error {
	subjects := map[string]func(context.Context, []byte){
		"message.created": s.handleCreated,
		"message.updated": s.handleUpdated,
		"message.deleted": s.handleDeleted,
	}

	subs := make([]*nats.Subscription, 0, len(subjects))
	for subj, fn := range subjects {
		f := fn // capture
		sub, err := s.nc.Subscribe(subj, func(msg *nats.Msg) {
			f(ctx, msg.Data)
		})
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", subj, err)
		}
		subs = append(subs, sub)
	}

	go func() {
		<-ctx.Done()
		for _, sub := range subs {
			_ = sub.Unsubscribe()
		}
	}()

	return nil
}

func (s *SearchService) handleCreated(ctx context.Context, data []byte) {
	var env struct {
		Type    string `json:"type"`
		Payload struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspace_id"`
			ChannelID   string `json:"channel_id"`
			AuthorID    string `json:"author_id"`
			Body        string `json:"body"`
			ThreadID    string `json:"thread_id"`
			CreatedAt   string `json:"created_at"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}
	p := env.Payload
	createdAt, _ := time.Parse(time.RFC3339, p.CreatedAt)
	doc := domain.MessageDoc{
		ID:          p.ID,
		WorkspaceID: p.WorkspaceID,
		ChannelID:   p.ChannelID,
		AuthorID:    p.AuthorID,
		Body:        p.Body,
		ThreadID:    p.ThreadID,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
	if err := s.IndexMessage(ctx, doc); err != nil {
		s.log.Error().Err(err).Str("message_id", p.ID).Msg("failed to index message")
	}
}

func (s *SearchService) handleUpdated(ctx context.Context, data []byte) {
	// Re-use handleCreated since we do a full PUT upsert.
	s.handleCreated(ctx, data)
}

func (s *SearchService) handleDeleted(ctx context.Context, data []byte) {
	var env struct {
		Payload struct {
			ID string `json:"id"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}
	if err := s.DeleteMessage(ctx, env.Payload.ID); err != nil {
		s.log.Error().Err(err).Str("message_id", env.Payload.ID).Msg("failed to soft-delete message")
	}
}

// ── response parsing ──────────────────────────────────────────────────────────

func (s *SearchService) parseResponse(raw []byte, from, size int) (*domain.SearchPage, error) {
	var esResp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score  float64         `json:"_score"`
				Source json.RawMessage `json:"_source"`
				Highlight struct {
					Body []string `json:"body"`
				} `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &esResp); err != nil {
		return nil, fmt.Errorf("parse es response: %w", err)
	}

	results := make([]domain.SearchResult, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		var doc domain.MessageDoc
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			continue
		}
		highlight := ""
		if len(hit.Highlight.Body) > 0 {
			highlight = hit.Highlight.Body[0]
		}
		results = append(results, domain.SearchResult{
			ID:          doc.ID,
			WorkspaceID: doc.WorkspaceID,
			ChannelID:   doc.ChannelID,
			AuthorID:    doc.AuthorID,
			Body:        doc.Body,
			Highlight:   highlight,
			CreatedAt:   doc.CreatedAt,
			Score:       hit.Score,
		})
	}

	return &domain.SearchPage{
		Results: results,
		Total:   esResp.Hits.Total.Value,
		From:    from,
		Size:    size,
	}, nil
}
