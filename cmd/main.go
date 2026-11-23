package main

import (
	"MeowSentinelBot/internal/bot/fsm"
	"MeowSentinelBot/internal/bot/handlers"
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/service"
	"MeowSentinelBot/internal/storage/sqlite"
	"MeowSentinelBot/internal/utils"
	"context"
	"errors"
	"fmt"
	"net/http"
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

	// Allow overriding SQLite DB path via environment variable (essential for Cloud Run volume mounts)
	if envDBPath := os.Getenv("DB_PATH"); envDBPath != "" {
		cfg.Bot.Db.Path = envDBPath
	}

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
		// Fallback to BOT_TOKEN if TELEGRAM_BOT_TOKEN isn't set
		if fallbackToken := os.Getenv("BOT_TOKEN"); fallbackToken != "" {
			token = fallbackToken
		} else {
			logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("getting bot token: %w", err))
			os.Exit(1)
		}
	}

	bot, err := telego.NewBot(token)
	if err != nil {
		logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("initializing bot: %w", err))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.LogEvent(service.LifeCycle, service.BotStart)

	// Server and webhook configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	webhookPath := os.Getenv("WEBHOOK_PATH")
	if webhookPath == "" {
		webhookPath = "/bot"
	}

	webhookSecret := os.Getenv("WEBHOOK_SECRET")

	// Set up HTTP mux
	mux := http.NewServeMux()

	// Container health check endpoint for Cloud Run
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Configure Telego webhook updates with secret token enforcement
	webhookURL := os.Getenv("WEBHOOK_URL")
	var updates <-chan telego.Update

	if webhookURL != "" {
		var secretOptions []string
		if webhookSecret != "" {
			secretOptions = append(secretOptions, webhookSecret)
		} else {
			logger.LogEvent(service.Warning, service.LogErrorText("WEBHOOK_SECRET is not set; incoming updates will not validate secret token header"))
		}

		// Automatically register webhook URL with Telegram API
		setParams := &telego.SetWebhookParams{
			URL:         webhookURL,
			SecretToken: webhookSecret,
		}
		if err := bot.SetWebhook(ctx, setParams); err != nil {
			logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("registering webhook with Telegram API: %w", err))
			os.Exit(1)
		}
		logger.LogEvent(service.LifeCycle, service.LogErrorText(fmt.Sprintf("Telegram webhook successfully registered to: %s", webhookURL)))

		webhookUpdates, err := bot.UpdatesViaWebhook(
			ctx,
			telego.WebhookHTTPServeMux(mux, webhookPath, secretOptions...),
		)
		if err != nil {
			logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("registering webhook updates: %w", err))
			os.Exit(1)
		}
		updates = webhookUpdates
	} else {
		logger.LogEvent(service.LifeCycle, service.LogErrorText("WEBHOOK_URL is not set; running in Long Polling mode (HTTP server remains active for /healthz)"))

		// Clean up any stale webhook on Telegram API so getUpdates receives messages
		_ = bot.DeleteWebhook(ctx, &telego.DeleteWebhookParams{})

		lpUpdates, err := bot.UpdatesViaLongPolling(ctx, nil)
		if err != nil {
			logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("starting long polling: %w", err))
			os.Exit(1)
		}
		updates = lpUpdates
	}

	bh, err := th.NewBotHandler(bot, updates)
	if err != nil {
		logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("creating bot handler: %w", err))
		os.Exit(1)
	}

	botHandler := handlers.NewBotHandler(bot, appService, fsmStore, logger)
	botHandler.RegisterHandlers(bh)

	// Start bot update processor
	go func() {
		if err := bh.Start(); err != nil {
			logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("starting bot handler: %w", err))
		}
	}()

	// Configure HTTP server with production timeouts to mitigate Slowloris attacks
	server := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start HTTP server
	go func() {
		logger.LogEvent(service.LifeCycle, service.LogErrorText(fmt.Sprintf("HTTP server listening on %s (webhook: %s, health: /healthz)", server.Addr, webhookPath)))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.LogEvent(service.Fatal, service.PanicLog, fmt.Errorf("http server terminated: %w", err))
			cancel()
		}
	}()

	// Wait for termination signal (SIGINT or SIGTERM from Cloud Run)
	<-ctx.Done()
	logger.LogEvent(service.LifeCycle, service.BotStop)

	// Graceful shutdown sequence
	// 1. Stop HTTP server (reject new requests, wait for in-flight requests to complete)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.LogEvent(service.Error, service.UnknownError, int64(0), "http_server_shutdown", err)
	}

	// 2. Stop bot handler (finish processing queued updates)
	_ = bh.Stop()

	// 3. Deferred closures (fsmStore.Close() and store.Close()) will now run and safely flush database
}
