package handlers

import (
	"MeowSentinelBot/internal/model"
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

func TestParseLocalIndex(t *testing.T) {
	tests := []struct {
		input     string
		wantIndex int
		wantErr   bool
	}{
		{"1", 1, false},
		{"#1", 1, false},
		{"# 3", 3, false},
		{" 42 ", 42, false},
		{"#99", 99, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"#-5", 0, true},
		{"abc", 0, true},
		{"", 0, true},
		{"   ", 0, true},
		{"#", 0, true},
		{"1.5", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLocalIndex(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLocalIndex(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.wantIndex {
				t.Errorf("ParseLocalIndex(%q) = %d, want %d", tt.input, got, tt.wantIndex)
			}
		})
	}
}

func TestResolveEntryByLocalIndex_AndReindexing(t *testing.T) {
	// Simulate a stack with 3 items having non-sequential DB IDs: 15, 88, 302
	entries := []model.Entry{
		{ID: 15, Title: "The Godfather"},
		{ID: 88, Title: "Pulp Fiction"},
		{ID: 302, Title: "The Dark Knight"},
	}

	// 1. Verify 1-based indexing maps to the correct DB IDs
	e1, err := ResolveEntryByLocalIndex(entries, 1)
	if err != nil || e1.ID != 15 {
		t.Fatalf("local index 1 failed: got ID %v, err %v", e1, err)
	}

	e2, err := ResolveEntryByLocalIndex(entries, 2)
	if err != nil || e2.ID != 88 {
		t.Fatalf("local index 2 failed: got ID %v, err %v", e2, err)
	}

	e3, err := ResolveEntryByLocalIndex(entries, 3)
	if err != nil || e3.ID != 302 {
		t.Fatalf("local index 3 failed: got ID %v, err %v", e3, err)
	}

	// 2. Strict Out-of-Bounds validation
	if _, err := ResolveEntryByLocalIndex(entries, 0); err == nil {
		t.Errorf("expected error for index 0, got nil")
	}
	if _, err := ResolveEntryByLocalIndex(entries, -1); err == nil {
		t.Errorf("expected error for index -1, got nil")
	}
	if _, err := ResolveEntryByLocalIndex(entries, 4); err == nil {
		t.Errorf("expected error for index 4, got nil")
	}

	// 3. Deletion from the middle (delete item with DB ID 88 at local index 2)
	// Remaining entries are re-queried / re-sliced
	remainingEntries := []model.Entry{
		entries[0], // ID 15
		entries[2], // ID 302
	}

	// After deletion, remaining items must re-index cleanly as 1, 2...
	re1, err := ResolveEntryByLocalIndex(remainingEntries, 1)
	if err != nil || re1.ID != 15 {
		t.Fatalf("after deletion, local index 1 should map to ID 15, got %v (err=%v)", re1, err)
	}

	re2, err := ResolveEntryByLocalIndex(remainingEntries, 2)
	if err != nil || re2.ID != 302 {
		t.Fatalf("after deletion, local index 2 should map to ID 302, got %v (err=%v)", re2, err)
	}

	// Local index 3 should now be out of bounds!
	if _, err := ResolveEntryByLocalIndex(remainingEntries, 3); err == nil {
		t.Errorf("after deletion, local index 3 should be out of bounds, got nil error")
	}

	// 4. Empty stack handling
	emptyEntries := []model.Entry{}
	if _, err := ResolveEntryByLocalIndex(emptyEntries, 1); !errors.Is(err, ErrStackEmpty) {
		t.Errorf("expected ErrStackEmpty for empty slice, got %v", err)
	}
}
