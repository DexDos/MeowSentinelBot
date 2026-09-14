package service

import (
	"sync"
	"time"
)

// State is an FSM type
type State int8

const (
	StateNone State = iota
	StateWaitForNewStackName
	StateWaitForExistingStackName
	StateWaitForNewEntryTitle
	StateWaitForNewEntryType
	StateWaitForSeasonReplenishment
	StateWaitForEpisodeReplenishment
	StateWaitForConfirmation
)

// UserState is an FSM type. Check for State validity by verifying Time with current.
// If Time is less than current it means State is expired
type UserState struct {
	ExpiresAt time.Time
	State     State
}

type CurrentStack struct {
	ExpiresAt time.Time
	ID        int64
}
type CurrentEntry struct {
	ExpiresAt time.Time
	ID        int64
}

// InMemFSMStore is a thread secure FSM for both user state (pending or so) and current stack selection
type InMemFSMStore struct {
	mu       sync.RWMutex
	sessions map[int64]UserState
	stacks   map[int64]CurrentStack
	entries  map[int64]CurrentEntry
}

func NewInMemFSMStore() *InMemFSMStore {
	store := &InMemFSMStore{
		sessions: make(map[int64]UserState),
		stacks:   make(map[int64]CurrentStack),
		entries:  make(map[int64]CurrentEntry),
	}
	go store.startCleaner(10 * time.Minute)
	return store
}

func (s *InMemFSMStore) CleanState(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, userID)
}
func (s *InMemFSMStore) CleanStack(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.stacks, userID)
}
func (s *InMemFSMStore) CleanEntry(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, userID)
}
func (s *InMemFSMStore) startCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for userID, state := range s.sessions {
			if now.After(state.ExpiresAt) {
				delete(s.sessions, userID)
			}
		}
		for userID, stack := range s.stacks {
			if now.After(stack.ExpiresAt) {
				delete(s.sessions, userID)
			}
		}
		for userID, entry := range s.entries {
			if now.After(entry.ExpiresAt) {
				delete(s.sessions, userID)
			}
		}
		s.mu.Unlock()
	}
}

func (s *InMemFSMStore) GetState(userID int64) State {
	s.mu.RLock()
	session, ok := s.sessions[userID]
	s.mu.RUnlock()
	if !ok {
		return StateNone
	}
	if time.Now().After(session.ExpiresAt) {
		s.CleanState(userID)
		return StateNone
	}
	return session.State
}
func (s *InMemFSMStore) GetStack(userID int64) (int64, bool) {
	s.mu.RLock()
	stack, ok := s.stacks[userID]
	s.mu.RUnlock()
	if !ok {
		return 0, false
	}
	return stack.ID, true
}
func (s *InMemFSMStore) GetEntry(userID int64) (int64, bool) {
	s.mu.RLock()
	entry, ok := s.entries[userID]
	s.mu.RUnlock()
	if !ok {
		return 0, false
	}
	return entry.ID, true
}
func (s *InMemFSMStore) SetState(userID int64, state State, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[userID] = UserState{
		ExpiresAt: time.Now().Add(ttl),
		State:     state,
	}
}
func (s *InMemFSMStore) SetStack(userID int64, stackID int64, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stacks[userID] = CurrentStack{
		ExpiresAt: time.Now().Add(ttl),
		ID:        stackID,
	}
}
func (s *InMemFSMStore) SetEntry(userID int64, entryID int64, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[userID] = CurrentEntry{
		ExpiresAt: time.Now().Add(ttl),
		ID:        entryID,
	}
}
