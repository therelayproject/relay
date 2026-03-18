package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/relay-im/relay/services/message-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// ── parseBody tests ───────────────────────────────────────────────────────────

func TestParseBody_NoMentions(t *testing.T) {
	raw := parseBody("Hello everyone, no mentions here!")
	var p struct {
		Raw      string   `json:"raw"`
		Mentions []string `json:"mentions"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Raw != "Hello everyone, no mentions here!" {
		t.Errorf("unexpected raw: %q", p.Raw)
	}
	if len(p.Mentions) != 0 {
		t.Errorf("expected no mentions, got %v", p.Mentions)
	}
}

func TestParseBody_SingleMention(t *testing.T) {
	raw := parseBody("Hey @alice, how are you?")
	var p struct {
		Mentions []string `json:"mentions"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Mentions) != 1 || p.Mentions[0] != "alice" {
		t.Errorf("expected [alice], got %v", p.Mentions)
	}
}

func TestParseBody_MultipleMentions(t *testing.T) {
	raw := parseBody("@alice and @bob should review this with @carol")
	var p struct {
		Mentions []string `json:"mentions"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Mentions) != 3 {
		t.Errorf("expected 3 mentions, got %v", p.Mentions)
	}
}

func TestParseBody_DeduplicatesMentions(t *testing.T) {
	raw := parseBody("@alice @alice @alice triple mention")
	var p struct {
		Mentions []string `json:"mentions"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Mentions) != 1 {
		t.Errorf("expected 1 unique mention, got %v", p.Mentions)
	}
}

