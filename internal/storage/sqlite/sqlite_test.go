package sqlite

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/model"
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := &config.DB{
		Controller: "sqlite",
		DSN:        "%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON",
		Path:       dbPath,
	}

	s, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	t.Cleanup(func() {
		_ = s.Close()
	})

	return s
}

func TestStorage_UsersAndStacks(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// 1. Ensure User
	user := model.User{
		TelegramID: 123456,
		Username:   "meow_user",
		FirstName:  "Meow",
	}
	if err := s.EnsureUser(ctx, user); err != nil {
		t.Fatalf("failed to ensure user: %v", err)
	}

	u, err := s.GetUser(ctx, user.TelegramID)
	if err != nil || u == nil {
		t.Fatalf("failed to get user: %v, got %v", err, u)
	}
	if u.Username != "meow_user" {
		t.Errorf("expected username meow_user, got %s", u.Username)
	}

	// 2. Create Stack
	stackID, err := s.CreateStack(ctx, user.TelegramID, "Sci-Fi Watchlist")
	if err != nil {
		t.Fatalf("failed to create stack: %v", err)
	}
	if stackID <= 0 {
		t.Fatalf("expected positive stackID, got %d", stackID)
	}

	// Verify selected stack was automatically set
	selStackID, err := s.GetSelectedStackID(ctx, user.TelegramID)
	if err != nil || selStackID != stackID {
		t.Fatalf("expected selected stack %d, got %d (err: %v)", stackID, selStackID, err)
	}

	// Verify user stacks list
	stacks, err := s.GetUserStacks(ctx, user.TelegramID)
	if err != nil {
		t.Fatalf("failed to get user stacks: %v", err)
	}
	if len(stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(stacks))
	}
	if stacks[0].Role != model.RoleOwner || stacks[0].Title != "Sci-Fi Watchlist" {
		t.Errorf("unexpected stack info: %+v", stacks[0])
	}

	// Update Title
	if err := s.UpdateStackTitle(ctx, stackID, "Sci-Fi & Cyberpunk"); err != nil {
		t.Fatalf("failed to update stack title: %v", err)
	}
	st, err := s.GetStackByID(ctx, stackID)
	if err != nil || st.Title != "Sci-Fi & Cyberpunk" {
		t.Fatalf("expected updated title, got %v", st)
	}
}

func TestStorage_InvitesAndSharing(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	owner := model.User{TelegramID: 1001, Username: "owner", FirstName: "Owner"}
	friend := model.User{TelegramID: 1002, Username: "friend", FirstName: "Friend"}
	_ = s.EnsureUser(ctx, owner)
	_ = s.EnsureUser(ctx, friend)

	stackID, err := s.CreateStack(ctx, owner.TelegramID, "Shared Anime")
	if err != nil {
		t.Fatalf("failed to create stack: %v", err)
	}

	// Create Invite
	invite := model.Invite{
		Code:      "MEOW-9999",
		StackID:   stackID,
		CreatedBy: owner.TelegramID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := s.CreateInvite(ctx, invite); err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}

	// Friend redeems invite
	joinedStack, err := s.ConsumeInvite(ctx, "MEOW-9999", friend.TelegramID)
	if err != nil {
		t.Fatalf("failed to consume invite: %v", err)
	}
	if joinedStack.ID != stackID {
		t.Errorf("expected joined stack ID %d, got %d", stackID, joinedStack.ID)
	}

	// Check member status
	isMember, role, err := s.IsStackMember(ctx, friend.TelegramID, stackID)
	if err != nil || !isMember || role != model.RoleMember {
		t.Fatalf("expected member role, got member=%v, role=%s, err=%v", isMember, role, err)
	}

	// Reusing same invite should fail
	_, err = s.ConsumeInvite(ctx, "MEOW-9999", friend.TelegramID)
	if err == nil {
		t.Fatalf("expected error on reusing consumed invite, got nil")
	}

	// Friend leaves stack
	if err := s.LeaveStack(ctx, friend.TelegramID, stackID); err != nil {
		t.Fatalf("failed to leave stack: %v", err)
	}
	isMember, _, _ = s.IsStackMember(ctx, friend.TelegramID, stackID)
	if isMember {
		t.Errorf("expected friend to no longer be a member")
	}
}

func TestStorage_EntriesAndRatings(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	user := model.User{TelegramID: 555, Username: "cinephile", FirstName: "Alex"}
	_ = s.EnsureUser(ctx, user)

	stackID, _ := s.CreateStack(ctx, user.TelegramID, "Cinema Classics")

	// Add Entry
	entry := model.Entry{
		StackID: stackID,
		Title:   "Interstellar",
		Type:    "Movie",
		Season:  1,
		Episode: 1,
	}
	entryID, err := s.AddEntry(ctx, entry)
	if err != nil {
		t.Fatalf("failed to add entry: %v", err)
	}

	// Get Entry
	e, err := s.GetEntryByID(ctx, entryID)
	if err != nil || e == nil {
		t.Fatalf("failed to get entry: %v", err)
	}
	if e.Title != "Interstellar" || e.Rating != model.NotSeen {
		t.Errorf("unexpected entry data: %+v", e)
	}

	// Update Progress
	if err := s.UpdateEntryProgress(ctx, entryID, 1, 2); err != nil {
		t.Fatalf("failed to update progress: %v", err)
	}

	// Rate Entry
	if err := s.RateEntry(ctx, user.TelegramID, entryID, model.MEOW); err != nil {
		t.Fatalf("failed to rate entry: %v", err)
	}

	// Verify ratings aggregation
	e, _ = s.GetEntryByID(ctx, entryID)
	if e.Rating != model.MEOW || e.RatingCount != 1 {
		t.Errorf("expected MEOW rating with count 1, got %v with count %d", e.Rating, e.RatingCount)
	}

	// Verify stack entries
	entries, err := s.GetStackEntries(ctx, stackID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (err: %v)", len(entries), err)
	}

	// Delete Entry
	if err := s.DeleteEntry(ctx, entryID); err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}
	e, _ = s.GetEntryByID(ctx, entryID)
	if e != nil {
		t.Fatalf("expected entry to be nil after deletion")
	}
}
