package fsm

import (
	"sync"
	"testing"
	"time"
)

func TestFSM_StateAndData(t *testing.T) {
	store := NewStore(0)
	defer store.Close()

	userID := int64(123)

	// Initially StateNone
	if st := store.GetState(userID); st != StateNone {
		t.Errorf("expected StateNone, got %v", st)
	}

	// Set state with data
	initialData := map[string]any{
		"stackID": int64(42),
		"title":   "Attack on Titan",
	}
	store.SetWithData(userID, StateWaitForEntryType, initialData, 5*time.Minute)

	st, data, ok := store.Get(userID)
	if !ok || st != StateWaitForEntryType {
		t.Fatalf("expected StateWaitForEntryType, got %v (ok=%v)", st, ok)
	}
	if data["title"] != "Attack on Titan" {
		t.Errorf("expected title Attack on Titan, got %v", data["title"])
	}

	// Update data
	store.SetData(userID, "mediaType", "Anime")
	val, exists := store.GetData(userID, "mediaType")
	if !exists || val != "Anime" {
		t.Fatalf("expected mediaType Anime, got %v", val)
	}

	// Clear
	store.Clear(userID)
	if st := store.GetState(userID); st != StateNone {
		t.Errorf("expected StateNone after clear, got %v", st)
	}
}

func TestFSM_Expiration(t *testing.T) {
	store := NewStore(10 * time.Millisecond)
	defer store.Close()

	userID := int64(456)
	store.Set(userID, StateWaitForStackTitle, 20*time.Millisecond)

	if st := store.GetState(userID); st != StateWaitForStackTitle {
		t.Fatalf("expected StateWaitForStackTitle immediately, got %v", st)
	}

	// Wait for expiration
	time.Sleep(50 * time.Millisecond)

	if st := store.GetState(userID); st != StateNone {
		t.Fatalf("expected StateNone after expiration, got %v", st)
	}
}

func TestFSM_Concurrency(t *testing.T) {
	store := NewStore(50 * time.Millisecond)
	defer store.Close()

	var wg sync.WaitGroup
	workers := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			uid := int64(id)
			store.Set(uid, StateWaitForStackTitle, time.Second)
			store.SetData(uid, "worker", id)
			_, _ = store.GetData(uid, "worker")
			_ = store.GetState(uid)
			store.Clear(uid)
		}(i)
	}

	wg.Wait()
}
