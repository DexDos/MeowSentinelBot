package storage

import (
	"MeowSentinelBot/internal/model"
	"context"
)

type Storage interface {
	// Users
	EnsureUser(ctx context.Context, user model.User) error
	GetUser(ctx context.Context, userID int64) (*model.User, error)

	// Stacks
	CreateStack(ctx context.Context, ownerID int64, title string) (int64, error)
	GetStackByID(ctx context.Context, stackID int64) (*model.StackInfo, error)
	GetUserStacks(ctx context.Context, userID int64) ([]model.StackInfo, error)
	UpdateStackTitle(ctx context.Context, stackID int64, title string) error
	DeleteStack(ctx context.Context, stackID int64) error
	LeaveStack(ctx context.Context, userID, stackID int64) error
	SetSelectedStack(ctx context.Context, userID, stackID int64) error
	GetSelectedStackID(ctx context.Context, userID int64) (int64, error)
	IsStackMember(ctx context.Context, userID, stackID int64) (bool, string, error)

	// Invites
	CreateInvite(ctx context.Context, invite model.Invite) error
	GetInviteByCode(ctx context.Context, code string) (*model.Invite, error)
	ConsumeInvite(ctx context.Context, code string, userID int64) (*model.StackInfo, error)

	// Entries
	AddEntry(ctx context.Context, entry model.Entry) (int64, error)
	GetEntryByID(ctx context.Context, entryID int64) (*model.Entry, error)
	GetStackEntries(ctx context.Context, stackID int64) ([]model.Entry, error)
	UpdateEntryTitle(ctx context.Context, entryID int64, title string) error
	UpdateEntryType(ctx context.Context, entryID int64, mediaType string) error
	UpdateEntryProgress(ctx context.Context, entryID int64, season, episode int64) error
	DeleteEntry(ctx context.Context, entryID int64) error

	// Ratings
	RateEntry(ctx context.Context, userID, entryID int64, score model.Score) error
	GetUserRating(ctx context.Context, userID, entryID int64) (model.Score, error)

	// Transaction & lifecycle
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
	Close() error
}
