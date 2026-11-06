package handlers

import (
	"MeowSentinelBot/internal/service"
	"errors"
	"testing"
)

func TestParseProgress(t *testing.T) {
	tests := []struct {
		input       string
		wantSeason  int64
		wantEpisode int64
	}{
		{"2 5", 2, 5},
		{"s2 e5", 2, 5},
		{"S03E14", 3, 14},
		{"season 4 episode 10", 4, 10},
		{"1, 8", 1, 8},
		{"2-6", 2, 6},
		{"12", 1, 12}, // Single number defaults to season 1, episode 12
		{"invalid", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s, e := parseProgress(tt.input)
			if s != tt.wantSeason || e != tt.wantEpisode {
				t.Errorf("parseProgress(%q) = (%d, %d), want (%d, %d)", tt.input, s, e, tt.wantSeason, tt.wantEpisode)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	h := &BotHandler{}

	// Known domain errors should return friendly text
	if msg := h.sanitizeError(service.ErrEmptyTitle, "default"); msg != "Title cannot be empty." {
		t.Errorf("expected 'Title cannot be empty.', got %q", msg)
	}
	if msg := h.sanitizeError(service.ErrNotOwner, "default"); msg != "Only the stack owner can perform this action." {
		t.Errorf("expected 'Only the stack owner can perform this action.', got %q", msg)
	}

	// Raw SQL errors MUST NOT leak to user
	sqlErr := errors.New("link owner in user_stacks: SQL logic error: ON CONFLICT clause does not match any PRIMARY KEY or UNIQUE constraint (1)")
	if msg := h.sanitizeError(sqlErr, "Failed to create stack. Please try again."); msg != "Failed to create stack. Please try again." {
		t.Errorf("expected sanitized fallback, got %q", msg)
	}
}
