package fsm

import (
	"sync"
	"time"
)

type State int

const (
	StateNone State = iota
	StateWaitForStackTitle
	StateWaitForRenameTitle
	StateWaitForRedeemCode
	StateWaitForEntryTitle
	StateWaitForEntryType
	StateWaitForEditTitle
	StateWaitForEditType
	StateWaitForEditProgress
	StateWaitForSelectEntry
)

func (s State) String() string {
	switch s {
	case StateNone:
		return "StateNone"
	case StateWaitForStackTitle:
		return "StateWaitForStackTitle"
	case StateWaitForRenameTitle:
		return "StateWaitForRenameTitle"
	case StateWaitForRedeemCode:
		return "StateWaitForRedeemCode"
	case StateWaitForEntryTitle:
		return "StateWaitForEntryTitle"
	case StateWaitForEntryType:
		return "StateWaitForEntryType"
	case StateWaitForEditTitle:
		return "StateWaitForEditTitle"
	case StateWaitForEditType:
		return "StateWaitForEditType"
	case StateWaitForEditProgress:
		return "StateWaitForEditProgress"
	case StateWaitForSelectEntry:
		return "StateWaitForSelectEntry"
	default:
		return "StateUnknown"
	}
}

type Session struct {
	State     State
	Data      map[string]any
	ExpiresAt time.Time
}

type Store struct {
	mu       sync.RWMutex
	sessions map[int64]Session
	stopChan chan struct{}
}

func NewStore(cleanupInterval time.Duration) *Store {
	s := &Store{
		sessions: make(map[int64]Session),
		stopChan: make(chan struct{}),
	}
	if cleanupInterval > 0 {
		go s.startCleaner(cleanupInterval)
	}
	return s
}

func (s *Store) Close() {
	select {
	case <-s.stopChan:
		// already closed
	default:
		close(s.stopChan)
	}
}

func (s *Store) startCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for userID, sess := range s.sessions {
				if now.After(sess.ExpiresAt) {
					delete(s.sessions, userID)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *Store) Set(userID int64, state State, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[userID] = Session{
		State:     state,
		Data:      make(map[string]any),
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (s *Store) SetWithData(userID int64, state State, data map[string]any, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := make(map[string]any, len(data))
	for k, v := range data {
		cloned[k] = v
	}

	s.sessions[userID] = Session{
		State:     state,
		Data:      cloned,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (s *Store) Get(userID int64) (State, map[string]any, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[userID]
	s.mu.RUnlock()

	if !ok {
		return StateNone, nil, false
	}

	if time.Now().After(sess.ExpiresAt) {
		s.Clear(userID)
		return StateNone, nil, false
	}

	cloned := make(map[string]any, len(sess.Data))
	for k, v := range sess.Data {
		cloned[k] = v
	}

	return sess.State, cloned, true
}

func (s *Store) GetState(userID int64) State {
	st, _, _ := s.Get(userID)
	return st
}

func (s *Store) SetData(userID int64, key string, val any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[userID]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return
	}
	if sess.Data == nil {
		sess.Data = make(map[string]any)
	}
	sess.Data[key] = val
	s.sessions[userID] = sess
}

func (s *Store) GetData(userID int64, key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[userID]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil, false
	}
	val, exists := sess.Data[key]
	return val, exists
}

func (s *Store) Clear(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, userID)
}
