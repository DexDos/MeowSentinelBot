package service

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/db"
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/utils"
	"context"
	"fmt"
	"strconv"

	"github.com/mymmrac/telego"
)

type BotService interface {
	GetTelegramBot() *telego.Bot
	AddNewUser(ctx context.Context, tUser *telego.User) error
	GetUserByID(ctx context.Context, uid int64) bool
	CreateNewUserStack(ctx context.Context, tUser *telego.User, title string) error
	JoinUserStack(ctx context.Context, tUser *telego.User, stackID int64) error
	GetOwnedStacks(ctx context.Context, tUser *telego.User) ([]model.StackInfo, error)
	GetUserStacks(ctx context.Context, tUser *telego.User) ([]model.StackInfo, error)
	GetStackInfo(ctx context.Context, stackID int64) (*model.StackInfo, error)
	SelectUserStack(ctx context.Context, tUser *telego.User, stackID int64) error
	GetSelectedUserStackID(ctx context.Context, tUser *telego.User) (int64, error)
	GetCurrentStackEntries(ctx context.Context, tUser *telego.User) ([]model.Entry, error)
	AddNewEntry(ctx context.Context, tUser *telego.User, title string) (int64, error)
	GetEntryByID(ctx context.Context, entryID int64) (*model.Entry, error)
	UpdateEntry(ctx context.Context, entryID int64, field db.EntryField, payload string) error
	DefineAndPerformChange(ctx context.Context, userID, entryID int64, field db.EntryField, rawModValue int64) (*model.Entry, error)
}

type InlineEntryUpdateFn func(ctx context.Context, userID, entryID int64, field db.EntryField, payload int64) (*model.Entry, error)

type BotServiceStd struct {
	Bot    *telego.Bot
	db     db.Storage
	logger Logger
}

func (s *BotServiceStd) GetEntryByID(ctx context.Context, entryID int64) (*model.Entry, error) {
	if entryID <= 0 {
		return nil, fmt.Errorf("invalid entry id")
	}
	entry, err := s.db.GetEntryByID(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry by id %d: %w", entryID, err)
	}
	if entry == nil {
		return nil, fmt.Errorf("entry not found")
	}
	return entry, nil
}

func (s *BotServiceStd) GetSelectedUserStackID(ctx context.Context, tUser *telego.User) (int64, error) {
	u := model.NewTelegramUser(tUser, "")
	stackID, err := s.db.GetCurrentStackID(ctx, u)
	if err != nil {
		return 0, fmt.Errorf("error on getting current user stack id: %w", err)
	}
	return stackID, nil
}

func (s *BotServiceStd) GetStackInfo(ctx context.Context, stackID int64) (*model.StackInfo, error) {
	info, err := s.db.GetStackByID(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("error while trying to fetch stack info: %w", err)
	}
	return info, nil
}

func (s *BotServiceStd) GetUserByID(ctx context.Context, id int64) bool {
	//s.db.GetUser()
	panic("implement me")
}

func (s *BotServiceStd) AddNewUser(ctx context.Context, tUser *telego.User) error {
	return s.db.Transaction(ctx, func(txCtx context.Context) error {
		u := model.NewTelegramUser(tUser, "")
		return s.db.AddUser(txCtx, u)
	})
}

func (s *BotServiceStd) CreateNewUserStack(ctx context.Context, tUser *telego.User, title string) error {
	return s.db.Transaction(ctx, func(txCtx context.Context) error {
		u := model.NewTelegramUser(tUser, title)

		stackID, err := s.db.CreateStack(txCtx, u)
		if err != nil {
			return fmt.Errorf("create stack: %w", err)
		}

		u.SetCurrentStackID(stackID)
		err = s.db.AddStackToUser(txCtx, u)
		if err != nil {
			return fmt.Errorf("add stack to user's: %w", err)
		}

		err = s.db.SetCurrentStack(txCtx, u)
		if err != nil {
			return fmt.Errorf("set current stack: %w", err)
		}
		return nil
	})
}
func (s *BotServiceStd) JoinUserStack(ctx context.Context, tUser *telego.User, stackID int64) error {
	// TODO executor := s.db.GetExecutor(ctx)
	u := model.NewTelegramUser(tUser, "")
	u.SetCurrentStackID(stackID)
	err := s.db.AddStackToUser(ctx, u)
	if err != nil {
		return err
	}

	err = s.db.SetCurrentStack(ctx, u)
	return err
}

func (s *BotServiceStd) GetOwnedStacks(ctx context.Context, tUser *telego.User) ([]model.StackInfo, error) {
	panic("implement me")
}
func (s *BotServiceStd) GetUserStacks(ctx context.Context, tUser *telego.User) ([]model.StackInfo, error) {
	user := model.NewTelegramUser(tUser, "")
	stacks, err := s.db.GetUserStacks(ctx, user)
	if err != nil {
		return nil, err
	}
	if len(stacks) == 0 {
		return nil, nil
	}

	return stacks, nil

}

func (s *BotServiceStd) GetTelegramBot() *telego.Bot {
	return s.Bot
}

