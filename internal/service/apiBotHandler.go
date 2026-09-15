package service

import (
	"MeowSentinelBot/internal/db"
	"MeowSentinelBot/internal/model"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	PendingStd       = 20 * time.Second
	PendingProlonged = 2 * time.Minute
)

type StateStrategy func(ctx *th.Context, update telego.Update) error
type RespondFuncSend func(*model.EventContext) *telego.SendMessageParams
type RespondFuncEdit func(*model.EventContext) *telego.EditMessageTextParams
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
		StateNone:                     h.stateNone,
		StateWaitForNewStackName:      h.newStackInput,
		StateWaitForExistingStackName: h.joinStackInput,
		StateWaitForNewEntryTitle: h.recordModification(
			db.EntryTitleField,
			h.responder.RespondToFieldUpdate,
			StateWaitForNewEntryType,
		),
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
	// START
	bh.Handle(h.startCmd(), th.CommandEqual("start"))

	// MENU
	//     — MAIN
	bh.Handle(func(ctx *th.Context, upd telego.Update) error {
		return h.mainMenu(ctx, model.ExtractEvent(upd))
	}, th.CommandEqual("menu"))
	bh.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
		_ = h.teleBot.AnswerCallbackQuery(
			ctx.Context(),
			tu.CallbackQuery(query.ID).
				WithText("Pending..."),
		)
		return h.mainMenu(ctx, model.ExtractEventFromCallback(&query))
	}, th.CallbackDataPrefix("menu:main"))

	// STACK MANAGEMENT
	bh.HandleCallbackQuery(
		h.callbackWithState(StateWaitForNewStackName, h.responder.RespondToNewStackCmdCallback),
		th.CallbackDataEqual("stack:new-stack"),
	)
	bh.HandleCallbackQuery(
		h.callbackWithState(StateWaitForExistingStackName, h.responder.RespondToJoinStackCmdCallback),
		th.CallbackDataEqual("stack:join-stack"),
	)
	bh.HandleCallbackQuery(h.stackView(), th.CallbackDataPrefix("stack:view:"))

	// TECHNICAL
	bh.HandleCallbackQuery(h.cancellation(), th.CallbackDataEqual("action:deny"))

	// STATE-DRIVEN (usually when direct user input required)
	bh.Handle(h.stateManagement(), th.AnyMessageWithText())
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

func (h *APIBotHandler) startCmd() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		ev := model.ExtractEvent(update)
		err := h.bot.AddNewUser(ctx, ev.User)
		if err != nil {
			return err
		}

		invitationMSG := h.responder.RespondToStart(ev)
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
func (h *APIBotHandler) stateManagement() th.Handler {
	return func(ctx *th.Context, update telego.Update) error {
		userState := h.fsm.GetState(update.Message.From.ID)

		handler, ok := h.sr[userState]
		if !ok {
			return nil // TODO handle random answers
		}
		return handler(ctx, update)
	}
}

func (h *APIBotHandler) stackSettingsMenu() th.CallbackQueryHandler {
	return func(ctx *th.Context, callback telego.CallbackQuery) error {
		return nil
	}
}
func (h *APIBotHandler) stackView() th.CallbackQueryHandler {
	return func(ctx *th.Context, callback telego.CallbackQuery) error {
		ev := model.ExtractEventFromCallback(&callback)
		stackID, err := strconv.ParseInt(
			strings.TrimPrefix(callback.Data, "stack:view:"),
			10,
			64,
		)
		if err != nil {
			h.log.permErrorHandle(
				Error,
				InvalidStackIDLog,
				stackID,
				ev.User.ID,
				err,
			)
			return err
		}

		defer func() {
			_ = h.teleBot.AnswerCallbackQuery(
				ctx.Context(),
				tu.CallbackQuery(ev.CallbackID).
					WithText("Pending..."))
			if err != nil {
				h.log.permErrorHandle(Warning, DBErrorLog, err)
			}
		}()

		stackInfo, err := h.bot.GetStackInfo(ctx, stackID)
		if err != nil {
			msg := h.responder.RespondWithError(ev, SystemError)
			_, err = h.teleBot.SendMessage(ctx, msg)
			return err
		}

		err = h.bot.SelectUserStack(ctx, ev.User, stackInfo.ID)
		if err != nil {
			msg := h.responder.RespondWithError(ev, SystemError)
			_, err = h.teleBot.SendMessage(ctx, msg)
			return err
		}

		entries, err := h.bot.GetCurrentStackEntries(ctx, ev.User)
		if err != nil {
			msg := h.responder.RespondWithError(ev, SystemError)
			_, err = h.teleBot.SendMessage(ctx, msg)
			return err
		}
		if len(entries) == 0 || entries == nil {
			msg := h.responder.RespondWithStackIsEmpty(ev, stackInfo)
			_, err = h.teleBot.EditMessageText(ctx, msg)
			h.fsm.SetState(ev.UserID, StateWaitForNewEntryTitle, PendingProlonged)
			return err
		}

		stack := model.NewStack(stackInfo, entries)

		msg := h.responder.RespondWithCurrentStackView(ev, stack)
		_, err = h.teleBot.EditMessageText(ctx, msg)

		return err
	}
}

func (h *APIBotHandler) cancellation() th.CallbackQueryHandler {
	return func(ctx *th.Context, callback telego.CallbackQuery) error {
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
}

// recordModification callback menu:record:new:stackID && State
func (h *APIBotHandler) recordModification(field db.EntryField, fn RespondFuncSend, nextState State) StateStrategy {
	return func(ctx *th.Context, update telego.Update) error {
		ev := model.ExtractEvent(update)
		h.fsm.SetState(ev.UserID, nextState, PendingProlonged)
		entry, exists := h.fsm.GetEntry(ev.UserID)
		if !exists {
			msg := h.responder.RespondWithError(ev, SystemError)
			_, _ = h.teleBot.SendMessage(ctx, msg)
			return fmt.Errorf("no entry found in fsm")
		}
		err := h.bot.UpdateEntry(ctx, entry, field, update.Message.Text) //TODO define how to retrieve an entry id (with fsm?)
		if err != nil {
			msg := h.responder.RespondWithError(ev, SystemError)
			_, _ = h.teleBot.SendMessage(ctx, msg)
			return err
		}
		msg := fn(ev)
		_, err = h.teleBot.SendMessage(ctx, msg)
		return err
	}
}

func (h *APIBotHandler) mainMenu(ctx *th.Context, ev *model.EventContext) error {
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

	msg := h.responder.RespondWithMainMenu(ev, stacks)
	_, err = h.teleBot.SendMessage(ctx.Context(), msg)
	return err
}

func (h *APIBotHandler) stateNone(ctx *th.Context, update telego.Update) error {
	return nil
}

func (h *APIBotHandler) newStackInput(ctx *th.Context, update telego.Update) error {
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
			ev,
			InvalidStackName,
		)
		_, err := h.teleBot.SendMessage(ctx.Context(), msg)
		return err
	}

	// TODO goto stack menu
	return nil
}

func (h *APIBotHandler) joinStackInput(ctx *th.Context, update telego.Update) error {
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
			ev,
			InvalidStackID,
		)
		_, err := h.teleBot.SendMessage(ctx.Context(), msg)
		return err
	}

	err = h.bot.JoinUserStack(ctx.Context(), ev.User, stackID)
	// TODO goto joined stack menu
	return err
}

func (h *APIBotHandler) callbackWithState(state State, respondFn RespondFuncEdit) th.CallbackQueryHandler {
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
