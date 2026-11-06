package main

import (
	"MeowSentinelBot/internal/bot/fsm"
	"MeowSentinelBot/internal/bot/handlers"
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/service"
	"MeowSentinelBot/internal/storage/sqlite"
	"MeowSentinelBot/internal/utils"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func main() {
	cfg := config.MustNewConfig("configs/config.json")
	logger := service.NewStdLogger()

	store, err := sqlite.NewStorage(&cfg.Bot.Db)
	if err != nil {
		logger.LogEvent(service.Fatal, service.CreatingDBErrorLog, err)
		os.Exit(1)
	}
	defer store.Close()

	appService := service.NewService(store)
	fsmStore := fsm.NewStore(5 * time.Minute)
	defer fsmStore.Close()

	token, err := utils.GetEnv(cfg.Bot.Telegram.EnvVar)
	if err != nil {
		logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("getting bot token: %w", err))
		os.Exit(1)
	}

	bot, err := telego.NewBot(token)
	if err != nil {
		logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("initializing bot: %w", err))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.LogEvent(service.LifeCycle, service.BotStart)

	updates, err := bot.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("long polling updates: %w", err))
		os.Exit(1)
	}

	bh, err := th.NewBotHandler(bot, updates)
	if err != nil {
		logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("creating bot handler: %w", err))
		os.Exit(1)
	}

	botHandler := handlers.NewBotHandler(bot, appService, fsmStore, logger)
	botHandler.RegisterHandlers(bh)

	go func() {
		if err := bh.Start(); err != nil {
			logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("starting bot handler: %w", err))
		}
	}()

	<-ctx.Done()
	logger.LogEvent(service.LifeCycle, service.BotStop)
	_ = bh.Stop()
}
