package service

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/db"
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/utils"
	"context"
	"fmt"

	"github.com/mymmrac/telego"
)

type BotService interface {
	GetTelegramBot() *telego.Bot
	AddNewUser(context.Context, *telego.User) error
	GetUserByID(context.Context, int64) bool
	CreateNewUserStack(context.Context, *telego.User, string) error
	JoinUserStack(context.Context, *telego.User, int64) error
	GetOwnedStacks(context.Context, *telego.User) ([]model.StackInfo, error)
	GetUserStacks(context.Context, *telego.User) ([]model.StackInfo, error)
	GetStackInfo(context.Context, int64) (*model.StackInfo, error)
	SelectUserStack(context.Context, *telego.User, int64) error
	GetCurrentStackEntries(context.Context, *telego.User) ([]model.Entry, error)
}

type BotServiceStd struct {
	Bot    *telego.Bot
	db     db.Storage
	logger Logger
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
	stackID, err := s.db.GetCurrentStack(ctx, u)
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
