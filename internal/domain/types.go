package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role is a typed string constant for chat message roles.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Conversation represents a chat conversation scoped to a partner and user.
type Conversation struct {
	ID        uuid.UUID
	PartnerID uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message represents a single message within a conversation.
type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	Role           Role
	Content        string
	TokenCount     int
	CreatedAt      time.Time
}
