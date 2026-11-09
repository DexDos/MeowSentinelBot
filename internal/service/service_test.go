package service

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/storage/sqlite"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
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

func TestService_IDOR_Protection(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	userA := int64(1001)
	userB := int64(2002)

	_ = svc.EnsureUser(ctx, model.User{TelegramID: userA, Username: "alice"})
	_ = svc.EnsureUser(ctx, model.User{TelegramID: userB, Username: "mallory"})

	// User A creates private stack and adds an entry
	stackA, err := svc.CreateStack(ctx, userA, "Alice Private Movies")
	if err != nil {
		t.Fatalf("create stack failed: %v", err)
	}

	entryA, err := svc.AddEntry(ctx, userA, stackA.ID, "Top Secret Movie", "Movie", 1, 1)
	if err != nil {
		t.Fatalf("add entry failed: %v", err)
	}

	// Verify User B cannot access User A's stack
	if _, err := svc.GetStack(ctx, userB, stackA.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not access stack A, got %v", err)
	}
	if _, err := svc.GetStackEntries(ctx, userB, stackA.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not read entries in stack A, got %v", err)
	}
	if err := svc.UpdateStackTitle(ctx, userB, stackA.ID, "Mallory Pwned This"); !errors.Is(err, ErrNotOwner) {
		t.Errorf("IDOR: user B should not rename stack A, got %v", err)
	}
	if err := svc.DeleteStack(ctx, userB, stackA.ID); !errors.Is(err, ErrNotOwner) {
		t.Errorf("IDOR: user B should not delete stack A, got %v", err)
	}

	// Verify User B cannot access, modify, rate, or delete User A's entry
	if _, err := svc.GetEntry(ctx, userB, entryA.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not view entry A, got %v", err)
	}
	if _, err := svc.UpdateEntryTitle(ctx, userB, entryA.ID, "Hacked Title"); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not edit title of entry A, got %v", err)
	}
	if _, err := svc.UpdateEntryType(ctx, userB, entryA.ID, "Anime"); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not edit type of entry A, got %v", err)
	}
	if _, err := svc.AdjustEntryProgress(ctx, userB, entryA.ID, 1, 1); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not step entry A, got %v", err)
	}
	if _, err := svc.SetEntryProgress(ctx, userB, entryA.ID, 2, 5); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not set progress on entry A, got %v", err)
	}
	if err := svc.DeleteEntry(ctx, userB, entryA.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not delete entry A, got %v", err)
	}
	if _, err := svc.RateEntry(ctx, userB, entryA.ID, model.MEOW); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not rate entry A, got %v", err)
	}
	if _, err := svc.GetUserRating(ctx, userB, entryA.ID); !errors.Is(err, ErrNotMember) {
		t.Errorf("IDOR: user B should not read rating for entry A, got %v", err)
	}
}

func TestService_MaliciousPayloads(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	user := int64(3001)
	_ = svc.EnsureUser(ctx, model.User{TelegramID: user, Username: "tester"})

	// 1. SQL Injection vectors in titles
	sqliPayloads := []string{
		"' OR 1=1 --",
		"'; DROP TABLE stacks; --",
		"1' UNION SELECT * FROM users --",
		"admin'--",
		"\" OR \"\"=\"",
	}

	for _, payload := range sqliPayloads {
		st, err := svc.CreateStack(ctx, user, payload)
		if err != nil {
			t.Fatalf("SQLi payload %q caused create stack error: %v", payload, err)
		}

		// Verify title is safely stored literally without executing SQL
		if st.Title != payload {
			t.Errorf("expected stack title %q, got %q", payload, st.Title)
		}

		// Add entry with SQLi title
		e, err := svc.AddEntry(ctx, user, st.ID, payload, "Movie", 1, 1)
		if err != nil {
			t.Fatalf("SQLi payload %q caused add entry error: %v", payload, err)
		}
		if e.Title != payload {
			t.Errorf("expected entry title %q, got %q", payload, e.Title)
		}
	}

	// 2. Empty and whitespace-only inputs
	if _, err := svc.CreateStack(ctx, user, ""); !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("expected ErrEmptyTitle for empty string, got %v", err)
	}
	if _, err := svc.CreateStack(ctx, user, "   \t\n  "); !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("expected ErrEmptyTitle for whitespace, got %v", err)
	}

	st, _ := svc.CreateStack(ctx, user, "Normal Stack")
	if _, err := svc.AddEntry(ctx, user, st.ID, "   ", "Movie", 1, 1); !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("expected ErrEmptyTitle for whitespace entry, got %v", err)
	}
	if err := svc.UpdateStackTitle(ctx, user, st.ID, "   "); !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("expected ErrEmptyTitle for whitespace rename, got %v", err)
	}

	// 3. Oversized string inputs
	longTitle := strings.Repeat("A", 129)
	if _, err := svc.CreateStack(ctx, user, longTitle); !errors.Is(err, ErrTitleTooLong) {
		t.Errorf("expected ErrTitleTooLong for 129-char stack title, got %v", err)
	}
	if _, err := svc.AddEntry(ctx, user, st.ID, longTitle, "Movie", 1, 1); !errors.Is(err, ErrTitleTooLong) {
		t.Errorf("expected ErrTitleTooLong for 129-char entry title, got %v", err)
	}
	if err := svc.UpdateStackTitle(ctx, user, st.ID, longTitle); !errors.Is(err, ErrTitleTooLong) {
		t.Errorf("expected ErrTitleTooLong for 129-char stack rename, got %v", err)
	}
}

