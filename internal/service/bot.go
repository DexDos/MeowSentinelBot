package service

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/db"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
)

type BotService interface {
	StartBot()
}

type BotServiceStd struct {
	bot    *telego.Bot
	db     db.Storage
	logger Logger
}

func (s *BotServiceStd) StartBot() {
	updates, _ := s.bot.UpdatesViaLongPolling(context.Background(), nil)
	for update := range updates {
		fmt.Printf("Update: %+v\n", update)
	}
}

func NewBotServiceStd(logger Logger, cfg *config.Bot) *BotServiceStd {

	dbService, err := db.NewSqliteStorage(&cfg.Db)
	if err != nil {
		logger.tempErrorHandle(
			fmt.Errorf("error creating db service: %w", err),
			Fatal,
		)
	}

	token, err := getEnv(cfg.Telegram.EnvVar)
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
		bot:    bot,
	}
}

func getEnv(envVar string) (string, error) {
	if err := godotenv.Load(".env"); err != nil {
		return "", fmt.Errorf("error loading .env file: %w", err)
	}

	token := os.Getenv(envVar)
	if token == "" {
		return "", errors.New("no telegram bot token provided")
	}
	return token, nil
}
