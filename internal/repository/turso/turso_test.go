package turso_test

import (
	"context"
	"os"
	"testing"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository/turso"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTurso creates a TursoRepository for testing.
// Tests are skipped when TURSO_URL and TURSO_AUTH_TOKEN are not set.
func setupTurso(t *testing.T) *turso.TursoRepository {
	t.Helper()
	url := os.Getenv("TURSO_URL")
	token := os.Getenv("TURSO_AUTH_TOKEN")
	if url == "" || token == "" {
		t.Skip("Skipping Turso tests: TURSO_URL and TURSO_AUTH_TOKEN not set")
	}
	ctx := context.Background()
	repo, err := turso.New(ctx, url, token)
	require.NoError(t, err)
	t.Cleanup(func() { repo.Close() })
	return repo
}

func TestTurso_CreateConversation(t *testing.T) {
	repo := setupTurso(t)
	ctx := context.Background()

	partnerID := uuid.New()
	userID := uuid.New()

	conv, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, conv.ID)
	assert.Equal(t, partnerID, conv.PartnerID)
	assert.Equal(t, userID, conv.UserID)
	assert.False(t, conv.CreatedAt.IsZero())
	assert.False(t, conv.UpdatedAt.IsZero())
}

func TestTurso_AppendMessage(t *testing.T) {
	repo := setupTurso(t)
	ctx := context.Background()

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	content := "Hello, world! This is a test message."
	msg, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, content)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, msg.ID)
	assert.Equal(t, conv.ID, msg.ConversationID)
	assert.Equal(t, domain.RoleUser, msg.Role)
	assert.Equal(t, content, msg.Content)
	assert.Equal(t, len(content)/4, msg.TokenCount)
	assert.False(t, msg.CreatedAt.IsZero())
}

func TestTurso_LoadWindow(t *testing.T) {
	repo := setupTurso(t)
	ctx := context.Background()

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	// Append 5 messages.
	messages := []string{"msg1", "msg2", "msg3", "msg4", "msg5"}
	for _, content := range messages {
		_, err := repo.AppendMessage(ctx, conv.ID, domain.RoleUser, content)
		require.NoError(t, err)
	}

	// LoadWindow with n=3 should return the last 3 messages, oldest-first.
	window, err := repo.LoadWindow(ctx, conv.ID, 3)
	require.NoError(t, err)
	require.Len(t, window, 3)

	// Verify oldest-first order and content matches last 3 appended.
	assert.Equal(t, "msg3", window[0].Content)
	assert.Equal(t, "msg4", window[1].Content)
	assert.Equal(t, "msg5", window[2].Content)

	// Verify timestamps are in ascending order (oldest-first).
	assert.True(t, !window[0].CreatedAt.After(window[1].CreatedAt))
	assert.True(t, !window[1].CreatedAt.After(window[2].CreatedAt))
}

func TestTurso_ListConversations(t *testing.T) {
	repo := setupTurso(t)
	ctx := context.Background()

	// Use unique partnerID/userID to isolate this test from stale data.
	partnerID := uuid.New()
	userID := uuid.New()

	// Create 3 conversations.
	conv1, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)
	conv2, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)
	conv3, err := repo.CreateConversation(ctx, partnerID, userID)
	require.NoError(t, err)

	// Append a message to conv2, making it the most recently updated.
	_, err = repo.AppendMessage(ctx, conv2.ID, domain.RoleAssistant, "reply")
	require.NoError(t, err)

	convs, err := repo.ListConversations(ctx, partnerID, userID)
	require.NoError(t, err)
	require.Len(t, convs, 3)

	// conv2 should be first (most recently updated).
	assert.Equal(t, conv2.ID, convs[0].ID)

	// conv1 and conv3 are in the list (order between them not strictly defined,
	// but both must be present).
	ids := map[uuid.UUID]bool{
		convs[1].ID: true,
		convs[2].ID: true,
	}
	assert.True(t, ids[conv1.ID])
	assert.True(t, ids[conv3.ID])
}

func TestTurso_LoadWindow_Empty(t *testing.T) {
	repo := setupTurso(t)
	ctx := context.Background()

	conv, err := repo.CreateConversation(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)

	msgs, err := repo.LoadWindow(ctx, conv.ID, 10)
	require.NoError(t, err)
	assert.NotNil(t, msgs)
	assert.Len(t, msgs, 0)
}

func TestTurso_ListConversations_Empty(t *testing.T) {
	repo := setupTurso(t)
	ctx := context.Background()

	// Use random UUIDs — no conversations should exist for this pair.
	convs, err := repo.ListConversations(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, convs)
	assert.Len(t, convs, 0)
}
