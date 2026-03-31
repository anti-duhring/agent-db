package repository

import (
	"context"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/google/uuid"
)

// ChatRepository defines the common interface for all chat storage backends.
// All methods take context.Context as the first argument and return (result, error).
type ChatRepository interface {
	// CreateConversation creates a new conversation scoped to the given partner and user.
	CreateConversation(ctx context.Context, partnerID, userID uuid.UUID) (domain.Conversation, error)

	// AppendMessage appends a new message to the specified conversation.
	AppendMessage(ctx context.Context, conversationID uuid.UUID, role domain.Role, content string) (domain.Message, error)

	// LoadWindow returns the last n messages from the specified conversation,
	// ordered from oldest to newest.
	LoadWindow(ctx context.Context, conversationID uuid.UUID, n int) ([]domain.Message, error)

	// ListConversations returns all conversations for the given (partnerID, userID) pair,
	// sorted by last activity (most recent first).
	ListConversations(ctx context.Context, partnerID, userID uuid.UUID) ([]domain.Conversation, error)
}
