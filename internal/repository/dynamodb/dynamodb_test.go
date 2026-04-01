package dynamodb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	dynamodbrepo "github.com/anti-duhring/agent-db/internal/repository/dynamodb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
)

// setupLocalStack starts a LocalStack container and returns a DynamoDBRepository backed by it.
func setupLocalStack(t *testing.T) *dynamodbrepo.DynamoDBRepository {
	t.Helper()
	ctx := context.Background()

	ctr, err := localstack.Run(ctx, "localstack/localstack:3.8")
	require.NoError(t, err)
	t.Cleanup(func() { testcontainers.TerminateContainer(ctr) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "4566/tcp")
	require.NoError(t, err)
	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	repo, err := dynamodbrepo.New(ctx, endpoint)
	require.NoError(t, err)
	return repo
}

// TestDynamoDB_CreateConversation verifies that CreateConversation returns a Conversation
// with matching partnerID/userID and non-zero IDs and timestamps.
func TestDynamoDB_CreateConversation(t *testing.T) {
	repo := setupLocalStack(t)
	ctx := context.Background()

	partnerID := uuid.New()
	userID := uuid.New()

	conv, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, conv.ID, "expected non-nil conversation ID")
	assert.Equal(t, partnerID, conv.PartnerID)
	assert.Equal(t, userID, conv.UserID)
	assert.False(t, conv.CreatedAt.IsZero(), "expected non-zero CreatedAt")
	assert.False(t, conv.UpdatedAt.IsZero(), "expected non-zero UpdatedAt")
}

// TestDynamoDB_AppendMessage verifies that AppendMessage returns a Message with the
// correct ConversationID, Role, Content, TokenCount (len/4), and non-zero CreatedAt.
func TestDynamoDB_AppendMessage(t *testing.T) {
	repo := setupLocalStack(t)
	ctx := context.Background()

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	content := "hello"
	msg, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, content)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, msg.ID, "expected non-nil message ID")
	assert.Equal(t, conv.ID, msg.ConversationID)
	assert.Equal(t, domain.RoleUser, msg.Role)
	assert.Equal(t, content, msg.Content)
	assert.Equal(t, len(content)/4, msg.TokenCount)
	assert.False(t, msg.CreatedAt.IsZero(), "expected non-zero CreatedAt")
}

// TestDynamoDB_LoadWindow verifies that LoadWindow returns the last N messages
// in oldest-first order.
func TestDynamoDB_LoadWindow(t *testing.T) {
	repo := setupLocalStack(t)
	ctx := context.Background()

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	// Append 5 messages with small sleeps to ensure distinct timestamps
	var appended []domain.Message
	for i := 0; i < 5; i++ {
		msg, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, fmt.Sprintf("message %d", i))
		require.NoError(t, err)
		appended = append(appended, msg)
		time.Sleep(2 * time.Millisecond)
	}

	// LoadWindow(3) should return the last 3 messages
	window, err := repo.LoadWindow(ctx, conv.ID, 3)
	require.NoError(t, err)

	assert.Len(t, window, 3, "expected exactly 3 messages")

	// Verify oldest-first order
	for i := 1; i < len(window); i++ {
		assert.False(t, window[i].CreatedAt.Before(window[i-1].CreatedAt),
			"messages not in oldest-first order at index %d", i)
	}

	// Verify we got the LAST 3 appended (indices 2, 3, 4)
	assert.Equal(t, appended[2].ID, window[0].ID, "expected window[0] to be appended[2]")
	assert.Equal(t, appended[4].ID, window[2].ID, "expected window[2] to be appended[4]")
}

// TestDynamoDB_ListConversations verifies that ListConversations returns all conversations
// for the given (partnerID, userID) pair, sorted by most recently updated first.
func TestDynamoDB_ListConversations(t *testing.T) {
	repo := setupLocalStack(t)
	ctx := context.Background()

	partnerID := uuid.New()
	userID := uuid.New()

	// Create 3 conversations
	_, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	conv2, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	_, err = repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)

	// Append a message to conv2 to make it most recently updated
	_, err = repo.AppendMessage(ctx, conv2.ID, domain.RoleUser, "bump conv2")
	require.NoError(t, err)

	convs, err := repo.ListConversations(ctx, partnerID, userID)
	require.NoError(t, err)

	assert.Len(t, convs, 3, "expected 3 conversations")

	// conv2 was most recently updated (got a message appended after conv3 was created)
	assert.Equal(t, conv2.ID, convs[0].ID, "expected conv2 first (most recently updated)")
}

// TestDynamoDB_AppendMessage_UpdatesListing verifies that AppendMessage updates the
// conversation's UpdatedAt (proves SK rotation via transactional write works correctly).
func TestDynamoDB_AppendMessage_UpdatesListing(t *testing.T) {
	repo := setupLocalStack(t)
	ctx := context.Background()

	partnerID := uuid.New()
	userID := uuid.New()

	conv, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)
	createdAt := conv.UpdatedAt

	time.Sleep(5 * time.Millisecond)

	_, err = repo.AppendMessage(ctx, conv.ID, domain.RoleUser, "a message")
	require.NoError(t, err)

	convs, err := repo.ListConversations(ctx, partnerID, userID)
	require.NoError(t, err)
	require.Len(t, convs, 1)

	// UpdatedAt should be after the original CreatedAt/UpdatedAt
	assert.True(t, convs[0].UpdatedAt.After(createdAt),
		"UpdatedAt (%v) should be after original (%v)", convs[0].UpdatedAt, createdAt)
}

// TestDynamoDB_LoadWindow_Empty verifies that LoadWindow returns an empty (non-nil) slice
// when the conversation has no messages.
func TestDynamoDB_LoadWindow_Empty(t *testing.T) {
	repo := setupLocalStack(t)
	ctx := context.Background()

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	msgs, err := repo.LoadWindow(ctx, conv.ID, 10)
	require.NoError(t, err)

	assert.NotNil(t, msgs, "expected empty slice, not nil")
	assert.Len(t, msgs, 0, "expected 0 messages")
}

// TestDynamoDB_ListConversations_Empty verifies that ListConversations returns an empty
// (non-nil) slice when no conversations exist for the given (partnerID, userID).
func TestDynamoDB_ListConversations_Empty(t *testing.T) {
	repo := setupLocalStack(t)
	ctx := context.Background()

	convs, err := repo.ListConversations(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	assert.NotNil(t, convs, "expected empty slice, not nil")
	assert.Len(t, convs, 0, "expected 0 conversations")
}
