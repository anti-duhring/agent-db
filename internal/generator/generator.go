// Package generator produces deterministic synthetic chat conversation data
// for benchmarking. The same seed always produces identical output, enabling
// reproducible benchmark runs across invocations.
package generator

import (
	"time"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
)

// Profile defines the shape of a generated dataset.
type Profile struct {
	Name          string
	Conversations int
	Messages      int // per conversation
}

// Predefined benchmark profiles matching D-11.
var (
	Small  = Profile{Name: "small", Conversations: 5, Messages: 10}
	Medium = Profile{Name: "medium", Conversations: 10, Messages: 500}
	Large  = Profile{Name: "large", Conversations: 10, Messages: 5000}
)

// GeneratedData holds the result of a Generate call.
type GeneratedData struct {
	Conversations []domain.Conversation
	Messages      map[uuid.UUID][]domain.Message // keyed by conversation ID
}

// Generator produces deterministic synthetic data from a fixed seed.
// NEVER use global gofakeit functions — always use the seeded instance (g.faker).
type Generator struct {
	faker *gofakeit.Faker
}

// New creates a Generator seeded with the given value.
// The same seed always produces identical output (DATA-01).
func New(seed int64) *Generator {
	return &Generator{faker: gofakeit.New(uint64(seed))}
}

// Generate produces a complete dataset for the given profile.
// The partnerID and userID are stamped on every generated Conversation.
func (g *Generator) Generate(partnerID, userID uuid.UUID, profile Profile) GeneratedData {
	// Deterministic base time — all timestamps are offsets from this point.
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	data := GeneratedData{
		Conversations: make([]domain.Conversation, 0, profile.Conversations),
		Messages:      make(map[uuid.UUID][]domain.Message, profile.Conversations),
	}

	for ci := 0; ci < profile.Conversations; ci++ {
		convID := g.newUUID()
		convCreatedAt := baseTime.Add(time.Duration(ci) * time.Hour)

		conv := domain.Conversation{
			ID:        convID,
			PartnerID: partnerID,
			UserID:    userID,
			CreatedAt: convCreatedAt,
			UpdatedAt: convCreatedAt,
		}

		msgs := make([]domain.Message, 0, profile.Messages)
		for mi := 0; mi < profile.Messages; mi++ {
			msgID := g.newUUID()

			var role domain.Role
			if mi%2 == 0 {
				role = domain.RoleUser
			} else {
				role = domain.RoleAssistant
			}

			content := g.generateContent()
			tc := tokenCount(content)
			msgCreatedAt := convCreatedAt.Add(time.Duration(mi) * time.Minute)

			msgs = append(msgs, domain.Message{
				ID:             msgID,
				ConversationID: convID,
				Role:           role,
				Content:        content,
				TokenCount:     tc,
				CreatedAt:      msgCreatedAt,
			})

			// Keep UpdatedAt at the latest message time.
			conv.UpdatedAt = msgCreatedAt
		}

		data.Conversations = append(data.Conversations, conv)
		data.Messages[convID] = msgs
	}

	return data
}

// newUUID generates a UUID deterministically from the seeded faker.
// gofakeit v7 uses math/rand/v2.Source which does not implement io.Reader,
// so we generate 16 random bytes via the seeded Uint8() calls instead.
func (g *Generator) newUUID() uuid.UUID {
	var b [16]byte
	for i := range b {
		b[i] = g.faker.Uint8()
	}
	// Set UUID version 4 and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	id, _ := uuid.FromBytes(b[:])
	return id
}

// generateContent returns a chat-like string between 100 and 2000 characters.
// Uses the seeded faker instance to guarantee determinism (DATA-01, DATA-03).
// CRITICAL: NEVER call package-level gofakeit.Paragraph / gofakeit.Sentence here.
func (g *Generator) generateContent() string {
	// Paragraph(paragraphCount, sentenceCount, wordCount, separator)
	content := g.faker.Paragraph(1, 3, 10, " ")

	// Pad to minimum 100 characters.
	for len(content) < 100 {
		content += " " + g.faker.Sentence(10)
	}

	// Truncate to maximum 2000 characters.
	if len(content) > 2000 {
		content = content[:2000]
	}

	return content
}

// tokenCount estimates token count from content length (D-12).
func tokenCount(content string) int {
	return len(content) / 4
}
