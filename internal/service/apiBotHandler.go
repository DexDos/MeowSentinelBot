package service

import (
	"MeowSentinelBot/internal/model"
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

type StateStrategy func(ctx *th.Context, update telego.Update) error
type RespondFunc func(*model.EventContext) *telego.EditMessageTextParams
type APIBotHandler struct {
	teleBot   *telego.Bot
	responder Responder
	bot       BotService
	log       Logger
	fsm       *InMemFSMStore
	sr        map[State]StateStrategy
}

func (h *APIBotHandler) StartBot(r *Responder) {
	h.sr = map[State]StateStrategy{
		StateNone:                     h.handleStateNone,
		StateWaitForNewStackName:      h.handleNewStackInput,
		StateWaitForExistingStackName: h.handleJoinStackInput,
	}
	h.responder = *r

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	h.log.LogEvent(LifeCycle, BotStart)
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
	bh.Handle(h.handleMainMenu(), th.CommandEqual("menu"))
	bh.HandleCallbackQuery(
		h.callbackWithState(StateWaitForNewStackName, h.responder.RespondToNewStackCmdCallback),
		th.CallbackDataEqual("stack:new-stack"),
	)
	bh.HandleCallbackQuery(
		h.callbackWithState(StateWaitForExistingStackName, h.responder.RespondToJoinStackCmdCallback),
		th.CallbackDataEqual("stack:join-stack"),
	)
	bh.HandleCallbackQuery(h.handleCancellation, th.CallbackDataPrefix("action:deny"))

	bh.Handle(h.handleStateManagement(), th.AnyMessageWithText())
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
	h.log.LogEvent(LifeCycle, BotStop)
	_ = bh.Stop()
}

func (h *APIBotHandler) handleStart() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		ev := model.ExtractEvent(update)
		err := h.bot.AddNewUser(ctx, ev.User)
		if err != nil {
			return err
		}

		invitationMSG := h.responder.RespondToStart(&ev)
		_, err = h.teleBot.SendMessage(ctx, invitationMSG)
		if err != nil {
			h.log.permErrorHandle(
				Error,
				MessageSendingFailureLog,
				ev.User.ID,
				ev.User.Username,
				err,
			)
			return err
		}
		return nil
	}
}
func (h *APIBotHandler) handleStateManagement() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		userState := h.fsm.GetState(update.Message.From.ID)

		handler, ok := h.sr[userState]
		if !ok {
			return nil // TODO handle random answers
		}
		return handler(ctx, update)
	}
}

func (h *APIBotHandler) handleStackSettingsMenu(ctx *th.Context, update telego.Update) error {
	return nil
}
func (h *APIBotHandler) handleStackView(ctx *th.Context, update telego.Update) error {
	return nil
}

func (h *APIBotHandler) handleCancellation(ctx *th.Context, callback telego.CallbackQuery) error {
	ev := model.ExtractEventFromCallback(&callback)
	h.fsm.CleanState(ev.UserID)

	err := h.teleBot.AnswerCallbackQuery(
		ctx.Context(),
		tu.CallbackQuery(ev.CallbackID).
			WithText("Pending..."))
	if err != nil {
		return err
	}

	editedMsg := h.responder.RespondToCancellationCallback(ev)
	_, err = h.teleBot.EditMessageText(ctx, editedMsg)

	return err
}

func (h *APIBotHandler) handleMainMenu() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		ev := model.ExtractEvent(update)
		stacks, err := h.bot.GetUserStacks(ctx, ev.User)
		if err != nil {
			h.log.permErrorHandle(
				Error,
				MessageSendingFailureLog,
				ev.User.ID,
				ev.User.Username,
				err,
			)
			return err
		}

		msg := h.responder.RespondWithMainMenu(&ev, stacks)
		_, err = h.teleBot.SendMessage(ctx.Context(), msg)
		return err
	}
}

func (h *APIBotHandler) handleStateNone(ctx *th.Context, update telego.Update) error {
	return nil
}

func (h *APIBotHandler) handleNewStackInput(ctx *th.Context, update telego.Update) error {
	ev := model.ExtractEvent(update)
	title := update.Message.Text

	defer h.fsm.CleanState(ev.UserID)

	err := h.bot.CreateNewUserStack(ctx.Context(), ev.User, title)
	if err != nil {
		h.log.permErrorHandle(
			Error,
			InvalidNewStackNameLog,
			title,
			ev.UserID,
			err,
		)
		msg := h.responder.RespondWithError(
			&ev,
			InvalidStackName,
		)
		_, err := h.teleBot.SendMessage(ctx.Context(), msg)
		return err
	}

	// TODO goto stack menu
	return nil
}

func (h *APIBotHandler) handleJoinStackInput(ctx *th.Context, update telego.Update) error {
	ev := model.ExtractEvent(update)

	defer h.fsm.CleanState(ev.UserID)

	stackID, err := strconv.ParseInt(update.Message.Text, 10, 64)
	if err != nil {
		h.log.permErrorHandle(
			Error,
			InvalidStackIDLog,
			stackID,
			ev.User.ID,
			err,
		)
		msg := h.responder.RespondWithError(
			&ev,
			InvalidStackID,
		)
		_, err := h.teleBot.SendMessage(ctx.Context(), msg)
		return err
	}

	err = h.bot.JoinUserStack(ctx.Context(), ev.User, stackID)
	// TODO goto joined stack menu
	return err
}

func (h *APIBotHandler) callbackWithState(state State, respondFn RespondFunc) th.CallbackQueryHandler {
	return func(ctx *th.Context, cb telego.CallbackQuery) error {
		ev := model.ExtractEventFromCallback(&cb)
		h.fsm.SetState(ev.UserID, state, PendingStd)

		err := h.teleBot.AnswerCallbackQuery(
			ctx.Context(),
			tu.CallbackQuery(ev.CallbackID).
				WithText("Pending..."))
		if err != nil {
			return err
		}

		editedMsg := respondFn(ev)
		_, err = h.teleBot.EditMessageText(ctx.Context(), editedMsg)
		return err
	}
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