func TestService_MiddleDeletionReindexing(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	user := int64(4001)
	_ = svc.EnsureUser(ctx, model.User{TelegramID: user, Username: "reindexer"})

	st, err := svc.CreateStack(ctx, user, "Queue")
	if err != nil {
		t.Fatalf("failed to create stack: %v", err)
	}

	// Add 3 entries
	e1, _ := svc.AddEntry(ctx, user, st.ID, "Item 1", "Movie", 1, 1)
	e2, _ := svc.AddEntry(ctx, user, st.ID, "Item 2", "Movie", 1, 1)
	e3, _ := svc.AddEntry(ctx, user, st.ID, "Item 3", "Movie", 1, 1)

	// Fetch entries before deletion
	entriesBefore, err := svc.GetStackEntries(ctx, user, st.ID)
	if err != nil || len(entriesBefore) != 3 {
		t.Fatalf("expected 3 entries, got %d (err: %v)", len(entriesBefore), err)
	}

	// Delete the middle entry (e2)
	if err := svc.DeleteEntry(ctx, user, e2.ID); err != nil {
		t.Fatalf("failed to delete middle entry: %v", err)
	}

	// Fetch entries after deletion
	entriesAfter, err := svc.GetStackEntries(ctx, user, st.ID)
	if err != nil || len(entriesAfter) != 2 {
		t.Fatalf("expected 2 entries remaining, got %d (err: %v)", len(entriesAfter), err)
	}

	// Verify middle entry is gone and other entries remain
	foundE2 := false
	for _, e := range entriesAfter {
		if e.ID == e2.ID {
			foundE2 = true
		}
	}
	if foundE2 {
		t.Fatalf("deleted entry %d still found in entries after deletion", e2.ID)
	}

	// Verify remaining entries IDs match e1 and e3
	hasE1, hasE3 := false, false
	for _, e := range entriesAfter {
		if e.ID == e1.ID {
			hasE1 = true
		}
		if e.ID == e3.ID {
			hasE3 = true
		}
	}
	if !hasE1 || !hasE3 {
		t.Fatalf("expected e1 (%d) and e3 (%d) to remain, got %+v", e1.ID, e3.ID, entriesAfter)
	}
}

func TestService_ConcurrencySafety(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	const numWorkers = 20
	const opsPerWorker = 10
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			userID := int64(5000 + workerID)
			_ = svc.EnsureUser(ctx, model.User{TelegramID: userID, Username: fmt.Sprintf("worker_%d", workerID)})

			for j := 0; j < opsPerWorker; j++ {
				st, err := svc.CreateStack(ctx, userID, fmt.Sprintf("Stack %d-%d", workerID, j))
				if err != nil {
					t.Errorf("worker %d failed to create stack %d: %v", workerID, j, err)
					return
				}

				e, err := svc.AddEntry(ctx, userID, st.ID, fmt.Sprintf("Movie %d-%d", workerID, j), "Movie", 1, 1)
				if err != nil {
					t.Errorf("worker %d failed to add entry %d: %v", workerID, j, err)
					return
				}

				_, _ = svc.AdjustEntryProgress(ctx, userID, e.ID, 0, 1)
				_, _ = svc.RateEntry(ctx, userID, e.ID, model.ScoreFromInt(int64((j%6)+1)))
				_, _ = svc.GetStackEntries(ctx, userID, st.ID)
				_, _ = svc.GetUserStacks(ctx, userID)
			}
		}(i)
	}

	wg.Wait()
}
