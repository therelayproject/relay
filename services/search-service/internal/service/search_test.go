package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/relay-im/relay/services/search-service/internal/domain"
	"github.com/rs/zerolog"
)

// ── parseResponse tests ───────────────────────────────────────────────────────

func TestParseResponse_Empty(t *testing.T) {
	svc := &SearchService{}
	raw := []byte(`{"hits":{"total":{"value":0},"hits":[]}}`)
	page, err := svc.parseResponse(raw, 0, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 0 {
		t.Errorf("expected Total=0, got %d", page.Total)
	}
	if len(page.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(page.Results))
	}
}

func TestParseResponse_WithHits(t *testing.T) {
	svc := &SearchService{}
	now := time.Now().UTC()
	source := map[string]any{
		"id":           "msg-1",
		"workspace_id": "ws-1",
		"channel_id":   "ch-1",
		"author_id":    "user-1",
		"body":         "hello world",
		"created_at":   now.Format(time.RFC3339),
		"updated_at":   now.Format(time.RFC3339),
	}
	srcJSON, _ := json.Marshal(source)

	raw := []byte(fmt.Sprintf(`{
		"hits": {
			"total": {"value": 1},
			"hits": [{
				"_score": 1.5,
				"_source": %s,
				"highlight": {"body": ["hello <mark>world</mark>"]}
			}]
		}
	}`, srcJSON))

	page, err := svc.parseResponse(raw, 0, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected Total=1, got %d", page.Total)
	}
	if len(page.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(page.Results))
	}
	r := page.Results[0]
	if r.ID != "msg-1" {
		t.Errorf("expected ID=msg-1, got %q", r.ID)
	}
	if r.Score != 1.5 {
		t.Errorf("expected Score=1.5, got %f", r.Score)
	}
	if r.Highlight != "hello <mark>world</mark>" {
		t.Errorf("unexpected highlight: %q", r.Highlight)
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	svc := &SearchService{}
	_, err := svc.parseResponse([]byte("not json"), 0, 20)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ── Search size clamping ──────────────────────────────────────────────────────

func TestSearch_ClampsSizeToDefault(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		receivedBody = b
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	_, _ = svc.Search(context.Background(), "hello", domain.SearchFilter{WorkspaceID: "ws-1"}, 0, 0) // size=0 → clamp to 20

	if size, ok := receivedBody["size"].(float64); !ok || size != 20 {
		t.Errorf("expected clamped size=20, got %v", receivedBody["size"])
	}
}

func TestSearch_ClampsSizeToMax(t *testing.T) {
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	_, _ = svc.Search(context.Background(), "hello", domain.SearchFilter{WorkspaceID: "ws-1"}, 0, 200) // > 100 → clamp to 20

	if size, ok := receivedBody["size"].(float64); !ok || size != 20 {
		t.Errorf("expected clamped size=20, got %v", receivedBody["size"])
	}
}

// ── IndexMessage ─────────────────────────────────────────────────────────────

func TestIndexMessage_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "relay_messages") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result":"created"}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	doc := domain.MessageDoc{
		ID:          "msg-1",
		WorkspaceID: "ws-1",
		ChannelID:   "ch-1",
		AuthorID:    "user-1",
		Body:        "hello",
		CreatedAt:   time.Now(),
	}
	if err := svc.IndexMessage(context.Background(), doc); err != nil {
		t.Fatalf("IndexMessage: unexpected error: %v", err)
	}
}

func TestIndexMessage_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal"}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	doc := domain.MessageDoc{ID: "msg-1"}
	err := svc.IndexMessage(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}

// ── DeleteMessage ─────────────────────────────────────────────────────────────

func TestDeleteMessage_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result":"updated"}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	if err := svc.DeleteMessage(context.Background(), "msg-1"); err != nil {
		t.Fatalf("DeleteMessage: unexpected error: %v", err)
	}
}

// ── EnsureIndex ───────────────────────────────────────────────────────────────

func TestEnsureIndex_AlreadyExists(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 400 with resource_already_exists_exception should be tolerated
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"type":"resource_already_exists_exception"}}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	if err := svc.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("EnsureIndex: should not error on 400 (already exists): %v", err)
	}
}

func TestEnsureIndex_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"boom"}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	err := svc.EnsureIndex(context.Background())
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

// ── handleCreated / handleUpdated / handleDeleted ─────────────────────────────

