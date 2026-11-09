package service

import (
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/storage"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotOwner       = errors.New("only the stack owner can perform this action")
	ErrNotMember      = errors.New("you are not a member of this stack")
	ErrStackNotFound  = errors.New("stack not found")
	ErrEntryNotFound  = errors.New("entry not found")
	ErrInvalidCode    = errors.New("invalid or expired invite code")
	ErrAlreadyMember  = errors.New("you are already a member of this stack")
	ErrEmptyTitle     = errors.New("title cannot be empty")
	ErrTitleTooLong   = errors.New("title is too long (maximum 128 characters)")
)

type AppService interface {
	// User
	EnsureUser(ctx context.Context, u model.User) error
	GetUser(ctx context.Context, userID int64) (*model.User, error)

	// Stack
	CreateStack(ctx context.Context, ownerID int64, title string) (*model.StackInfo, error)
	GetUserStacks(ctx context.Context, userID int64) ([]model.StackInfo, error)
	GetStack(ctx context.Context, userID, stackID int64) (*model.StackInfo, error)
	UpdateStackTitle(ctx context.Context, userID, stackID int64, title string) error
	DeleteStack(ctx context.Context, userID, stackID int64) error
	LeaveStack(ctx context.Context, userID, stackID int64) error
	GetSelectedStackID(ctx context.Context, userID int64) (int64, error)
	SelectStack(ctx context.Context, userID, stackID int64) error

	// Invite
	CreateInvite(ctx context.Context, userID, stackID int64, ttl time.Duration) (*model.Invite, error)
	RedeemInvite(ctx context.Context, userID int64, code string) (*model.StackInfo, error)

	// Entry
	AddEntry(ctx context.Context, userID, stackID int64, title, mediaType string, season, episode int64) (*model.Entry, error)
	GetEntry(ctx context.Context, userID, entryID int64) (*model.Entry, error)
	GetStackEntries(ctx context.Context, userID, stackID int64) ([]model.Entry, error)
	UpdateEntryTitle(ctx context.Context, userID, entryID int64, title string) (*model.Entry, error)
	UpdateEntryType(ctx context.Context, userID, entryID int64, mediaType string) (*model.Entry, error)
	AdjustEntryProgress(ctx context.Context, userID, entryID int64, seasonDelta, episodeDelta int64) (*model.Entry, error)
	SetEntryProgress(ctx context.Context, userID, entryID int64, season, episode int64) (*model.Entry, error)
	DeleteEntry(ctx context.Context, userID, entryID int64) error

	// Rating
	RateEntry(ctx context.Context, userID, entryID int64, score model.Score) (*model.Entry, error)
	GetUserRating(ctx context.Context, userID, entryID int64) (model.Score, error)
}

type Service struct {
	storage storage.Storage
}

var _ AppService = (*Service)(nil)

func NewService(store storage.Storage) *Service {
	return &Service{
		storage: store,
	}
}

// =========================================================================
// User
// =========================================================================

func (s *Service) EnsureUser(ctx context.Context, u model.User) error {
	return s.storage.EnsureUser(ctx, u)
}

func (s *Service) GetUser(ctx context.Context, userID int64) (*model.User, error) {
	return s.storage.GetUser(ctx, userID)
}

// =========================================================================
// Stack
// =========================================================================

func (s *Service) CreateStack(ctx context.Context, ownerID int64, title string) (*model.StackInfo, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrEmptyTitle
	}
	if len([]rune(title)) > 128 {
		return nil, ErrTitleTooLong
	}

	stackID, err := s.storage.CreateStack(ctx, ownerID, title)
	if err != nil {
		return nil, fmt.Errorf("create stack: %w", err)
	}

	return s.storage.GetStackByID(ctx, stackID)
}

func (s *Service) GetUserStacks(ctx context.Context, userID int64) ([]model.StackInfo, error) {
	return s.storage.GetUserStacks(ctx, userID)
}

func (s *Service) GetStack(ctx context.Context, userID, stackID int64) (*model.StackInfo, error) {
	isMember, role, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	stack, err := s.storage.GetStackByID(ctx, stackID)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, ErrStackNotFound
	}
	stack.Role = role
	return stack, nil
}

