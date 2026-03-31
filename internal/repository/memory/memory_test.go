package memory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository/memory"
	"github.com/google/uuid"
)

func TestCreateConversation_ReturnsValidConversation(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()
	partnerID := uuid.New()
	userID := uuid.New()

	conv, err := repo.CreateConversation(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conv.ID == (uuid.UUID{}) {
		t.Error("expected non-zero UUID for conversation ID")
	}
	if conv.PartnerID != partnerID {
		t.Errorf("expected PartnerID %v, got %v", partnerID, conv.PartnerID)
	}
	if conv.UserID != userID {
		t.Errorf("expected UserID %v, got %v", userID, conv.UserID)
	}
	if conv.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if conv.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestAppendMessage_ReturnsValidMessage(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	conv, _ := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	content := "hello world this is a test message"

	msg, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.ID == (uuid.UUID{}) {
		t.Error("expected non-zero UUID for message ID")
	}
	if msg.ConversationID != conv.ID {
		t.Errorf("expected ConversationID %v, got %v", conv.ID, msg.ConversationID)
	}
	if msg.Role != domain.RoleUser {
		t.Errorf("expected Role %v, got %v", domain.RoleUser, msg.Role)
	}
	if msg.Content != content {
		t.Errorf("expected Content %q, got %q", content, msg.Content)
	}
	expectedTokenCount := len(content) / 4
	if msg.TokenCount != expectedTokenCount {
		t.Errorf("expected TokenCount %d (len/4), got %d", expectedTokenCount, msg.TokenCount)
	}
	if msg.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestAppendMessage_NonExistentConversation_ReturnsError(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	_, err := repo.AppendMessage(ctx, uuid.New(), domain.RoleUser, "hello")
	if err == nil {
		t.Error("expected error for non-existent conversation, got nil")
	}
}

func TestAppendMessage_UpdatesConversationUpdatedAt(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	conv, _ := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	before := conv.UpdatedAt

	// Small sleep to ensure time difference
	time.Sleep(1 * time.Millisecond)
	_, err := repo.AppendMessage(ctx, conv.ID, domain.RoleAssistant, "response")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify via ListConversations that UpdatedAt changed
	convs, err := repo.ListConversations(ctx, conv.PartnerID, conv.UserID)
	if err != nil {
		t.Fatalf("unexpected error listing: %v", err)
	}
	if len(convs) == 0 {
		t.Fatal("expected at least one conversation")
	}
	if !convs[0].UpdatedAt.After(before) {
		t.Errorf("expected UpdatedAt to be updated after AppendMessage: before=%v, after=%v", before, convs[0].UpdatedAt)
	}
}

func TestLoadWindow_ReturnsLastNMessages(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	conv, _ := repo.CreateConversation(ctx, uuid.New(), uuid.New())

	// Append 10 messages
	var allMsgs []domain.Message
	for i := 0; i < 10; i++ {
		content := "message"
		msg, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, content)
		if err != nil {
			t.Fatalf("unexpected error appending message %d: %v", i, err)
		}
		allMsgs = append(allMsgs, msg)
	}

	// Load last 3
	window, err := repo.LoadWindow(ctx, conv.ID, 3)
	if err != nil {
		t.Fatalf("unexpected error loading window: %v", err)
	}

	if len(window) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(window))
	}

	// Should be the last 3 messages in chronological order (oldest first)
	for i, msg := range window {
		expected := allMsgs[7+i]
		if msg.ID != expected.ID {
			t.Errorf("window[%d]: expected message ID %v, got %v", i, expected.ID, msg.ID)
		}
	}
}

func TestLoadWindow_ReturnsAllWhenNExceedsCount(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	conv, _ := repo.CreateConversation(ctx, uuid.New(), uuid.New())

	// Append 5 messages
	for i := 0; i < 5; i++ {
		_, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, "msg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Request 20 (more than available)
	window, err := repo.LoadWindow(ctx, conv.ID, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(window) != 5 {
		t.Errorf("expected 5 messages, got %d", len(window))
	}
}

func TestLoadWindow_NonExistentConversation_ReturnsError(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	_, err := repo.LoadWindow(ctx, uuid.New(), 10)
	if err == nil {
		t.Error("expected error for non-existent conversation, got nil")
	}
}

func TestListConversations_FiltersCorrectly(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	partnerID := uuid.New()
	userID := uuid.New()

	// Create 2 conversations for the target pair
	_, _ = repo.CreateConversation(ctx, partnerID, userID)
	_, _ = repo.CreateConversation(ctx, partnerID, userID)

	// Create 1 conversation for a different pair
	_, _ = repo.CreateConversation(ctx, uuid.New(), uuid.New())

	result, err := repo.ListConversations(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(result))
	}
	for _, c := range result {
		if c.PartnerID != partnerID || c.UserID != userID {
			t.Errorf("unexpected conversation in result: partnerID=%v userID=%v", c.PartnerID, c.UserID)
		}
	}
}

func TestListConversations_SortsByUpdatedAtDescending(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	partnerID := uuid.New()
	userID := uuid.New()

	conv1, _ := repo.CreateConversation(ctx, partnerID, userID)
	time.Sleep(2 * time.Millisecond)
	conv2, _ := repo.CreateConversation(ctx, partnerID, userID)
	time.Sleep(2 * time.Millisecond)

	// Append to conv1 to make it most recently active
	_, _ = repo.AppendMessage(ctx, conv1.ID, domain.RoleUser, "latest update")

	result, err := repo.ListConversations(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(result))
	}

	// conv1 was updated last, so it should be first
	if result[0].ID != conv1.ID {
		t.Errorf("expected most recently active conversation first (conv1=%v), got %v", conv1.ID, result[0].ID)
	}
	if result[1].ID != conv2.ID {
		t.Errorf("expected conv2=%v second, got %v", conv2.ID, result[1].ID)
	}
}

func TestListConversations_ReturnsEmptySliceNotNil(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	result, err := repo.ListConversations(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(result))
	}
}

func TestConcurrentAccess_NoDataRace(t *testing.T) {
	repo := memory.New()
	ctx := context.Background()

	conv, _ := repo.CreateConversation(ctx, uuid.New(), uuid.New())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = repo.AppendMessage(ctx, conv.ID, domain.RoleUser, "concurrent message")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = repo.LoadWindow(ctx, conv.ID, 5)
		}()
	}
	wg.Wait()
}