// SelectUserStack pins selected stack for user, so that they access it quickly
func (s *BotServiceStd) SelectUserStack(ctx context.Context, tUser *telego.User, stackID int64) error {
	u := model.NewTelegramUser(tUser, "")
	u.SetCurrentStackID(stackID)
	return s.db.SetCurrentStack(ctx, u)
}

func (s *BotServiceStd) GetCurrentStackEntries(ctx context.Context, tUser *telego.User) ([]model.Entry, error) {
	u := model.NewTelegramUser(tUser, "")
	stackID, err := s.db.GetCurrentStackID(ctx, u)
	if stackID == 0 {
		if err != nil {
			return nil, fmt.Errorf("error on fetching current stack ID: %w", err)
		}
		return nil, nil
	}
	entries, err := s.db.GetStackRecords(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("error on fetching stack records: %w", err)
	}
	return entries, nil
}

func (s *BotServiceStd) AddNewEntry(ctx context.Context, tUser *telego.User, payload string) (int64, error) {
	u := model.NewTelegramUser(tUser, payload)
	entryID, err := s.db.AddEntry(ctx, u)
	if err != nil {
		return 0, fmt.Errorf("error on adding new entry: %w", err)
	}
	return entryID, nil
}
func (s *BotServiceStd) UpdateEntry(ctx context.Context, entryID int64, field db.EntryField, payload string) error {
	if field == db.EntryRatingField {
		panic("implement me")
		//TODO make the respective behaviour
	}

	err := s.db.UpdateEntryField(ctx, entryID, field, payload)
	if err != nil {
		return fmt.Errorf("error on updating entry: %w", err)
	}
	return nil
}

func (s *BotServiceStd) DefineAndPerformChange(
	ctx context.Context,
	userID, entryID int64,
	field db.EntryField,
	rawModValue int64,
) (*model.Entry, error) {
	entry, err := s.GetEntryByID(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("error on getting entry: %w", err)
	}

	switch field {
	case db.EntryCurrentSeasonField:
		return s.InlineSeasonScroll(ctx, entry, rawModValue)
	case db.EntryCurrentEpisodeField:
		return s.InlineEpisodeScroll(ctx, entry, rawModValue)
	case db.EntryRatingField:
		return s.InlineRate(ctx, entry, userID, rawModValue)
	default:
		return nil, fmt.Errorf("invalid entry field")
	}
}
func (s *BotServiceStd) InlineRate(ctx context.Context, entry *model.Entry, userID, payload int64) (*model.Entry, error) {
	score := model.Rating{}.SetScore(payload)

	err := s.db.RateEntry(ctx, userID, entry.Id, score)
	if err != nil {
		return nil, fmt.Errorf("error on inline rate: %w", err)
	}

	updatedEntry, err := s.GetEntryByID(ctx, entry.Id)
	if err != nil {
		return nil, fmt.Errorf("error on getting updated entry: %w", err)
	}
	return updatedEntry, nil
}
func (s *BotServiceStd) InlineSeasonScroll(ctx context.Context, entry *model.Entry, payload int64) (*model.Entry, error) {
	updatedSeason := entry.Status.Season + payload
	str := strconv.Itoa(int(updatedSeason))
	err := s.db.UpdateEntryField(ctx, entry.Id, db.EntryCurrentSeasonField, str)
	if err != nil {
		return nil, fmt.Errorf("error on inline season scroll: %w", err)
	}
	updatedEntry, err := s.GetEntryByID(ctx, entry.Id)
	if err != nil {
		return nil, fmt.Errorf("error on updating entry: %w", err)
	}
	return updatedEntry, nil
}
func (s *BotServiceStd) InlineEpisodeScroll(ctx context.Context, entry *model.Entry, payload int64) (*model.Entry, error) {
	updatedEpisode := entry.Status.LastEpisodeSeen + payload
	str := strconv.Itoa(int(updatedEpisode))
	err := s.db.UpdateEntryField(ctx, entry.Id, db.EntryCurrentEpisodeField, str)
	if err != nil {
		return nil, fmt.Errorf("error on inline season scroll: %w", err)
	}
	updatedEntry, err := s.GetEntryByID(ctx, entry.Id)
	if err != nil {
		return nil, fmt.Errorf("error on updating entry: %w", err)
	}
	return updatedEntry, nil
}

func NewBotServiceStd(logger Logger, cfg *config.Bot) *BotServiceStd {

	dbService, err := db.NewSqliteStorage(&cfg.Db)
	if err != nil {
		logger.tempErrorHandle(
			Error,
			CreatingDBErrorLog,
			err,
		)
	}

	token, err := utils.GetEnv(cfg.Telegram.EnvVar)
	if err != nil {
		logger.panicHandle(fmt.Errorf("error getting telegram token: %w", err))
	}
	bot, err := telego.NewBot(token, telego.WithDefaultDebugLogger())
	if err != nil {
		logger.panicHandle(fmt.Errorf("error initializing bot: %w", err))
	}

	return &BotServiceStd{
		logger: logger,
		db:     dbService,
		Bot:    bot,
	}
}
