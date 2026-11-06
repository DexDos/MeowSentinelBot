package handlers

import (
	"MeowSentinelBot/internal/bot/fsm"
	"MeowSentinelBot/internal/service"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type BotHandler struct {
	bot     *telego.Bot
	service service.AppService
	fsm     *fsm.Store
	logger  service.Logger
}

func NewBotHandler(bot *telego.Bot, svc service.AppService, fsmStore *fsm.Store, logger service.Logger) *BotHandler {
	return &BotHandler{
		bot:     bot,
		service: svc,
		fsm:     fsmStore,
		logger:  logger,
	}
}

// RegisterHandlers registers all command, message, and callback query handlers
func (h *BotHandler) RegisterHandlers(bh *th.BotHandler) {
	// Handlers will be fully wired in Stage 3 according to approved user flow
}
