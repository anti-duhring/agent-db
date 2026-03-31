package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository/postgres"
	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go"
)

// setupTestRepo starts a real Postgres container and returns a PostgresRepository
// along with a cleanup function.
func setupTestRepo(t *testing.T) (*postgres.PostgresRepository, func()) {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("agentdb"),
		tcpostgres.WithUsername("bench"),
		tcpostgres.WithPassword("bench"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		testcontainers.TerminateContainer(ctr)
		t.Fatalf("failed to get connection string: %v", err)
	}

	repo, err := postgres.New(ctx, connStr)
	if err != nil {
		testcontainers.TerminateContainer(ctr)
		t.Fatalf("failed to create PostgresRepository: %v", err)
	}

	cleanup := func() {
		repo.Close()
		testcontainers.TerminateContainer(ctr)
	}

	return repo, cleanup
}

// Test 1: CreateConversation returns a Conversation with matching partnerID/userID
// and non-zero timestamps.
func TestCreateConversation(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	partnerID := uuid.New()
	userID := uuid.New()

	conv, err := repo.CreateConversation(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}

	if conv.ID == uuid.Nil {
		t.Error("expected non-nil conversation ID")
	}
	if conv.PartnerID != partnerID {
		t.Errorf("partnerID mismatch: got %v, want %v", conv.PartnerID, partnerID)
	}
	if conv.UserID != userID {
		t.Errorf("userID mismatch: got %v, want %v", conv.UserID, userID)
	}
	if conv.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if conv.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

// Test 2: AppendMessage returns a Message with correct conversationID, role,
// content, and estimated token count (len/4).
func TestAppendMessage_ReturnsCorrectFields(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	content := "Hello, this is a test message for the benchmark."
	msg, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, content)
	if err != nil {
		t.Fatalf("AppendMessage returned error: %v", err)
	}

	if msg.ID == uuid.Nil {
		t.Error("expected non-nil message ID")
	}
	if msg.ConversationID != conv.ID {
		t.Errorf("conversationID mismatch: got %v, want %v", msg.ConversationID, conv.ID)
	}
	if msg.Role != domain.RoleUser {
		t.Errorf("role mismatch: got %v, want %v", msg.Role, domain.RoleUser)
	}
	if msg.Content != content {
		t.Errorf("content mismatch: got %v, want %v", msg.Content, content)
	}
	expectedTokenCount := len(content) / 4
	if msg.TokenCount != expectedTokenCount {
		t.Errorf("token count mismatch: got %d, want %d", msg.TokenCount, expectedTokenCount)
	}
	if msg.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

// Test 3: AppendMessage updates the parent conversation's UpdatedAt field.
func TestAppendMessage_UpdatesConversationUpdatedAt(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	partnerID := uuid.New()
	userID := uuid.New()

	conv, err := repo.CreateConversation(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	originalUpdatedAt := conv.UpdatedAt

	// Sleep briefly to ensure time difference is detectable
	time.Sleep(10 * time.Millisecond)

	_, err = repo.AppendMessage(ctx, conv.ID, domain.RoleUser, "test message")
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	// Retrieve conversations to check UpdatedAt changed
	convs, err := repo.ListConversations(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convs))
	}

	if !convs[0].UpdatedAt.After(originalUpdatedAt) {
		t.Errorf("UpdatedAt was not updated: original=%v, after=%v", originalUpdatedAt, convs[0].UpdatedAt)
	}
}

