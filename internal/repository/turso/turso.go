package turso

import (
	"context"
	"database/sql"
	_ "embed"
	"strings"
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/repository"
	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

//go:embed migrations/001_create_tables.sql
var schema string

// Compile-time interface check: TursoRepository must implement ChatRepository.
var _ repository.ChatRepository = (*TursoRepository)(nil)

// TursoRepository implements ChatRepository using the libsql remote driver with database/sql.
type TursoRepository struct {
	db *sql.DB
}

// New creates a new TursoRepository, connects to Turso Cloud, and applies the schema.
func New(ctx context.Context, url string, authToken string) (*TursoRepository, error) {
	connStr := url + "?authToken=" + authToken
	db, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// Apply schema. Try as a single exec first; fall back to statement-by-statement
	// because some libsql versions don't support multi-statement exec.
	if _, err := db.ExecContext(ctx, schema); err != nil {
		// Split by semicolon and execute each statement individually.
		stmts := strings.Split(schema, ";")
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				db.Close()
				return nil, err
			}
		}
	}

	return &TursoRepository{db: db}, nil
}

// Close closes the underlying database connection.
func (r *TursoRepository) Close() error {
	return r.db.Close()
}

// CreateConversation creates a new conversation scoped to the given partner and user.
func (r *TursoRepository) CreateConversation(ctx context.Context, partnerID, userID uuid.UUID) (domain.Conversation, error) {
	id := uuid.New()
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO conversations (id, partner_id, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		id.String(), partnerID.String(), userID.String(), nowStr, nowStr,
	)
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
func (r *TursoRepository) AppendMessage(ctx context.Context, conversationID uuid.UUID, role domain.Role, content string) (domain.Message, error) {
	id := uuid.New()
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	tc := len(content) / 4

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO messages (id, conversation_id, role, content, token_count, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id.String(), conversationID.String(), string(role), content, tc, nowStr,
	)
	if err != nil {
		return domain.Message{}, err
	}

	_, err = r.db.ExecContext(ctx,
		"UPDATE conversations SET updated_at = ? WHERE id = ?",
		nowStr, conversationID.String(),
	)
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
func (r *TursoRepository) LoadWindow(ctx context.Context, conversationID uuid.UUID, n int) ([]domain.Message, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, conversation_id, role, content, token_count, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at DESC LIMIT ?",
		conversationID.String(), n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var idStr, convIDStr, roleStr, content, createdAtStr string
		var tokenCount int
		if err := rows.Scan(&idStr, &convIDStr, &roleStr, &content, &tokenCount, &createdAtStr); err != nil {
			return nil, err
		}

		msgID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, err
		}
		convID, err := uuid.Parse(convIDStr)
		if err != nil {
			return nil, err
		}
		createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			return nil, err
		}

		msgs = append(msgs, domain.Message{
			ID:             msgID,
			ConversationID: convID,
			Role:           domain.Role(roleStr),
			Content:        content,
			TokenCount:     tokenCount,
			CreatedAt:      createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to produce oldest-first order (query returned DESC).
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	if msgs == nil {
		return []domain.Message{}, nil
	}
	return msgs, nil
}

// ListConversations returns all conversations for the given (partnerID, userID) pair,
// sorted by most recently updated first. Returns an empty slice (not nil) when none match.
func (r *TursoRepository) ListConversations(ctx context.Context, partnerID, userID uuid.UUID) ([]domain.Conversation, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, partner_id, user_id, created_at, updated_at FROM conversations WHERE partner_id = ? AND user_id = ? ORDER BY updated_at DESC",
		partnerID.String(), userID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	convs := []domain.Conversation{}
	for rows.Next() {
		var idStr, partnerIDStr, userIDStr, createdAtStr, updatedAtStr string
		if err := rows.Scan(&idStr, &partnerIDStr, &userIDStr, &createdAtStr, &updatedAtStr); err != nil {
			return nil, err
		}

		convID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, err
		}
		pID, err := uuid.Parse(partnerIDStr)
		if err != nil {
			return nil, err
		}
		uID, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, err
		}
		createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			return nil, err
		}
		updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtStr)
		if err != nil {
			return nil, err
		}

		convs = append(convs, domain.Conversation{
			ID:        convID,
			PartnerID: pID,
			UserID:    uID,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return convs, nil
}
