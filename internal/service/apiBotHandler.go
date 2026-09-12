package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type APIHandler interface {
	StartBot(*Responder)
}

var (
	PendingStd       = 10 * time.Minute
	PendingProlonged = 1 * time.Hour
)

type StateStrategy func(ctx *th.Context, update telego.Update, r *Responder) error

type APIBotHandler struct {
	teleBot *telego.Bot
	bot     BotService
	log     Logger
	fsm     *InMemFSMStore
	sr      map[State]StateStrategy
}

func (h *APIBotHandler) StartBot(r *Responder) {
	h.sr = map[State]StateStrategy{
		StateNone:                     h.handleStateNone,
		StateWaitForNewStackName:      h.handleNewStackInput,
		StateWaitForExistingStackName: h.handleJoinStackInput,
	}

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

	bh.Handle(h.handleStateManagement(r), th.AnyMessageWithText())
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

		invitationMSG := tu.Message(chatID, "invitation").
			WithReplyMarkup(keyboard).WithParseMode(telego.ModeMarkdown)

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
func (h *APIBotHandler) handleStateManagement(r *Responder) th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		if update.Message == nil {
			return nil
		}
		userID := update.Message.Chat.ID
		userState := h.fsm.GetState(userID)
		if userState == StateNone {
			return nil
		}

		handler, ok := h.sr[userState]
		if !ok {
			return nil
		}
		return handler(ctx, update, r)
	}
}

func (h *APIBotHandler) handleStateNone(ctx *th.Context, update telego.Update, _ *Responder) error {
	user := update.Message.From

	stacks, err := h.bot.GetUserStacks(ctx, user)
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

	// TODO make responder method to take respMeth(stacks []model.Stack) []string {...}
	// For stack of stacks write an embeded message for user to choose a stack from keyboard
	return nil
}

func (h *APIBotHandler) handleNewStackInput(ctx *th.Context, update telego.Update, r *Responder) error {
	//chatID := update.Message.Chat.ID
	user := update.Message.From
	title := update.Message.Text

	defer h.fsm.CleanState(user.ID)

	err := h.bot.CreateNewUserStack(ctx.Context(), user, title)
	if err != nil {
		h.log.permErrorHandle(
			fmt.Errorf(
				"error while tried to create a stack %s for user id[%d]: %w",
				title,
				user.ID,
				err,
			),
			Error,
		)
		return err
	}

	return h.handleStateNone(ctx, update, r)
}

func (h *APIBotHandler) handleJoinStackInput(ctx *th.Context, update telego.Update, r *Responder) error {
	user := update.Message.From
	stackID, err := strconv.ParseInt(update.Message.Text, 10, 64)
	if err != nil {
		h.log.permErrorHandle(
			fmt.Errorf(
				"error while trying to convert user input (%v) into integer. UserID[%d]: %w",
				stackID,
				user.ID,
				err,
			),
			Error,
		)
		return err
	}

	err = h.bot.JoinUserStack(ctx.Context(), user, stackID)
	return err
}

func (h *APIBotHandler) handleAddNewStackCmd(ctx *th.Context, callback telego.CallbackQuery) error {
	h.fsm.SetState(callback.From.ID, StateWaitForNewStackName, PendingStd)

	err := h.teleBot.AnswerCallbackQuery(
		ctx.Context(),
		tu.CallbackQuery(callback.ID).
			WithText("Pending..."))
	if err != nil {
		return err
	}

	chatID := tu.ID(callback.Message.GetChat().ID)
	msgID := callback.Message.Message().MessageID
	editedMsg := tu.EditMessageText(
		chatID,
		msgID,
		"**Action chosen: ** create new stack\n\\> Please, enter new stack name:",
	).WithParseMode(telego.ModeMarkdown)
	_, err = h.teleBot.EditMessageText(ctx.Context(), editedMsg)
	return err
}

func (h *APIBotHandler) handleAddExistingStackCmd(ctx *th.Context, callback telego.CallbackQuery) error {
	h.fsm.SetState(callback.From.ID, StateWaitForExistingStackName, PendingStd)

	err := h.teleBot.AnswerCallbackQuery(
		ctx.Context(),
		tu.CallbackQuery(callback.ID).
			WithText("Pending..."))
	if err != nil {
		return err
	}

	chatID := tu.ID(callback.Message.GetChat().ID)
	msgID := callback.Message.Message().MessageID
	editedMsg := tu.EditMessageText(
		chatID,
		msgID,
		"**Action chosen: ** join stack\n\\> Please, enter new stack ID\n *The stack owner must request an identifier.*",
	).WithParseMode(telego.ModeMarkdown)
	_, err = h.teleBot.EditMessageText(ctx.Context(), editedMsg)
	return err
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
