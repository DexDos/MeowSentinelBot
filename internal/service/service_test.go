package service

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/storage/sqlite"
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "service_test.db")

	cfg := &config.DB{
		Controller: "sqlite",
		DSN:        "%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON",
		Path:       dbPath,
	}

	store, err := sqlite.NewStorage(cfg)
	if err != nil {
		t.Fatalf("failed to init test sqlite storage: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	return NewService(store)
}

func TestService_StackPermissions(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	ownerID := int64(101)
	otherUserID := int64(102)

	_ = svc.EnsureUser(ctx, model.User{TelegramID: ownerID, Username: "owner"})
	_ = svc.EnsureUser(ctx, model.User{TelegramID: otherUserID, Username: "other"})

	// Create Stack
	st, err := svc.CreateStack(ctx, ownerID, "Owner's Watchlist")
	if err != nil {
		t.Fatalf("failed to create stack: %v", err)
	}

	// Other user cannot rename or delete
	if err := svc.UpdateStackTitle(ctx, otherUserID, st.ID, "Hacked Title"); err != ErrNotOwner {
		t.Fatalf("expected ErrNotOwner, got %v", err)
	}
	if err := svc.DeleteStack(ctx, otherUserID, st.ID); err != ErrNotOwner {
		t.Fatalf("expected ErrNotOwner, got %v", err)
	}

	// Owner can rename
	if err := svc.UpdateStackTitle(ctx, ownerID, st.ID, "Owner's Renamed Watchlist"); err != nil {
		t.Fatalf("failed to rename stack: %v", err)
	}
}

func TestService_InviteSharingFlow(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	ownerID := int64(201)
	friendID := int64(202)

	_ = svc.EnsureUser(ctx, model.User{TelegramID: ownerID, Username: "cat_owner"})
	_ = svc.EnsureUser(ctx, model.User{TelegramID: friendID, Username: "cat_friend"})

	st, err := svc.CreateStack(ctx, ownerID, "Meow Cinema")
	if err != nil {
		t.Fatalf("create stack failed: %v", err)
	}

	// Non-owner cannot generate invite
	if _, err := svc.CreateInvite(ctx, friendID, st.ID, 24*time.Hour); err != ErrNotOwner {
		t.Fatalf("expected ErrNotOwner when friend creates invite, got %v", err)
	}

	// Owner generates invite
	inv, err := svc.CreateInvite(ctx, ownerID, st.ID, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}
	if len(inv.Code) == 0 {
		t.Fatalf("expected non-empty invite code")
	}

	// Friend redeems invite
	joined, err := svc.RedeemInvite(ctx, friendID, inv.Code)
	if err != nil {
		t.Fatalf("failed to redeem invite: %v", err)
	}
	if joined.ID != st.ID {
		t.Fatalf("expected stack ID %d, got %d", st.ID, joined.ID)
	}

	// Friend can now view entries
	entries, err := svc.GetStackEntries(ctx, friendID, st.ID)
	if err != nil {
		t.Fatalf("failed to get stack entries as member: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries initially, got %d", len(entries))
	}

	// Friend adds entry
	entry, err := svc.AddEntry(ctx, friendID, st.ID, "Spirited Away", "Anime", 1, 1)
	if err != nil {
		t.Fatalf("failed to add entry: %v", err)
	}

	// Friend adjusts progress (+1 episode)
	updated, err := svc.AdjustEntryProgress(ctx, friendID, entry.ID, 0, 1)
	if err != nil || updated.Episode != 2 {
		t.Fatalf("expected episode 2, got %v (err: %v)", updated.Episode, err)
	}

	// Owner rates entry
	rated, err := svc.RateEntry(ctx, ownerID, entry.ID, model.MEOW)
	if err != nil || rated.Rating != model.MEOW {
		t.Fatalf("expected rating MEOW, got %v (err: %v)", rated.Rating, err)
	}

	// Friend leaves stack
	if err := svc.LeaveStack(ctx, friendID, st.ID); err != nil {
		t.Fatalf("failed to leave stack: %v", err)
	}

	// After leaving, friend can no longer access entries
	if _, err := svc.GetStackEntries(ctx, friendID, st.ID); err != ErrNotMember {
		t.Fatalf("expected ErrNotMember after leaving stack, got %v", err)
	}
}
