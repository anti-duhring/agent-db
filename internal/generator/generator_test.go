package generator_test

import (
	"testing"

	"github.com/anti-duhring/agent-db/internal/domain"
	"github.com/anti-duhring/agent-db/internal/generator"
	"github.com/google/uuid"
)

var (
	testPartnerID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testUserID    = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

// TestDeterminism verifies that the same seed produces identical output.
func TestDeterminism(t *testing.T) {
	g1 := generator.New(42)
	g2 := generator.New(42)

	d1 := g1.Generate(testPartnerID, testUserID, generator.Small)
	d2 := g2.Generate(testPartnerID, testUserID, generator.Small)

	if len(d1.Conversations) != len(d2.Conversations) {
		t.Fatalf("different conversation counts: %d vs %d", len(d1.Conversations), len(d2.Conversations))
	}

	for i, c1 := range d1.Conversations {
		c2 := d2.Conversations[i]
		if c1.ID != c2.ID {
			t.Errorf("conversation[%d] ID mismatch: %v vs %v", i, c1.ID, c2.ID)
		}
		if c1.CreatedAt != c2.CreatedAt {
			t.Errorf("conversation[%d] CreatedAt mismatch: %v vs %v", i, c1.CreatedAt, c2.CreatedAt)
		}
		if c1.UpdatedAt != c2.UpdatedAt {
			t.Errorf("conversation[%d] UpdatedAt mismatch: %v vs %v", i, c1.UpdatedAt, c2.UpdatedAt)
		}

		msgs1 := d1.Messages[c1.ID]
		msgs2 := d2.Messages[c2.ID]
		if len(msgs1) != len(msgs2) {
			t.Fatalf("conversation[%d] message count mismatch: %d vs %d", i, len(msgs1), len(msgs2))
		}
		for j, m1 := range msgs1 {
			m2 := msgs2[j]
			if m1.ID != m2.ID {
				t.Errorf("conv[%d] msg[%d] ID mismatch: %v vs %v", i, j, m1.ID, m2.ID)
			}
			if m1.Content != m2.Content {
				t.Errorf("conv[%d] msg[%d] Content mismatch", i, j)
			}
			if m1.TokenCount != m2.TokenCount {
				t.Errorf("conv[%d] msg[%d] TokenCount mismatch: %d vs %d", i, j, m1.TokenCount, m2.TokenCount)
			}
			if m1.CreatedAt != m2.CreatedAt {
				t.Errorf("conv[%d] msg[%d] CreatedAt mismatch: %v vs %v", i, j, m1.CreatedAt, m2.CreatedAt)
			}
		}
	}
}

// TestDifferentSeeds verifies that different seeds produce different output.
func TestDifferentSeeds(t *testing.T) {
	d1 := generator.New(42).Generate(testPartnerID, testUserID, generator.Small)
	d2 := generator.New(99).Generate(testPartnerID, testUserID, generator.Small)

	// It would be astronomically unlikely for both seeds to produce the same first conversation ID.
	if d1.Conversations[0].ID == d2.Conversations[0].ID {
		t.Error("different seeds produced the same first conversation ID — seeding broken")
	}
}

// TestSmallProfile verifies Small profile counts.
func TestSmallProfile(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Small)

	if len(data.Conversations) != 5 {
		t.Errorf("Small profile: expected 5 conversations, got %d", len(data.Conversations))
	}
	for i, c := range data.Conversations {
		msgs := data.Messages[c.ID]
		if len(msgs) != 10 {
			t.Errorf("Small profile: conversation[%d] expected 10 messages, got %d", i, len(msgs))
		}
	}
}

// TestMediumProfile verifies Medium profile counts.
func TestMediumProfile(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Medium)

	if len(data.Conversations) != 10 {
		t.Errorf("Medium profile: expected 10 conversations, got %d", len(data.Conversations))
	}
	for i, c := range data.Conversations {
		msgs := data.Messages[c.ID]
		if len(msgs) != 500 {
			t.Errorf("Medium profile: conversation[%d] expected 500 messages, got %d", i, len(msgs))
		}
	}
}

// TestLargeProfile verifies Large profile counts.
func TestLargeProfile(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Large)

	if len(data.Conversations) != 10 {
		t.Errorf("Large profile: expected 10 conversations, got %d", len(data.Conversations))
	}
	for i, c := range data.Conversations {
		msgs := data.Messages[c.ID]
		if len(msgs) != 5000 {
			t.Errorf("Large profile: conversation[%d] expected 5000 messages, got %d", i, len(msgs))
		}
	}
}

// TestContentLength verifies all messages in Small profile have 100-2000 char content.
func TestContentLength(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Small)

	for _, c := range data.Conversations {
		for j, m := range data.Messages[c.ID] {
			if len(m.Content) < 100 {
				t.Errorf("message[%d] content too short: %d chars", j, len(m.Content))
			}
			if len(m.Content) > 2000 {
				t.Errorf("message[%d] content too long: %d chars", j, len(m.Content))
			}
		}
	}
}

// TestRoleAlternation verifies messages alternate user/assistant starting with user.
func TestRoleAlternation(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Small)

	for ci, c := range data.Conversations {
		msgs := data.Messages[c.ID]
		for i, m := range msgs {
			var expectedRole domain.Role
			if i%2 == 0 {
				expectedRole = domain.RoleUser
			} else {
				expectedRole = domain.RoleAssistant
			}
			if m.Role != expectedRole {
				t.Errorf("conv[%d] msg[%d]: expected role %q, got %q", ci, i, expectedRole, m.Role)
			}
		}
	}
}

// TestTokenCount verifies every message TokenCount == len(Content) / 4.
func TestTokenCount(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Small)

	for _, c := range data.Conversations {
		for j, m := range data.Messages[c.ID] {
			expected := len(m.Content) / 4
			if m.TokenCount != expected {
				t.Errorf("message[%d] TokenCount: expected %d (len/4), got %d", j, expected, m.TokenCount)
			}
		}
	}
}

// TestNonZeroUUIDs verifies all conversations and messages have non-zero UUIDs.
func TestNonZeroUUIDs(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Small)
	zeroUUID := uuid.UUID{}

	for i, c := range data.Conversations {
		if c.ID == zeroUUID {
			t.Errorf("conversation[%d] has zero UUID", i)
		}
		for j, m := range data.Messages[c.ID] {
			if m.ID == zeroUUID {
				t.Errorf("conv[%d] msg[%d] has zero UUID", i, j)
			}
		}
	}
}

// TestPartnerAndUserIDs verifies generated conversations carry the supplied partner/user IDs.
func TestPartnerAndUserIDs(t *testing.T) {
	data := generator.New(42).Generate(testPartnerID, testUserID, generator.Small)

	for i, c := range data.Conversations {
		if c.PartnerID != testPartnerID {
			t.Errorf("conversation[%d] PartnerID mismatch: %v vs %v", i, c.PartnerID, testPartnerID)
		}
		if c.UserID != testUserID {
			t.Errorf("conversation[%d] UserID mismatch: %v vs %v", i, c.UserID, testUserID)
		}
	}
}
