package postgres

import (
	"context"
	_ "embed"
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_create_tables.sql
var schema string

// Compile-time interface check: PostgresRepository must implement ChatRepository.
var _ repository.ChatRepository = (*PostgresRepository)(nil)

// PostgresRepository implements ChatRepository using a pgxpool connection pool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// New creates a new PostgresRepository, applies the schema, and prepares statements
// via AfterConnect so every connection in the pool has them ready.
func New(ctx context.Context, connString string) (*PostgresRepository, error) {
	// Apply schema first using a direct connection, before creating the pool.
	// This ensures tables exist when AfterConnect prepares statements on pool connections.
	schemaConn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return nil, err
	}
	if _, err := schemaConn.Exec(ctx, schema); err != nil {
		schemaConn.Close(ctx)
		return nil, err
	}
	schemaConn.Close(ctx)

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	// Register prepared statements on every new connection.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		stmts := []struct {
			name string
			sql  string
		}{
			{
				"insert_conversation",
				"INSERT INTO conversations (id, partner_id, user_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)",
			},
			{
				"insert_message",
				"INSERT INTO messages (id, conversation_id, role, content, token_count, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
			},
			{
				"load_window",
				"SELECT id, conversation_id, role, content, token_count, created_at FROM messages WHERE conversation_id = $1 ORDER BY created_at DESC LIMIT $2",
			},
			{
				"list_conversations",
				"SELECT id, partner_id, user_id, created_at, updated_at FROM conversations WHERE partner_id = $1 AND user_id = $2 ORDER BY updated_at DESC",
			},
			{
				"update_conversation_updated_at",
				"UPDATE conversations SET updated_at = $1 WHERE id = $2",
			},
		}

		for _, s := range stmts {
			if _, err := conn.Prepare(ctx, s.name, s.sql); err != nil {
				return err
			}
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &PostgresRepository{pool: pool}, nil
}

// Close shuts down the connection pool.
func (r *PostgresRepository) Close() {
	r.pool.Close()
}

// CreateConversation creates a new conversation scoped to the given partner and user.
func (r *PostgresRepository) CreateConversation(ctx context.Context, partnerID, userID uuid.UUID) (domain.Conversation, error) {
	id := uuid.New()
	now := time.Now().UTC()

	_, err := r.pool.Exec(ctx, "insert_conversation", id, partnerID, userID, now, now)
	if err != nil {
		return domain.Conversation{}, err
	}

	return domain.Conversation{
		ID:        id,
		PartnerID: partnerID,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// AppendMessage appends a new message to the specified conversation and updates
// the conversation's UpdatedAt timestamp.
func (r *PostgresRepository) AppendMessage(ctx context.Context, conversationID uuid.UUID, role domain.Role, content string) (domain.Message, error) {
	id := uuid.New()
	now := time.Now().UTC()
	tc := len(content) / 4

	_, err := r.pool.Exec(ctx, "insert_message", id, conversationID, string(role), content, tc, now)
	if err != nil {
		return domain.Message{}, err
	}

	_, err = r.pool.Exec(ctx, "update_conversation_updated_at", now, conversationID)
	if err != nil {
		return domain.Message{}, err
	}

	return domain.Message{
		ID:             id,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		TokenCount:     tc,
		CreatedAt:      now,
	}, nil
}

// LoadWindow returns the last n messages from the specified conversation,
// ordered oldest-first. Queries DESC for efficiency, then reverses in-place.
func (r *PostgresRepository) LoadWindow(ctx context.Context, conversationID uuid.UUID, n int) ([]domain.Message, error) {
	rows, err := r.pool.Query(ctx, "load_window", conversationID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var msg domain.Message
		var roleStr string
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &roleStr, &msg.Content, &msg.TokenCount, &msg.CreatedAt); err != nil {
			return nil, err
		}
		msg.Role = domain.Role(roleStr)
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to produce oldest-first order (query returned DESC).
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}

// ListConversations returns all conversations for the given (partnerID, userID) pair,
// sorted by most recently updated first. Returns an empty slice (not nil) when none match.
func (r *PostgresRepository) ListConversations(ctx context.Context, partnerID, userID uuid.UUID) ([]domain.Conversation, error) {
	rows, err := r.pool.Query(ctx, "list_conversations", partnerID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	convs := []domain.Conversation{}
	for rows.Next() {
		var conv domain.Conversation
		if err := rows.Scan(&conv.ID, &conv.PartnerID, &conv.UserID, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, conv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return convs, nil
}