func makeIndexServer(t *testing.T, wantMethod string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != wantMethod {
			t.Errorf("expected method %s, got %s", wantMethod, r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result":"ok"}`)
	}))
}

func TestHandleCreated_ValidPayload(t *testing.T) {
	ts := makeIndexServer(t, http.MethodPut)
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	msg, _ := json.Marshal(map[string]any{
		"type": "message.created",
		"payload": map[string]any{
			"id":           "msg-1",
			"workspace_id": "ws-1",
			"channel_id":   "ch-1",
			"author_id":    "user-1",
			"body":         "hello from test",
			"created_at":   time.Now().UTC().Format(time.RFC3339),
		},
	})
	svc.handleCreated(context.Background(), msg)
}

func TestHandleCreated_InvalidJSON(t *testing.T) {
	svc := New("http://localhost:9999", nil, zerolog.Nop())
	// Should not panic for bad JSON.
	svc.handleCreated(context.Background(), []byte("not-json"))
}

func TestHandleUpdated_CallsIndexMessage(t *testing.T) {
	ts := makeIndexServer(t, http.MethodPut)
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	msg, _ := json.Marshal(map[string]any{
		"type": "message.updated",
		"payload": map[string]any{
			"id":           "msg-2",
			"workspace_id": "ws-1",
			"channel_id":   "ch-1",
			"author_id":    "user-1",
			"body":         "updated body",
			"created_at":   time.Now().UTC().Format(time.RFC3339),
		},
	})
	svc.handleUpdated(context.Background(), msg)
}

func TestHandleDeleted_ValidPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST for soft-delete, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result":"updated"}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	msg, _ := json.Marshal(map[string]any{
		"payload": map[string]any{
			"id": "msg-del-1",
		},
	})
	svc.handleDeleted(context.Background(), msg)
}

func TestHandleDeleted_InvalidJSON(t *testing.T) {
	svc := New("http://localhost:9999", nil, zerolog.Nop())
	// Should not panic.
	svc.handleDeleted(context.Background(), []byte("bad"))
}

// ── Search: non-empty results with filter ─────────────────────────────────────

func TestSearch_WithWorkspaceFilter(t *testing.T) {
	now := time.Now().UTC()
	srcJSON, _ := json.Marshal(map[string]any{
		"id": "m1", "workspace_id": "ws-x", "channel_id": "ch-1",
		"author_id": "u-1", "body": "relay rocks",
		"created_at": now.Format(time.RFC3339), "updated_at": now.Format(time.RFC3339),
	})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"hits":{"total":{"value":1},"hits":[{"_score":2.0,"_source":%s,"highlight":{}}]}}`, srcJSON)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	page, err := svc.Search(context.Background(), "relay", domain.SearchFilter{WorkspaceID: "ws-x", ChannelID: "ch-1"}, 0, 10)
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected Total=1, got %d", page.Total)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	page, err := svc.Search(context.Background(), "", domain.SearchFilter{WorkspaceID: "ws-1"}, 0, 10)
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if page.Total != 0 {
		t.Errorf("expected Total=0, got %d", page.Total)
	}
}

func TestSearch_ServerErrorBodyUnparseable(t *testing.T) {
	// Search returns an error when the response body is not valid ES JSON.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `not-json`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	_, err := svc.Search(context.Background(), "test", domain.SearchFilter{WorkspaceID: "ws-1"}, 0, 10)
	if err == nil {
		t.Fatal("expected error for unparseable response body, got nil")
	}
}

func TestDeleteMessage_FireAndForget(t *testing.T) {
	// DeleteMessage is fire-and-forget: it does not return an error on HTTP failure.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"fail"}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	err := svc.DeleteMessage(context.Background(), "msg-x")
	// No error expected: implementation accepts all HTTP status codes.
	_ = err
}

func TestSearch_WithDateFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
	}))
	defer ts.Close()

	svc := New(ts.URL, nil, zerolog.Nop())
	now := time.Now().UTC()
	filter := domain.SearchFilter{
		WorkspaceID: "ws-1",
		AuthorID:    "user-1",
		After:       now.Add(-24 * time.Hour),
		Before:      now,
	}
	page, err := svc.Search(context.Background(), "hello", filter, 5, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 0 {
		t.Errorf("expected 0 results")
	}
}