// Test 4: LoadWindow returns last N messages in oldest-first order from a
// conversation with more than N messages.
func TestLoadWindow_ReturnsLastNMessagesOldestFirst(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	// Append 5 messages
	var messages []domain.Message
	for i := 0; i < 5; i++ {
		msg, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, "message content")
		if err != nil {
			t.Fatalf("AppendMessage %d: %v", i, err)
		}
		messages = append(messages, msg)
		// Brief sleep to ensure distinct timestamps
		time.Sleep(2 * time.Millisecond)
	}

	// Load window of 3 (should return last 3 messages: index 2, 3, 4)
	window, err := repo.LoadWindow(ctx, conv.ID, 3)
	if err != nil {
		t.Fatalf("LoadWindow: %v", err)
	}

	if len(window) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(window))
	}

	// Verify oldest-first order: window[0] should be before window[1] before window[2]
	for i := 1; i < len(window); i++ {
		if window[i].CreatedAt.Before(window[i-1].CreatedAt) {
			t.Errorf("messages not in oldest-first order at index %d: %v before %v", i, window[i].CreatedAt, window[i-1].CreatedAt)
		}
	}

	// Verify we got the LAST 3 messages (most recent subset)
	// The last 3 messages appended are messages[2], messages[3], messages[4]
	if window[0].ID != messages[2].ID {
		t.Errorf("expected first window message to be messages[2], got %v (want %v)", window[0].ID, messages[2].ID)
	}
	if window[2].ID != messages[4].ID {
		t.Errorf("expected last window message to be messages[4], got %v (want %v)", window[2].ID, messages[4].ID)
	}
}

// Test 5: LoadWindow returns all messages when conversation has fewer than N messages.
func TestLoadWindow_ReturnsAllWhenFewerThanN(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	// Append 2 messages
	for i := 0; i < 2; i++ {
		_, err := repo.AppendMessage(ctx, conv.ID, domain.RoleAssistant, "response content")
		if err != nil {
			t.Fatalf("AppendMessage %d: %v", i, err)
		}
	}

	// Request 10 messages — should return only 2
	window, err := repo.LoadWindow(ctx, conv.ID, 10)
	if err != nil {
		t.Fatalf("LoadWindow: %v", err)
	}

	if len(window) != 2 {
		t.Errorf("expected 2 messages, got %d", len(window))
	}
}

// Test 6: ListConversations returns only conversations for the specified
// (partnerID, userID) pair, sorted by UpdatedAt DESC.
func TestListConversations_FiltersAndSortsCorrectly(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	partnerID := uuid.New()
	userID := uuid.New()
	otherPartnerID := uuid.New()
	otherUserID := uuid.New()

	// Create 2 conversations for our target (partnerID, userID) pair
	conv1, err := repo.CreateConversation(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("CreateConversation conv1: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	conv2, err := repo.CreateConversation(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("CreateConversation conv2: %v", err)
	}

	// Create conversations for other users (should NOT appear in our query)
	_, err = repo.CreateConversation(ctx, otherPartnerID, userID)
	if err != nil {
		t.Fatalf("CreateConversation other1: %v", err)
	}
	_, err = repo.CreateConversation(ctx, partnerID, otherUserID)
	if err != nil {
		t.Fatalf("CreateConversation other2: %v", err)
	}

	// Append a message to conv1 to make it more recent
	time.Sleep(5 * time.Millisecond)
	_, err = repo.AppendMessage(ctx, conv1.ID, domain.RoleUser, "bump conv1")
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	convs, err := repo.ListConversations(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}

	if len(convs) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(convs))
	}

	// conv1 was updated more recently, so it should appear first
	if convs[0].ID != conv1.ID {
		t.Errorf("expected conv1 first (most recently updated), got %v", convs[0].ID)
	}
	if convs[1].ID != conv2.ID {
		t.Errorf("expected conv2 second, got %v", convs[1].ID)
	}

	// Verify sorted DESC by UpdatedAt
	if convs[0].UpdatedAt.Before(convs[1].UpdatedAt) {
		t.Errorf("conversations not sorted by UpdatedAt DESC: convs[0]=%v, convs[1]=%v", convs[0].UpdatedAt, convs[1].UpdatedAt)
	}
}

// Test 7: ListConversations returns empty slice (not nil) when no conversations match.
func TestListConversations_ReturnsEmptySliceNotNil(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	partnerID := uuid.New()
	userID := uuid.New()

	convs, err := repo.ListConversations(ctx, partnerID, userID)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}

	if convs == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(convs) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(convs))
	}
}
