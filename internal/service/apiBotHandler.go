package service

import (
	"MeowSentinelBot/internal/db"
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/utils/cbUtils"
	"MeowSentinelBot/internal/utils/peUtils"
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
	PendingStd       = 20 * time.Second
	PendingProlonged = 2 * time.Minute
)

type StateStrategy func(ctx *th.Context, update telego.Update) error
type InlineEntryChange func(ctx *th.Context, cb telego.CallbackQuery) error
type APIBotHandler struct {
	teleBot   *telego.Bot
	responder Responder
	bot       BotService
	log       Logger
	fsm       *InMemFSMStore
	sr        map[State]StateStrategy
}

func (h *APIBotHandler) StartBot(r *Responder) {
	h.responder = *r

	h.sr = map[State]StateStrategy{
		StateNone:                     h.stateNone,
		StateWaitForNewStackName:      h.newStackInput,
		StateWaitForExistingStackName: h.joinStackInput,
		StateWaitForNewEntryTitle: h.recordModification(
			db.EntryTitleField,
			h.responder.RespondToFieldUpdate,
			StateWaitForNewEntryType,
			newEntryInstructionsType,
		),
		StateWaitForNewEntryType: h.recordModification(
			db.EntryTypeField,
			h.responder.RespondToFieldUpdate,
			StateNone,
		),
		StateWaitForEntryID: h.recordSelection(
			h.responder.RespondWithSelectedEntryMenu,
			StateNone,
		),
		StateWaitForDeletionEntryID: h.recordSelection(
			h.responder.RespondWithSelectedEntryDeletionMenu,
			StateWaitForConfirmation,
		),
	}

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

	// MIDDLEWARE to delete any inline keyboard from the message
	bh.Use(func(ctx *th.Context, update telego.Update) error {
		if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
			msg := update.CallbackQuery.Message
			chatID := msg.GetChat().ChatID()
			msgID := msg.GetMessageID()

			_, editErr := h.teleBot.EditMessageReplyMarkup(ctx, &telego.EditMessageReplyMarkupParams{
				ChatID:      chatID,
				MessageID:   msgID,
				ReplyMarkup: nil,
			})

			if editErr != nil {
				// Log error but consider continuing so you don't break the bot flow
				fmt.Printf("Failed to delete keyboard: %v\n", editErr)
			}
		}
		return ctx.Next(update)
	})
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
	//bh.HandleCallbackQuery(h.stackView(), th.CallbackDataEqual("stack:current"))

	// 			— ENTRIES
	bh.HandleCallbackQuery(
		h.callbackWithState(StateWaitForNewEntryTitle, h.responder.RespondToNewEntryCallback),
		th.CallbackDataPrefix("entry:add:"),
	)
	bh.HandleCallbackQuery(h.entryMenu(), th.CallbackDataPrefix("entry:view:"))
	bh.HandleCallbackQuery(
		h.callbackWithState(StateWaitForEntryID, h.responder.RespondToEntryUpdateCallback),
		th.CallbackDataPrefix("entry:update:"),
	)
	bh.HandleCallbackQuery(
		h.callbackWithState(StateWaitForDeletionEntryID, h.responder.RespondToNewEntryCallback),
		th.CallbackDataPrefix("entry:delete:"),
	)
	bh.HandleCallbackQuery(
		h.callbackWithState(StateNone, h.responder.RespondWithSelEntryRatingMenu),
		th.CallbackDataPrefix("entry:rate:"),
	)

	bh.HandleCallbackQuery(
		th.CallbackQueryHandler(h.callbackEntryUpdateNoState(
			"entry:changeValue:",
			h.bot.DefineAndPerformChange,
			h.responder.RespondWithSelectedEntryMenuEdited,
		)),
		th.CallbackDataPrefix("entry:changeValue:"),
	)

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
		stackID, err := peUtils.GetInfoFromCallbackData[int64]("stack:view:", callback.Data)
		if err != nil {
			stackID, err = h.getCurrStackIDFromFSMorDB(ctx, ev)
			if err != nil {
				return err
			}
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
			msg := h.responder.RespondWithError(ev, CannotGetStackInfo)
			_, err = h.teleBot.SendMessage(ctx, msg)
			return err
		}

		err = h.bot.SelectUserStack(ctx, ev.User, stackInfo.ID)
		if err != nil {
			msg := h.responder.RespondWithError(ev, CannotSelectUserStack)
			_, err = h.teleBot.SendMessage(ctx, msg)
			return err
		}

		entries, err := h.bot.GetCurrentStackEntries(ctx, ev.User)
		if err != nil {
			msg := h.responder.RespondWithError(ev, CannotGetStackEntries)
			_, _ = h.teleBot.SendMessage(ctx, msg)
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
		h.fsm.CleanStack(ev.UserID) //TODO needed?
		h.fsm.CleanEntry(ev.UserID) //TODO needed?

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

// recordModification is used to register any changes provided with user input
func (h *APIBotHandler) recordModification(
	field db.EntryField,
	fn RespondFuncSend,
	nextState State,
	nextPrompt ...UserInputPrompt,
) StateStrategy {
	return func(ctx *th.Context, update telego.Update) error {
		ev := model.ExtractEvent(update)

		entry, exists := h.fsm.GetEntry(ev.UserID)
		if !exists {
			if h.fsm.GetState(ev.UserID) != StateWaitForNewEntryTitle {
				msg := h.responder.RespondWithError(ev, SystemError)
				_, _ = h.teleBot.SendMessage(ctx, msg)
				return fmt.Errorf("no entry found in fsm")
			}
			entry, err := h.bot.AddNewEntry(ctx, ev.User, update.Message.Text)
			if err != nil {
				msg := h.responder.RespondWithError(ev, SystemError)
				_, _ = h.teleBot.SendMessage(ctx, msg)
				return fmt.Errorf("unknown error on creating entry: %w", err)
			}
			h.fsm.SetEntry(ev.UserID, entry, PendingProlonged)
		} else {
			err := h.bot.UpdateEntry(ctx, entry, field, update.Message.Text)
			if err != nil {
				msg := h.responder.RespondWithError(ev, UpdError, InvalidInput)
				_, _ = h.teleBot.SendMessage(ctx, msg)
				return err
			}
		}

		msg := fn(ev, nextPrompt...)
		_, err := h.teleBot.SendMessage(ctx, msg)
		h.fsm.SetState(ev.UserID, nextState, PendingProlonged)
		return err
	}
}

func (h *APIBotHandler) recordSelection(
	fn RespondFuncEntry,
	nextState State,
	_ ...UserInputPrompt, //next prompt (wtf)
) StateStrategy {
	return func(ctx *th.Context, update telego.Update) error {
		ev := model.ExtractEvent(update)
		rawEID := update.Message.Text

		eID, err := peUtils.GetInfoFromPayload[int64](rawEID)
		if err != nil {
			msg := h.responder.RespondWithError(ev, InvalidEntryID)
			_, _ = h.teleBot.SendMessage(ctx, msg)
			return err
		}

		entry, err := h.bot.GetEntryByID(ctx, eID)
		if err != nil {
			msg := h.responder.RespondWithError(ev, InvalidEntryID)
			_, _ = h.teleBot.SendMessage(ctx, msg)
			return err
		}

		msg := fn(ev, entry)
		_, _ = h.teleBot.SendMessage(ctx, msg)
		h.fsm.SetState(ev.UserID, nextState, PendingProlonged)
		h.fsm.SetEntry(ev.UserID, entry.Id, PendingProlonged)
		return nil
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

func (h *APIBotHandler) stackMenu() th.CallbackQueryHandler {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		return nil
	}
}

func (h *APIBotHandler) entryMenu() th.CallbackQueryHandler {
	return func(ctx *th.Context, query telego.CallbackQuery) error {
		ev := model.ExtractEventFromCallback(&query)
		entryID, exists := h.fsm.GetEntry(ev.UserID)
		if !exists {
			h.errBackToMainMenu(ctx, *ev)
			return nil
		}

		entry, err := h.bot.GetEntryByID(ctx, entryID)
		if err != nil {
			h.unknownError(ctx, *ev, entryID, err)
		}

		msg := h.responder.RespondWithSelectedEntryMenu(ev, entry)
		editMsg := cbUtils.ToEditParams(msg, ev.MessageID)
		_, err = h.teleBot.EditMessageText(ctx, editMsg)

		return err
	}
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

// callbackWithState is used to trigger user input prompting with further (FMS) State-driven
// user input determination
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

		editedMsg := respondFn(ev, cb.Message.Message())
		_, err = h.teleBot.EditMessageText(ctx.Context(), editedMsg)
		return err
	}
}

// callbackEntryUpdateNoState is used to trigger DB changes without any user input prompting
func (h *APIBotHandler) callbackEntryUpdateNoState(
	prefix string,
	botFn InlineEntryUpdateFn,
	respFn RespondFuncEditEntry,
) InlineEntryChange {
	return func(ctx *th.Context, cb telego.CallbackQuery) error {
		ev := model.ExtractEventFromCallback(&cb)
		entryID, exists := h.fsm.GetEntry(ev.UserID)
		if !exists {
			h.errBackToMainMenu(ctx, *ev)
			return nil
		}

		rawDBField, err := peUtils.ExtractSegment(
			cb.Data,
			":",
			2,
		)
		if err != nil {
			h.possibleHackAlert(ctx, *ev, cb.Data)
			return err
		}

		var field db.EntryField
		field, err = field.AssertEntryField(rawDBField)
		if err != nil {
			h.possibleHackAlert(ctx, *ev, rawDBField)
			return err
		}

		valPrefix := prefix + rawDBField + ":"
		updValue, err := peUtils.GetInfoFromCallbackData[int64](valPrefix, cb.Data)
		if err != nil {
			h.possibleHackAlert(ctx, *ev, cb.Data)
			return err
		}

		updatedEntry, err := botFn(ctx, ev.UserID, entryID, field, updValue)
		if err != nil {
			h.unknownError(ctx, *ev, updValue, err)
			return err
		}

		msg := respFn(ev, updatedEntry)
		_, err = h.teleBot.EditMessageText(ctx, msg)
		return err
	}
}

func (h *APIBotHandler) getCurrStackIDFromFSMorDB(ctx context.Context, ev *model.EventContext) (int64, error) {
	stackID, exists := h.fsm.GetStack(ev.UserID)
	if !exists {
		var err error
		stackID, err = h.bot.GetSelectedUserStackID(ctx, ev.User)
		if err != nil {
			h.log.permErrorHandle(
				Error,
				InvalidStackIDLog,
				stackID,
				ev.User.ID,
				err,
			)
			return 0, err
		}
	}
	return stackID, nil
}

func (h *APIBotHandler) errBackToMainMenu(ctx *th.Context, ev model.EventContext) {
	msg := h.responder.RespondWithError(&ev, TimeHasExpired)
	_, _ = h.teleBot.SendMessage(ctx.Context(), msg)
}

func (h *APIBotHandler) unknownError(ctx *th.Context, ev model.EventContext, reasonValue any, err error) {
	h.log.permErrorHandle(Error, UnknownError, ev.UserID, reasonValue, err)
	msg := h.responder.RespondWithError(&ev, SystemError)
	_, _ = h.teleBot.SendMessage(ctx.Context(), msg)
}

func (h *APIBotHandler) possibleHackAlert(ctx *th.Context, ev model.EventContext, susPayload string) {
	h.log.permErrorHandle(Attack, PossibleAttack, ev.UserID, susPayload)
	msg := h.responder.RespondWithError(&ev, PossibleHackAttempt)
	_, _ = h.teleBot.SendMessage(ctx.Context(), msg)
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
