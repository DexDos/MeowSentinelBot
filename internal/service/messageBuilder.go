package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	sqlite3 "modernc.org/sqlite/lib"
)

type Responder interface {
	StartBot()
}

var (
	PendingStd       = 10 * time.Minute
	PendingProlonged = 1 * time.Hour
)

type APIBotHandler struct {
	teleBot *telego.Bot
	bot     BotService
	log     Logger
	fsm     *InMemFSMStore
}

func (h *APIBotHandler) StartBot() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	h.log.LogEvent("Bot is deployed", LifeCycle)
	updates, err := h.teleBot.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		h.log.panicHandle(fmt.Errorf("UpdatesViaLongPolling error: %w", err))
	}

	// MAIN BOT HANDLER
	bh, err := th.NewBotHandler(h.teleBot, updates)
	if err != nil {
		h.log.panicHandle(fmt.Errorf("NewBotHandler: %w", err))
	}

	// =====================================================================
	// ====================== HANDLERS REGISTRATION ========================
	// =====================================================================

	//bh.Group()

	bh.Handle(h.handleStart(), th.CommandEqual("start"))
	bh.HandleCallbackQuery(h.handleAddNewStackCmd, th.CallbackDataPrefix("stack:new-stack"))
	bh.HandleCallbackQuery(h.handleAddExistingStackCmd, th.CallbackDataPrefix("stack:join-stack"))

	bh.Handle(h.handleStackManagement(), th.AnyMessageWithText())
	// create-or-chose-stack
	// stack chosen -> watch stack
	// actions: filter, edit, add, remove, ...
	// ... : (extra actions) : share, unsubscribe, delete (for owners)

	// =====================================================================
	// =====================================================================

	go func() {
		if err := bh.Start(); err != nil {
			h.log.panicHandle(fmt.Errorf("panic when tried to start bot: %w", err))
		}
	}()

	<-ctx.Done()
	h.log.LogEvent("Bot is stopped", LifeCycle)
	_ = bh.Stop()
}

func (h *APIBotHandler) handleStart() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)

		keyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("New stack").WithCallbackData("stack:new-stack"),
				tu.InlineKeyboardButton("Join stack").WithCallbackData("stack:join-stack"),
			),
		)

		invitationMSG := tu.Message(chatID, "invitation").WithReplyMarkup(keyboard)

		_, err := h.teleBot.SendMessage(ctx, invitationMSG)
		if err != nil {
			h.log.permErrorHandle(
				fmt.Errorf(
					"failed to send invitational message to [%v] %v due to %w",
					update.Message.From.ID,
					update.Message.From.Username,
					err,
				),
				Error,
			)
			return err
		}
		return nil
	}
}
func (h *APIBotHandler) handleStackManagement() th.Handler {
	return h.handleStackNameInput
}

func (h *APIBotHandler) handleStackNameInput(_ *th.Context, update telego.Update) error {

	return nil
}

func (h *APIBotHandler) handleAddNewStackCmd(ctx *th.Context, callback telego.CallbackQuery) error {
	h.fsm.SetState(callback.From.ID, StateWaitForNewStackName, PendingStd)

	//TODO send text forcing user to type desired stack name

	err := h.teleBot.AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID).WithText("Pending..."))

	chatID := tu.ID(callback.Message.GetChat().ID)
	msgID := tu.ID((int64)(callback.Message.Message().MessageID))
	msg := tu.EditMessageText()
	return err
}

func (h *APIBotHandler) handleAddExistingStackCmd(ctx *th.Context, callback telego.CallbackQuery) error {
	h.fsm.SetState(callback.From.ID, StateWaitForExistingStackName, PendingStd)
	//TODO send text forcing user to type desired stack name
	return nil
}

func NewAPIBotHandler(l Logger, bot BotService) *APIBotHandler {
	teleBot := bot.GetTelegramBot()
	return &APIBotHandler{
		teleBot: teleBot,
		bot:     bot,
		log:     l,
		fsm:     NewInMemFSMStore(),
	}
}
