package service

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/db"
	"MeowSentinelBot/internal/utils"
	"fmt"

	"github.com/mymmrac/telego"
)

type BotService interface {
	GetTelegramBot() *telego.Bot
}

type BotServiceStd struct {
	Bot    *telego.Bot
	db     db.Storage
	logger Logger
}

func (s *BotServiceStd) GetTelegramBot() *telego.Bot {
	return s.Bot
}

func NewBotServiceStd(logger Logger, cfg *config.Bot) *BotServiceStd {

	dbService, err := db.NewSqliteStorage(&cfg.Db)
	if err != nil {
		logger.tempErrorHandle(
			fmt.Errorf("error creating db service: %w", err),
			Fatal,
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