func TestParseBody_EmptyBody(t *testing.T) {
	raw := parseBody("")
	if len(raw) == 0 {
		t.Fatal("expected non-empty JSON for empty body")
	}
	var p struct {
		Mentions []string `json:"mentions"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// ── Send validation tests (fails before first repo call) ─────────────────────

func newTestMessageService() *MessageService {
	return &MessageService{repo: nil, nc: nil}
}

func TestSend_EmptyBody(t *testing.T) {
	svc := newTestMessageService()
	_, err := svc.Send(context.Background(), "ch-1", "user-1", "   ", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
	assertMsgCode(t, err, apperrors.CodeInvalidArgument)
}

func TestSend_WhitespaceBody(t *testing.T) {
	svc := newTestMessageService()
	_, err := svc.Send(context.Background(), "ch-1", "user-1", "\t\n  ", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for whitespace-only body, got nil")
	}
	assertMsgCode(t, err, apperrors.CodeInvalidArgument)
}

func TestSend_BodyTooLong(t *testing.T) {
	svc := newTestMessageService()
	longBody := strings.Repeat("x", 40001)
	_, err := svc.Send(context.Background(), "ch-1", "user-1", longBody, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for body > 40000 chars, got nil")
	}
	assertMsgCode(t, err, apperrors.CodeInvalidArgument)
}

func TestSend_Body40000Chars_PassesValidation(t *testing.T) {
	// 40000 chars is at the limit and must NOT be rejected by validation.
	// We wrap the call in recover to handle the nil-repo panic that happens
	// after validation succeeds.
	svc := newTestMessageService()
	body := strings.Repeat("x", 40000)

	didPanic := false
	var validationErr bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		_, err := svc.Send(context.Background(), "ch-1", "user-1", body, nil, nil, nil)
		if err != nil {
			var ae *apperrors.AppError
			if errors.As(err, &ae) && ae.Code == apperrors.CodeInvalidArgument {
				validationErr = true
			}
		}
	}()

	if validationErr {
		t.Fatal("40000-char body should NOT be rejected by validation")
	}
	// A panic (nil-repo) is acceptable here — it means validation passed.
	_ = didPanic
}

// ── Edit validation ───────────────────────────────────────────────────────────

func TestEdit_EmptyBody(t *testing.T) {
	svc := newTestMessageService()
	_, err := svc.Edit(context.Background(), "msg-1", "user-1", "")
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
	assertMsgCode(t, err, apperrors.CodeInvalidArgument)
}

// ── AddReaction validation ────────────────────────────────────────────────────

func TestAddReaction_EmptyEmoji(t *testing.T) {
	svc := newTestMessageService()
	err := svc.AddReaction(context.Background(), "msg-1", "ch-1", "user-1", "")
	if err == nil {
		t.Fatal("expected error for empty emoji, got nil")
	}
	assertMsgCode(t, err, apperrors.CodeInvalidArgument)
}

// ── helper ────────────────────────────────────────────────────────────────────

func assertMsgCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Errorf("expected *apperrors.AppError, got %T: %v", err, err)
		return
	}
	if ae.Code != want {
		t.Errorf("error code: got %q, want %q", ae.Code, want)
	}
}

// ── mock repository ───────────────────────────────────────────────────────────

type mockMessageRepo struct {
	createFn             func(ctx context.Context, m *domain.Message) (*domain.Message, error)
	getByIDFn            func(ctx context.Context, id string) (*domain.Message, error)
	listByChannelFn      func(ctx context.Context, channelID string, limit int, cursor string) (*domain.Page, error)
	listThreadFn         func(ctx context.Context, threadID string, limit int, cursor string) (*domain.Page, error)
	updateFn             func(ctx context.Context, id, authorID, body string, bodyParsed json.RawMessage) (*domain.Message, error)
	softDeleteFn         func(ctx context.Context, id, authorID string) error
	incrementReplyFn     func(ctx context.Context, messageID string) error
	addReactionFn        func(ctx context.Context, rx *domain.Reaction) error
	removeReactionFn     func(ctx context.Context, messageID, userID, emoji string) error
	listReactionsFn      func(ctx context.Context, messageID string) ([]domain.ReactionSummary, error)
	pinMessageFn         func(ctx context.Context, pin *domain.Pin) error
	unpinMessageFn       func(ctx context.Context, channelID, messageID string) error
	listPinsFn           func(ctx context.Context, channelID string) ([]*domain.Pin, error)
}

func (m *mockMessageRepo) Create(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	if m.createFn != nil {
		return m.createFn(ctx, msg)
	}
	msg.ID = "msg-created"
	return msg, nil
}
func (m *mockMessageRepo) GetByID(ctx context.Context, id string) (*domain.Message, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &domain.Message{ID: id, AuthorID: "user-1", ChannelID: "ch-1"}, nil
}
func (m *mockMessageRepo) ListByChannel(ctx context.Context, channelID string, limit int, cursor string) (*domain.Page, error) {
	if m.listByChannelFn != nil {
		return m.listByChannelFn(ctx, channelID, limit, cursor)
	}
	return &domain.Page{}, nil
}
func (m *mockMessageRepo) ListThread(ctx context.Context, threadID string, limit int, cursor string) (*domain.Page, error) {
	if m.listThreadFn != nil {
		return m.listThreadFn(ctx, threadID, limit, cursor)
	}
	return &domain.Page{}, nil
}
func (m *mockMessageRepo) Update(ctx context.Context, id, authorID, body string, bodyParsed json.RawMessage) (*domain.Message, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, authorID, body, bodyParsed)
	}
	return &domain.Message{ID: id, Body: body, ChannelID: "ch-1"}, nil
}
func (m *mockMessageRepo) SoftDelete(ctx context.Context, id, authorID string) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id, authorID)
	}
	return nil
}
func (m *mockMessageRepo) IncrementReplyCount(ctx context.Context, messageID string) error {
	if m.incrementReplyFn != nil {
		return m.incrementReplyFn(ctx, messageID)
	}
	return nil
}
func (m *mockMessageRepo) AddReaction(ctx context.Context, rx *domain.Reaction) error {
	if m.addReactionFn != nil {
		return m.addReactionFn(ctx, rx)
	}
	return nil
}
func (m *mockMessageRepo) RemoveReaction(ctx context.Context, messageID, userID, emoji string) error {
	if m.removeReactionFn != nil {
		return m.removeReactionFn(ctx, messageID, userID, emoji)
	}
	return nil
}
func (m *mockMessageRepo) ListReactions(ctx context.Context, messageID string) ([]domain.ReactionSummary, error) {
	if m.listReactionsFn != nil {
		return m.listReactionsFn(ctx, messageID)
	}
	return []domain.ReactionSummary{}, nil
}
func (m *mockMessageRepo) PinMessage(ctx context.Context, pin *domain.Pin) error {
	if m.pinMessageFn != nil {
		return m.pinMessageFn(ctx, pin)
	}
	return nil
}
func (m *mockMessageRepo) UnpinMessage(ctx context.Context, channelID, messageID string) error {
	if m.unpinMessageFn != nil {
		return m.unpinMessageFn(ctx, channelID, messageID)
	}
	return nil
}
func (m *mockMessageRepo) ListPins(ctx context.Context, channelID string) ([]*domain.Pin, error) {
	if m.listPinsFn != nil {
		return m.listPinsFn(ctx, channelID)
	}
	return []*domain.Pin{}, nil
}

func newMockedMsgSvc(repo MessageRepository) *MessageService {
	return &MessageService{repo: repo, nc: nil}
}

// ── Send with mock ────────────────────────────────────────────────────────────

func TestSend_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	msg, err := svc.Send(context.Background(), "ch-1", "user-1", "Hello!", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Body != "Hello!" {
		t.Errorf("unexpected body: %q", msg.Body)
	}
}

func TestSend_WithThreadID(t *testing.T) {
	replyCounted := false
	threadID := "thread-root"
	repo := &mockMessageRepo{
		incrementReplyFn: func(_ context.Context, _ string) error {
			replyCounted = true
			return nil
		},
	}
	svc := newMockedMsgSvc(repo)
	_, err := svc.Send(context.Background(), "ch-1", "user-1", "reply!", &threadID, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !replyCounted {
		t.Error("expected IncrementReplyCount to be called for threaded message")
	}
}

// ── Edit with mock ────────────────────────────────────────────────────────────

func TestEdit_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	msg, err := svc.Edit(context.Background(), "msg-1", "user-1", "updated body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ID != "msg-1" {
		t.Errorf("unexpected ID: %q", msg.ID)
	}
}

// ── Delete with mock ──────────────────────────────────────────────────────────

func TestDelete_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	err := svc.Delete(context.Background(), "msg-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_WrongAuthor(t *testing.T) {
	repo := &mockMessageRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Message, error) {
			return &domain.Message{ID: id, AuthorID: "owner", ChannelID: "ch-1"}, nil
		},
	}
	svc := newMockedMsgSvc(repo)
	err := svc.Delete(context.Background(), "msg-1", "other-user")
	if err == nil {
		t.Fatal("expected permission denied for wrong author")
	}
	assertMsgCode(t, err, apperrors.CodePermissionDenied)
}

// ── AddReaction with mock ─────────────────────────────────────────────────────

func TestAddReaction_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	err := svc.AddReaction(context.Background(), "msg-1", "ch-1", "user-1", "👍")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddReaction_WithMatchingReactionCount(t *testing.T) {
	repo := &mockMessageRepo{
		listReactionsFn: func(_ context.Context, _ string) ([]domain.ReactionSummary, error) {
			return []domain.ReactionSummary{{Emoji: "👍", Count: 3}}, nil
		},
	}
	svc := newMockedMsgSvc(repo)
	err := svc.AddReaction(context.Background(), "msg-1", "ch-1", "user-1", "👍")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── RemoveReaction with mock ──────────────────────────────────────────────────

func TestRemoveReaction_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	err := svc.RemoveReaction(context.Background(), "msg-1", "ch-1", "user-1", "👍")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── List/GetThread/ListReactions/GetByID with mock ────────────────────────────

func TestList_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	page, err := svc.List(context.Background(), "ch-1", 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = page
}

func TestGetThread_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	_, err := svc.GetThread(context.Background(), "thread-1", 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListReactions_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	_, err := svc.ListReactions(context.Background(), "msg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetByID_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	msg, err := svc.GetByID(context.Background(), "msg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ID != "msg-1" {
		t.Errorf("unexpected ID: %q", msg.ID)
	}
}

// ── PinMessage / UnpinMessage / ListPins with mock ───────────────────────────

func TestPinMessage_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	err := svc.PinMessage(context.Background(), "ch-1", "msg-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnpinMessage_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	if err := svc.UnpinMessage(context.Background(), "ch-1", "msg-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPins_Success(t *testing.T) {
	svc := newMockedMsgSvc(&mockMessageRepo{})
	pins, err := svc.ListPins(context.Background(), "ch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = pins
}