func (s *Service) UpdateStackTitle(ctx context.Context, userID, stackID int64, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrEmptyTitle
	}
	if len([]rune(title)) > 128 {
		return ErrTitleTooLong
	}

	isMember, role, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return err
	}
	if !isMember || role != model.RoleOwner {
		return ErrNotOwner
	}

	return s.storage.UpdateStackTitle(ctx, stackID, title)
}

func (s *Service) DeleteStack(ctx context.Context, userID, stackID int64) error {
	isMember, role, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return err
	}
	if !isMember || role != model.RoleOwner {
		return ErrNotOwner
	}

	return s.storage.DeleteStack(ctx, stackID)
}

func (s *Service) LeaveStack(ctx context.Context, userID, stackID int64) error {
	isMember, role, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}
	if role == model.RoleOwner {
		return errors.New("stack owner cannot leave the stack; you must delete it or transfer ownership")
	}

	return s.storage.LeaveStack(ctx, userID, stackID)
}

func (s *Service) GetSelectedStackID(ctx context.Context, userID int64) (int64, error) {
	return s.storage.GetSelectedStackID(ctx, userID)
}

func (s *Service) SelectStack(ctx context.Context, userID, stackID int64) error {
	isMember, _, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}
	return s.storage.SetSelectedStack(ctx, userID, stackID)
}

// =========================================================================
// Invite
// =========================================================================

func (s *Service) CreateInvite(ctx context.Context, userID, stackID int64, ttl time.Duration) (*model.Invite, error) {
	// 1. Verify user is owner
	isMember, role, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return nil, err
	}
	if !isMember || role != model.RoleOwner {
		return nil, ErrNotOwner
	}

	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	code := generateInviteCode()
	invite := model.Invite{
		Code:      code,
		StackID:   stackID,
		CreatedBy: userID,
		ExpiresAt: time.Now().Add(ttl),
	}

	if err := s.storage.CreateInvite(ctx, invite); err != nil {
		return nil, fmt.Errorf("create invite record: %w", err)
	}

	return &invite, nil
}

func (s *Service) RedeemInvite(ctx context.Context, userID int64, code string) (*model.StackInfo, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, ErrInvalidCode
	}

	// 1. Check if invite exists
	inv, err := s.storage.GetInviteByCode(ctx, code)
	if err != nil || inv == nil {
		return nil, ErrInvalidCode
	}
	if inv.IsUsed || inv.IsExpired() {
		return nil, ErrInvalidCode
	}

	// 2. Check if user is already a member
	isMember, _, err := s.storage.IsStackMember(ctx, userID, inv.StackID)
	if err == nil && isMember {
		_ = s.storage.SetSelectedStack(ctx, userID, inv.StackID)
		st, _ := s.storage.GetStackByID(ctx, inv.StackID)
		return st, ErrAlreadyMember
	}

	// 3. Consume invite
	stack, err := s.storage.ConsumeInvite(ctx, code, userID)
	if err != nil {
		return nil, fmt.Errorf("consume invite: %w", err)
	}

	return stack, nil
}

func generateInviteCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return fmt.Sprintf("MEOW-%s", string(b))
}

// =========================================================================
// Entry
// =========================================================================

func (s *Service) AddEntry(ctx context.Context, userID, stackID int64, title, mediaType string, season, episode int64) (*model.Entry, error) {
	isMember, _, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrEmptyTitle
	}
	if len([]rune(title)) > 128 {
		return nil, ErrTitleTooLong
	}

	mediaType = strings.TrimSpace(mediaType)
	if mediaType == "" {
		mediaType = "Movie"
	} else if len([]rune(mediaType)) > 64 {
		mediaType = string([]rune(mediaType)[:64])
	}

	if season <= 0 {
		season = 1
	}
	if episode <= 0 {
		episode = 1
	}

	entry := model.Entry{
		StackID: stackID,
		Title:   title,
		Type:    mediaType,
		Season:  season,
		Episode: episode,
	}

	entryID, err := s.storage.AddEntry(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("add entry: %w", err)
	}

	return s.storage.GetEntryByID(ctx, entryID)
}

