package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
)

// Compile-time interface check: MemoryRepository must implement ChatRepository.
var _ repository.ChatRepository = (*MemoryRepository)(nil)

// MemoryRepository is an in-memory implementation of ChatRepository.
// It serves as the reference implementation and enables Phase 2 benchmark runner
// testing without database dependencies.
type MemoryRepository struct {
	mu            sync.RWMutex
	conversations map[uuid.UUID]domain.Conversation
	messages      map[uuid.UUID][]domain.Message
}

// New creates a new MemoryRepository with initialized maps.
func New() *MemoryRepository {
	return &MemoryRepository{
		conversations: make(map[uuid.UUID]domain.Conversation),
		messages:      make(map[uuid.UUID][]domain.Message),
	}
}

// CreateConversation creates a new conversation scoped to the given partner and user.
func (r *MemoryRepository) CreateConversation(ctx context.Context, partnerID, userID uuid.UUID) (domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := uuid.New()
	now := time.Now()
	conv := domain.Conversation{
		ID:        id,
		PartnerID: partnerID,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.conversations[id] = conv
	r.messages[id] = []domain.Message{}

	return conv, nil
}

// AppendMessage appends a new message to the specified conversation.
// Returns an error if the conversation does not exist.
func (r *MemoryRepository) AppendMessage(ctx context.Context, conversationID uuid.UUID, role domain.Role, content string) (domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conv, ok := r.conversations[conversationID]
	if !ok {
		return domain.Message{}, fmt.Errorf("conversation %s not found", conversationID)
	}

	id := uuid.New()
	now := time.Now()
	tokenCount := len(content) / 4

	msg := domain.Message{
		ID:             id,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		TokenCount:     tokenCount,
		CreatedAt:      now,
	}

	r.messages[conversationID] = append(r.messages[conversationID], msg)

	// Update the conversation's UpdatedAt (must re-assign since structs are values)
	conv.UpdatedAt = now
	r.conversations[conversationID] = conv

	return msg, nil
}

// LoadWindow returns the last n messages from the specified conversation,
// ordered from oldest to newest (chronological order).
// Returns an error if the conversation does not exist.
func (r *MemoryRepository) LoadWindow(ctx context.Context, conversationID uuid.UUID, n int) ([]domain.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.conversations[conversationID]; !ok {
		return nil, fmt.Errorf("conversation %s not found", conversationID)
	}

	msgs := r.messages[conversationID]

	// Return a copy to prevent callers from mutating internal state
	if n >= len(msgs) {
		result := make([]domain.Message, len(msgs))
		copy(result, msgs)
		return result, nil
	}

	start := len(msgs) - n
	result := make([]domain.Message, n)
	copy(result, msgs[start:])
	return result, nil
}

// ListConversations returns all conversations for the given (partnerID, userID) pair,
// sorted by last activity (most recently updated first).
// Returns an empty slice (not nil) if no conversations are found.
func (r *MemoryRepository) ListConversations(ctx context.Context, partnerID, userID uuid.UUID) ([]domain.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := []domain.Conversation{}

	for _, c := range r.conversations {
		if c.PartnerID == partnerID && c.UserID == userID {
			result = append(result, c)
		}
	}

	// Sort by UpdatedAt descending (most recently active first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result, nil
}