func (s *Service) GetEntry(ctx context.Context, userID, entryID int64) (*model.Entry, error) {
	entry, err := s.storage.GetEntryByID(ctx, entryID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, ErrEntryNotFound
	}

	isMember, _, err := s.storage.IsStackMember(ctx, userID, entry.StackID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	return entry, nil
}

func (s *Service) GetStackEntries(ctx context.Context, userID, stackID int64) ([]model.Entry, error) {
	isMember, _, err := s.storage.IsStackMember(ctx, userID, stackID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	return s.storage.GetStackEntries(ctx, stackID)
}

func (s *Service) UpdateEntryTitle(ctx context.Context, userID, entryID int64, title string) (*model.Entry, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrEmptyTitle
	}
	if len([]rune(title)) > 128 {
		return nil, ErrTitleTooLong
	}

	entry, err := s.GetEntry(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}

	if err := s.storage.UpdateEntryTitle(ctx, entry.ID, title); err != nil {
		return nil, err
	}

	return s.storage.GetEntryByID(ctx, entry.ID)
}

func (s *Service) UpdateEntryType(ctx context.Context, userID, entryID int64, mediaType string) (*model.Entry, error) {
	mediaType = strings.TrimSpace(mediaType)
	if mediaType == "" {
		mediaType = "Movie"
	} else if len([]rune(mediaType)) > 64 {
		mediaType = string([]rune(mediaType)[:64])
	}

	entry, err := s.GetEntry(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}

	if err := s.storage.UpdateEntryType(ctx, entry.ID, mediaType); err != nil {
		return nil, err
	}

	return s.storage.GetEntryByID(ctx, entry.ID)
}

func (s *Service) AdjustEntryProgress(ctx context.Context, userID, entryID int64, seasonDelta, episodeDelta int64) (*model.Entry, error) {
	entry, err := s.GetEntry(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}

	newSeason := entry.Season + seasonDelta
	if newSeason < 1 {
		newSeason = 1
	}

	newEpisode := entry.Episode + episodeDelta
	if newEpisode < 1 {
		newEpisode = 1
	}

	if err := s.storage.UpdateEntryProgress(ctx, entry.ID, newSeason, newEpisode); err != nil {
		return nil, err
	}

	return s.storage.GetEntryByID(ctx, entry.ID)
}

func (s *Service) SetEntryProgress(ctx context.Context, userID, entryID int64, season, episode int64) (*model.Entry, error) {
	entry, err := s.GetEntry(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}

	if season < 1 {
		season = 1
	}
	if episode < 1 {
		episode = 1
	}

	if err := s.storage.UpdateEntryProgress(ctx, entry.ID, season, episode); err != nil {
		return nil, err
	}

	return s.storage.GetEntryByID(ctx, entry.ID)
}

func (s *Service) DeleteEntry(ctx context.Context, userID, entryID int64) error {
	entry, err := s.GetEntry(ctx, userID, entryID)
	if err != nil {
		return err
	}

	return s.storage.DeleteEntry(ctx, entry.ID)
}

// =========================================================================
// Rating
// =========================================================================

func (s *Service) RateEntry(ctx context.Context, userID, entryID int64, score model.Score) (*model.Entry, error) {
	entry, err := s.GetEntry(ctx, userID, entryID)
	if err != nil {
		return nil, err
	}

	if err := s.storage.RateEntry(ctx, userID, entry.ID, score); err != nil {
		return nil, fmt.Errorf("rate entry: %w", err)
	}

	return s.storage.GetEntryByID(ctx, entry.ID)
}

func (s *Service) GetUserRating(ctx context.Context, userID, entryID int64) (model.Score, error) {
	entry, err := s.GetEntry(ctx, userID, entryID)
	if err != nil {
		return model.NotSeen, err
	}

	return s.storage.GetUserRating(ctx, userID, entry.ID)
}
