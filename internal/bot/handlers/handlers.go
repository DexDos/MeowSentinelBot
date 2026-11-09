package handlers

import (
	"MeowSentinelBot/internal/bot/fsm"
	"MeowSentinelBot/internal/bot/keyboards"
	"MeowSentinelBot/internal/bot/presenters"
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/service"
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var (
	ErrInvalidIndex     = errors.New("invalid record number: must be a positive integer")
	ErrIndexOutOfBounds = errors.New("record number is out of bounds")
	ErrStackEmpty       = errors.New("stack is empty")
)

// ParseLocalIndex parses user input into a 1-based local index.
// It handles inputs such as "1", " 2 ", "#3", "# 4".
func ParseLocalIndex(input string) (int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, ErrInvalidIndex
	}
	trimmed = strings.TrimPrefix(trimmed, "#")
	trimmed = strings.TrimSpace(trimmed)

	idx, err := strconv.Atoi(trimmed)
	if err != nil || idx <= 0 {
		return 0, ErrInvalidIndex
	}
	return idx, nil
}

// ResolveEntryByLocalIndex safely maps a 1-based local index back to the actual model.Entry
// from a stack's slice of entries, with strict Out-of-Bounds validation.
func ResolveEntryByLocalIndex(entries []model.Entry, localIndex int) (*model.Entry, error) {
	if len(entries) == 0 {
		return nil, ErrStackEmpty
	}
	if localIndex < 1 || localIndex > len(entries) {
		return nil, fmt.Errorf("%w: index %d not in range 1..%d", ErrIndexOutOfBounds, localIndex, len(entries))
	}
	return &entries[localIndex-1], nil
}

type BotHandler struct {
	bot         *telego.Bot
	service     service.AppService
	fsm         *fsm.Store
	logger      service.Logger
	botUsername string
}

func NewBotHandler(bot *telego.Bot, svc service.AppService, fsmStore *fsm.Store, logger service.Logger) *BotHandler {
	username := "MeowSentinelBot"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	me, err := bot.GetMe(ctx)
	if err == nil && me != nil && me.Username != "" {
		username = me.Username
	}

	return &BotHandler{
		bot:         bot,
		service:     svc,
		fsm:         fsmStore,
		logger:      logger,
		botUsername: username,
	}
}

func (h *BotHandler) RegisterHandlers(bh *th.BotHandler) {
	// =========================================================================
	// Commands
	// =========================================================================
	bh.Handle(h.handleStartCmd, th.CommandEqual("start"))
	bh.Handle(h.handleMenuCmd, th.CommandEqual("menu"))
	bh.Handle(h.handleCancelCmd, th.CommandEqual("cancel"))

	// =========================================================================
	// Callback Queries: Main Menu & Stack Management
	// =========================================================================
	bh.HandleCallbackQuery(h.handleMainMenuCallback, th.CallbackDataEqual("menu:main"))
	bh.HandleCallbackQuery(h.handleNewStackCallback, th.CallbackDataEqual("stack:new"))
	bh.HandleCallbackQuery(h.handleRedeemCallback, th.CallbackDataEqual("stack:redeem"))
	bh.HandleCallbackQuery(h.handleViewStackCallback, th.CallbackDataPrefix("stack:view:"))
	bh.HandleCallbackQuery(h.handleSettingsStackCallback, th.CallbackDataPrefix("stack:settings:"))
	bh.HandleCallbackQuery(h.handleShareStackCallback, th.CallbackDataPrefix("stack:share:"))
	bh.HandleCallbackQuery(h.handleRenameStackCallback, th.CallbackDataPrefix("stack:rename:"))
	bh.HandleCallbackQuery(h.handleDeleteConfirmStackCallback, th.CallbackDataPrefix("stack:delete_confirm:"))
	bh.HandleCallbackQuery(h.handleDeleteStackCallback, th.CallbackDataPrefix("stack:delete:"))
	bh.HandleCallbackQuery(h.handleLeaveConfirmStackCallback, th.CallbackDataPrefix("stack:leave_confirm:"))
	bh.HandleCallbackQuery(h.handleLeaveStackCallback, th.CallbackDataPrefix("stack:leave:"))

	// =========================================================================
	// Callback Queries: Entry CRUD & Actions
	// =========================================================================
	bh.HandleCallbackQuery(h.handleAddEntryCallback, th.CallbackDataPrefix("entry:add:"))
	bh.HandleCallbackQuery(h.handleTypeChoiceCallback, th.CallbackDataPrefix("entry:type_choice:"))
	bh.HandleCallbackQuery(h.handleViewEntryCallback, th.CallbackDataPrefix("entry:view:"))
	bh.HandleCallbackQuery(h.handleStepEntryCallback, th.CallbackDataPrefix("entry:step:"))
	bh.HandleCallbackQuery(h.handleRateMenuEntryCallback, th.CallbackDataPrefix("entry:rate_menu:"))
	bh.HandleCallbackQuery(h.handleRateEntryCallback, th.CallbackDataPrefix("entry:rate:"))
	bh.HandleCallbackQuery(h.handleEditTitleEntryCallback, th.CallbackDataPrefix("entry:edit_title:"))
	bh.HandleCallbackQuery(h.handleEditTypeEntryCallback, th.CallbackDataPrefix("entry:edit_type:"))
	bh.HandleCallbackQuery(h.handleEditProgressEntryCallback, th.CallbackDataPrefix("entry:edit_progress:"))
	bh.HandleCallbackQuery(h.handleDeleteConfirmEntryCallback, th.CallbackDataPrefix("entry:delete_confirm:"))
	bh.HandleCallbackQuery(h.handleDeleteEntryCallback, th.CallbackDataPrefix("entry:delete:"))

	// =========================================================================
	// Technical / Cancellation Callback
	// =========================================================================
	bh.HandleCallbackQuery(h.handleCancelCallback, th.CallbackDataEqual("action:cancel"))

	// =========================================================================
	// State-Driven Text Messages
	// =========================================================================
	bh.Handle(h.handleTextMessage, th.AnyMessageWithText())
}

// =============================================================================
// Command Handlers
// =============================================================================

func (h *BotHandler) handleStartCmd(ctx *th.Context, update telego.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	userID := msg.From.ID
	h.fsm.Clear(userID)
	h.ensureUser(ctx.Context(), msg.From)

	// Check for deep-linking: /start <CODE>
	payload := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/start"))
	if payload != "" {
		stack, err := h.service.RedeemInvite(ctx.Context(), userID, payload)
		if err == nil {
			_ = h.service.SelectStack(ctx.Context(), userID, stack.ID)
			entries, _ := h.service.GetStackEntries(ctx.Context(), userID, stack.ID)
			escapedTitle := html.EscapeString(stack.Title)
			text := fmt.Sprintf("🎉 <b>Welcome! You joined \"%s\"!</b>\n\n%s", escapedTitle, presenters.RenderStackView(stack, entries))
			_, sendErr := h.bot.SendMessage(ctx, tu.Message(msg.Chat.ChatID(), text).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.StackViewKeyboard(stack.ID, entries)))
			return sendErr
		} else if errors.Is(err, service.ErrAlreadyMember) {
			_ = h.service.SelectStack(ctx.Context(), userID, stack.ID)
			entries, _ := h.service.GetStackEntries(ctx.Context(), userID, stack.ID)
			escapedTitle := html.EscapeString(stack.Title)
			text := fmt.Sprintf("ℹ️ <b>You are already a member of \"%s\"!</b>\n\n%s", escapedTitle, presenters.RenderStackView(stack, entries))
			_, sendErr := h.bot.SendMessage(ctx, tu.Message(msg.Chat.ChatID(), text).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.StackViewKeyboard(stack.ID, entries)))
			return sendErr
		} else {
			text := presenters.RenderError("The invite link is invalid or has expired.")
			_, sendErr := h.bot.SendMessage(ctx, tu.Message(msg.Chat.ChatID(), text).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return sendErr
		}
	}

	return h.sendMainMenu(ctx, msg.Chat.ChatID())
}

func (h *BotHandler) handleMenuCmd(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}
	h.fsm.Clear(update.Message.From.ID)
	h.ensureUser(ctx.Context(), update.Message.From)
	return h.sendMainMenu(ctx, update.Message.Chat.ChatID())
}

func (h *BotHandler) handleCancelCmd(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}
	h.fsm.Clear(update.Message.From.ID)
	_, _ = h.bot.SendMessage(ctx, tu.Message(update.Message.Chat.ChatID(), "Action cancelled."))
	return h.sendMainMenu(ctx, update.Message.Chat.ChatID())
}

// =============================================================================
// Main Menu & Stack Management Callbacks
// =============================================================================

func (h *BotHandler) handleMainMenuCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	h.fsm.Clear(query.From.ID)
	h.ensureUser(ctx.Context(), &query.From)

	stacks, err := h.service.GetUserStacks(ctx.Context(), query.From.ID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Failed to fetch stacks.")
	}

	text := presenters.RenderMainMenu(stacks)
	kb := keyboards.MainMenuKeyboard(stacks)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleNewStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	h.fsm.Set(query.From.ID, fsm.StateWaitForStackTitle, 5*time.Minute)

	text := presenters.RenderPrompt("Create New Stack", "Please enter the title for your new stack (e.g. 'Weekend Watchlist', 'Anime 2026'):")
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, keyboards.CancelKeyboard())
}

func (h *BotHandler) handleRedeemCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	h.fsm.Set(query.From.ID, fsm.StateWaitForRedeemCode, 5*time.Minute)

	text := presenters.RenderPrompt("Redeem Invite Code", "Please enter the invite code you received (e.g. MEOW-7X9K):")
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, keyboards.CancelKeyboard())
}

func (h *BotHandler) handleViewStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	h.fsm.Clear(query.From.ID)

	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:view:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	stack, err := h.service.GetStack(ctx.Context(), query.From.ID, stackID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Stack not found or access denied.")
	}

	_ = h.service.SelectStack(ctx.Context(), query.From.ID, stackID)
	entries, err := h.service.GetStackEntries(ctx.Context(), query.From.ID, stackID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Failed to load stack entries.")
	}

	text := presenters.RenderStackView(stack, entries)
	kb := keyboards.StackViewKeyboard(stack.ID, entries)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleSettingsStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	h.fsm.Clear(query.From.ID)

	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:settings:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	stack, err := h.service.GetStack(ctx.Context(), query.From.ID, stackID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Stack not found or access denied.")
	}

	text := presenters.RenderStackSettings(stack)
	kb := keyboards.StackSettingsKeyboard(stack)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleShareStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:share:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	inv, err := h.service.CreateInvite(ctx.Context(), query.From.ID, stackID, 24*time.Hour)
	if err != nil {
		if errors.Is(err, service.ErrNotOwner) {
			return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Only the stack owner can generate invite codes.")
		}
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Failed to create invite.")
	}

	text := presenters.RenderInviteShare(inv, h.botUsername)
	kb := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("⬅️ Back to Settings").WithCallbackData(fmt.Sprintf("stack:settings:%d", stackID)),
	))
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleRenameStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:rename:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	h.fsm.SetWithData(query.From.ID, fsm.StateWaitForRenameTitle, map[string]any{"stackID": stackID}, 5*time.Minute)
	text := presenters.RenderPrompt("Rename Stack", "Please enter the new title for this stack:")
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, keyboards.CancelKeyboard())
}

func (h *BotHandler) handleDeleteConfirmStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:delete_confirm:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	text := "⚠️ <b>Are you sure you want to delete this stack?</b>\n\nAll entries and ratings inside it will be permanently lost for all members."
	kb := keyboards.ConfirmStackDeleteKeyboard(stackID)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleDeleteStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID, "Deleting stack...")
	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:delete:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	if err := h.service.DeleteStack(ctx.Context(), query.From.ID, stackID); err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, query.From.ID, "delete_stack", err)
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), h.sanitizeError(err, "Failed to delete stack. Please make sure you are the owner."))
	}

	return h.handleMainMenuCallback(ctx, query)
}

func (h *BotHandler) handleLeaveConfirmStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:leave_confirm:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	text := "🚪 <b>Are you sure you want to leave this stack?</b>\n\nYou will need a new invite code from the owner to rejoin."
	kb := keyboards.ConfirmLeaveStackKeyboard(stackID)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleLeaveStackCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID, "Leaving stack...")
	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "stack:leave:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	if err := h.service.LeaveStack(ctx.Context(), query.From.ID, stackID); err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, query.From.ID, "leave_stack", err)
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), h.sanitizeError(err, "Failed to leave stack."))
	}

	return h.handleMainMenuCallback(ctx, query)
}

// =============================================================================
// Entry CRUD & Actions Callbacks
// =============================================================================

func (h *BotHandler) handleAddEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	stackID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:add:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid stack ID.")
	}

	h.fsm.SetWithData(query.From.ID, fsm.StateWaitForEntryTitle, map[string]any{"stackID": stackID}, 5*time.Minute)
	text := presenters.RenderPrompt("Add New Media (Step 1 of 2)", "Please enter the title of the movie or series:")
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, keyboards.CancelKeyboard())
}

func (h *BotHandler) handleTypeChoiceCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	mediaType := strings.TrimPrefix(query.Data, "entry:type_choice:")

	state, data, ok := h.fsm.Get(query.From.ID)
	if !ok || state != fsm.StateWaitForEntryType {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Session expired. Please try adding the entry again.")
	}

	stackID, _ := data["stackID"].(int64)
	title, _ := data["title"].(string)
	h.fsm.Clear(query.From.ID)

	entry, err := h.service.AddEntry(ctx.Context(), query.From.ID, stackID, title, mediaType, 1, 1)
	if err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, query.From.ID, "add_entry", err)
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), h.sanitizeError(err, "Failed to add entry. Please try again."))
	}

	userScore, _ := h.service.GetUserRating(ctx.Context(), query.From.ID, entry.ID)
	localIdx := h.findEntryLocalIndex(ctx.Context(), query.From.ID, entry.StackID, entry.ID)
	text := presenters.RenderEntryDetail(entry, userScore, localIdx)
	kb := keyboards.EntryDetailKeyboard(entry)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleViewEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	h.fsm.Clear(query.From.ID)

	entryID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:view:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid entry ID.")
	}

	entry, err := h.service.GetEntry(ctx.Context(), query.From.ID, entryID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Entry not found or access denied.")
	}

	userScore, _ := h.service.GetUserRating(ctx.Context(), query.From.ID, entryID)
	localIdx := h.findEntryLocalIndex(ctx.Context(), query.From.ID, entry.StackID, entry.ID)
	text := presenters.RenderEntryDetail(entry, userScore, localIdx)
	kb := keyboards.EntryDetailKeyboard(entry)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleStepEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	// Format: entry:step:<field>:<delta>:<entryID>
	parts := strings.Split(query.Data, ":")
	if len(parts) != 5 {
		h.answerCallback(ctx, query.ID, "Invalid action")
		return nil
	}

	field := parts[2]
	delta, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		h.answerCallback(ctx, query.ID, "Invalid step")
		return nil
	}

	entryID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		h.answerCallback(ctx, query.ID, "Invalid entry")
		return nil
	}

	h.answerCallback(ctx, query.ID) // Quick acknowledge

	var seasonDelta, episodeDelta int64
	if field == "season" {
		seasonDelta = delta
	} else if field == "episode" {
		episodeDelta = delta
	}

	updated, err := h.service.AdjustEntryProgress(ctx.Context(), query.From.ID, entryID, seasonDelta, episodeDelta)
	if err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, query.From.ID, "adjust_progress", err)
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), h.sanitizeError(err, "Failed to update watching progress."))
	}

	userScore, _ := h.service.GetUserRating(ctx.Context(), query.From.ID, entryID)
	localIdx := h.findEntryLocalIndex(ctx.Context(), query.From.ID, updated.StackID, updated.ID)
	text := presenters.RenderEntryDetail(updated, userScore, localIdx)
	kb := keyboards.EntryDetailKeyboard(updated)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleRateMenuEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	entryID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:rate_menu:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid entry ID.")
	}

	entry, err := h.service.GetEntry(ctx.Context(), query.From.ID, entryID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Entry not found.")
	}

	userScore, _ := h.service.GetUserRating(ctx.Context(), query.From.ID, entryID)
	localIdx := h.findEntryLocalIndex(ctx.Context(), query.From.ID, entry.StackID, entry.ID)
	text := fmt.Sprintf("%s\n\n⭐️ <b>Choose your rating below:</b>", presenters.RenderEntryDetail(entry, userScore, localIdx))
	kb := keyboards.RatingPickerKeyboard(entryID)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleRateEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	// Format: entry:rate:<entryID>:<score>
	parts := strings.Split(query.Data, ":")
	if len(parts) != 4 {
		h.answerCallback(ctx, query.ID, "Invalid rating")
		return nil
	}

	entryID, _ := strconv.ParseInt(parts[2], 10, 64)
	scoreVal, _ := strconv.ParseInt(parts[3], 10, 64)
	score := model.ScoreFromInt(scoreVal)

	h.answerCallback(ctx, query.ID, fmt.Sprintf("Rated %s", score.Stars()))

	updated, err := h.service.RateEntry(ctx.Context(), query.From.ID, entryID, score)
	if err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, query.From.ID, "rate_entry", err)
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), h.sanitizeError(err, "Failed to submit rating."))
	}

	localIdx := h.findEntryLocalIndex(ctx.Context(), query.From.ID, updated.StackID, updated.ID)
	text := presenters.RenderEntryDetail(updated, score, localIdx)
	kb := keyboards.EntryDetailKeyboard(updated)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleEditTitleEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	entryID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:edit_title:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid entry ID.")
	}

	h.fsm.SetWithData(query.From.ID, fsm.StateWaitForEditTitle, map[string]any{"entryID": entryID}, 5*time.Minute)
	text := presenters.RenderPrompt("Edit Title", "Please enter the new title for this media item:")
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, keyboards.CancelKeyboard())
}

func (h *BotHandler) handleEditTypeEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	entryID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:edit_type:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid entry ID.")
	}

	h.fsm.SetWithData(query.From.ID, fsm.StateWaitForEditType, map[string]any{"entryID": entryID}, 5*time.Minute)
	text := presenters.RenderPrompt("Edit Type", "Please enter the new media type (e.g. Movie, TV Series, Anime):")
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, keyboards.CancelKeyboard())
}

func (h *BotHandler) handleEditProgressEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	entryID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:edit_progress:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid entry ID.")
	}

	h.fsm.SetWithData(query.From.ID, fsm.StateWaitForEditProgress, map[string]any{"entryID": entryID}, 5*time.Minute)
	text := presenters.RenderPrompt("Set Progress", "Enter season and episode (e.g. '2 5' or 'S2 E5'):")
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, keyboards.CancelKeyboard())
}

func (h *BotHandler) handleDeleteConfirmEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID)
	entryID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:delete_confirm:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid entry ID.")
	}

	entry, err := h.service.GetEntry(ctx.Context(), query.From.ID, entryID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Entry not found.")
	}

	escapedTitle := html.EscapeString(entry.Title)
	text := fmt.Sprintf("🗑 <b>Delete \"%s\"?</b>\n\nAre you sure you want to remove this entry?", escapedTitle)
	kb := keyboards.ConfirmEntryDeleteKeyboard(entry)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleDeleteEntryCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID, "Entry deleted")
	entryID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, "entry:delete:"), 10, 64)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Invalid entry ID.")
	}

	entry, err := h.service.GetEntry(ctx.Context(), query.From.ID, entryID)
	if err != nil {
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), "Entry not found.")
	}

	stackID := entry.StackID
	if err := h.service.DeleteEntry(ctx.Context(), query.From.ID, entryID); err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, query.From.ID, "delete_entry", err)
		return h.sendError(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), h.sanitizeError(err, "Failed to delete entry."))
	}

	// Return to stack view with cleanly re-indexed entries
	stack, _ := h.service.GetStack(ctx.Context(), query.From.ID, stackID)
	entries, _ := h.service.GetStackEntries(ctx.Context(), query.From.ID, stackID)
	text := presenters.RenderStackView(stack, entries)
	kb := keyboards.StackViewKeyboard(stackID, entries)
	return h.editOrSend(ctx, query.Message.GetChat().ChatID(), query.Message.GetMessageID(), text, kb)
}

func (h *BotHandler) handleCancelCallback(ctx *th.Context, query telego.CallbackQuery) error {
	h.answerCallback(ctx, query.ID, "Cancelled")
	h.fsm.Clear(query.From.ID)
	return h.handleMainMenuCallback(ctx, query)
}

// =============================================================================
// State-Driven Text Message Handler
// =============================================================================

func (h *BotHandler) handleTextMessage(ctx *th.Context, update telego.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	userID := msg.From.ID
	userInput := strings.TrimSpace(msg.Text)
	chatID := msg.Chat.ChatID()

	state, data, ok := h.fsm.Get(userID)
	if !ok || state == fsm.StateNone {
		// If user typed a record index (e.g., "1" or "#2"), check if they have a currently selected stack
		if localIdx, err := ParseLocalIndex(userInput); err == nil {
			stackID, err := h.service.GetSelectedStackID(ctx.Context(), userID)
			if err == nil && stackID > 0 {
				return h.handleSelectEntryByLocalIndex(ctx, chatID, userID, stackID, localIdx)
			}
		}

		// No active conversational state; prompt to use /menu
		text := "🐱 Send /menu to view your stacks or /start to begin!"
		_, err := h.bot.SendMessage(ctx, tu.Message(chatID, text))
		return err
	}

	switch state {
	case fsm.StateWaitForSelectEntry:
		h.fsm.Clear(userID)
		stackID, _ := data["stackID"].(int64)
		localIdx, err := ParseLocalIndex(userInput)
		if err != nil {
			userMsg := "Please enter a valid positive number."
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}
		return h.handleSelectEntryByLocalIndex(ctx, chatID, userID, stackID, localIdx)

	case fsm.StateWaitForStackTitle:
		h.fsm.Clear(userID)
		stack, err := h.service.CreateStack(ctx.Context(), userID, userInput)
		if err != nil {
			h.logger.LogEvent(service.Error, service.UnknownError, userID, "create_stack", err)
			userMsg := h.sanitizeError(err, "Failed to create stack. Please try again.")
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}

		_ = h.service.SelectStack(ctx.Context(), userID, stack.ID)
		entries, _ := h.service.GetStackEntries(ctx.Context(), userID, stack.ID)
		text := fmt.Sprintf("✨ <b>Stack created!</b>\n\n%s", presenters.RenderStackView(stack, entries))
		_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.StackViewKeyboard(stack.ID, entries)))
		return err

	case fsm.StateWaitForRenameTitle:
		h.fsm.Clear(userID)
		stackID, _ := data["stackID"].(int64)
		if err := h.service.UpdateStackTitle(ctx.Context(), userID, stackID, userInput); err != nil {
			h.logger.LogEvent(service.Error, service.UnknownError, userID, "rename_stack", err)
			userMsg := h.sanitizeError(err, "Failed to rename stack. Please try again.")
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}

		stack, _ := h.service.GetStack(ctx.Context(), userID, stackID)
		text := fmt.Sprintf("✅ <b>Stack renamed!</b>\n\n%s", presenters.RenderStackSettings(stack))
		_, err := h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.StackSettingsKeyboard(stack)))
		return err

	case fsm.StateWaitForRedeemCode:
		h.fsm.Clear(userID)
		stack, err := h.service.RedeemInvite(ctx.Context(), userID, userInput)
		if err != nil {
			if errors.Is(err, service.ErrAlreadyMember) {
				_ = h.service.SelectStack(ctx.Context(), userID, stack.ID)
				entries, _ := h.service.GetStackEntries(ctx.Context(), userID, stack.ID)
				escapedStackTitle := html.EscapeString(stack.Title)
				text := fmt.Sprintf("ℹ️ <b>You are already in \"%s\"!</b>\n\n%s", escapedStackTitle, presenters.RenderStackView(stack, entries))
				_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
					WithParseMode(telego.ModeHTML).
					WithReplyMarkup(keyboards.StackViewKeyboard(stack.ID, entries)))
				return err
			}
			h.logger.LogEvent(service.Error, service.UnknownError, userID, "redeem_code", err)
			userMsg := h.sanitizeError(err, "Invalid or expired invite code.")
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}

		_ = h.service.SelectStack(ctx.Context(), userID, stack.ID)
		entries, _ := h.service.GetStackEntries(ctx.Context(), userID, stack.ID)
		escapedStackTitle := html.EscapeString(stack.Title)
		text := fmt.Sprintf("🎉 <b>Success! You joined \"%s\"!</b>\n\n%s", escapedStackTitle, presenters.RenderStackView(stack, entries))
		_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.StackViewKeyboard(stack.ID, entries)))
		return err

	case fsm.StateWaitForEntryTitle:
		stackID, _ := data["stackID"].(int64)
		// Save title and prompt for type (Step 2)
		h.fsm.SetWithData(userID, fsm.StateWaitForEntryType, map[string]any{
			"stackID": stackID,
			"title":   userInput,
		}, 5*time.Minute)

		escapedInput := html.EscapeString(userInput)
		text := fmt.Sprintf("Title recorded: <b>%s</b>\n\nNow select or type the <b>Media Type</b>:", escapedInput)
		_, err := h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.MediaTypeKeyboard()))
		return err

	case fsm.StateWaitForEntryType:
		stackID, _ := data["stackID"].(int64)
		title, _ := data["title"].(string)
		h.fsm.Clear(userID)

		entry, err := h.service.AddEntry(ctx.Context(), userID, stackID, title, userInput, 1, 1)
		if err != nil {
			h.logger.LogEvent(service.Error, service.UnknownError, userID, "add_entry", err)
			userMsg := h.sanitizeError(err, "Failed to add entry. Please try again.")
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}

		userScore, _ := h.service.GetUserRating(ctx.Context(), userID, entry.ID)
		localIdx := h.findEntryLocalIndex(ctx.Context(), userID, entry.StackID, entry.ID)
		text := fmt.Sprintf("✨ <b>Added to stack!</b>\n\n%s", presenters.RenderEntryDetail(entry, userScore, localIdx))
		_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.EntryDetailKeyboard(entry)))
		return err

	case fsm.StateWaitForEditTitle:
		h.fsm.Clear(userID)
		entryID, _ := data["entryID"].(int64)

		updated, err := h.service.UpdateEntryTitle(ctx.Context(), userID, entryID, userInput)
		if err != nil {
			h.logger.LogEvent(service.Error, service.UnknownError, userID, "edit_entry_title", err)
			userMsg := h.sanitizeError(err, "Failed to update title. Please try again.")
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}

		userScore, _ := h.service.GetUserRating(ctx.Context(), userID, entryID)
		localIdx := h.findEntryLocalIndex(ctx.Context(), userID, updated.StackID, updated.ID)
		text := fmt.Sprintf("✅ <b>Title updated!</b>\n\n%s", presenters.RenderEntryDetail(updated, userScore, localIdx))
		_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.EntryDetailKeyboard(updated)))
		return err

	case fsm.StateWaitForEditType:
		h.fsm.Clear(userID)
		entryID, _ := data["entryID"].(int64)

		updated, err := h.service.UpdateEntryType(ctx.Context(), userID, entryID, userInput)
		if err != nil {
			h.logger.LogEvent(service.Error, service.UnknownError, userID, "edit_entry_type", err)
			userMsg := h.sanitizeError(err, "Failed to update type. Please try again.")
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}

		userScore, _ := h.service.GetUserRating(ctx.Context(), userID, entryID)
		localIdx := h.findEntryLocalIndex(ctx.Context(), userID, updated.StackID, updated.ID)
		text := fmt.Sprintf("✅ <b>Type updated!</b>\n\n%s", presenters.RenderEntryDetail(updated, userScore, localIdx))
		_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.EntryDetailKeyboard(updated)))
		return err

	case fsm.StateWaitForEditProgress:
		h.fsm.Clear(userID)
		entryID, _ := data["entryID"].(int64)

		season, episode := parseProgress(userInput)
		updated, err := h.service.SetEntryProgress(ctx.Context(), userID, entryID, season, episode)
		if err != nil {
			h.logger.LogEvent(service.Error, service.UnknownError, userID, "edit_entry_progress", err)
			userMsg := h.sanitizeError(err, "Failed to update watching progress. Please try again.")
			_, err = h.bot.SendMessage(ctx, tu.Message(chatID, presenters.RenderError(userMsg)).
				WithParseMode(telego.ModeHTML).
				WithReplyMarkup(keyboards.BackToMenuKeyboard()))
			return err
		}

		userScore, _ := h.service.GetUserRating(ctx.Context(), userID, entryID)
		localIdx := h.findEntryLocalIndex(ctx.Context(), userID, updated.StackID, updated.ID)
		text := fmt.Sprintf("✅ <b>Progress updated!</b>\n\n%s", presenters.RenderEntryDetail(updated, userScore, localIdx))
		_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
			WithParseMode(telego.ModeHTML).
			WithReplyMarkup(keyboards.EntryDetailKeyboard(updated)))
		return err
	}

	return nil
}

// =============================================================================
// Internal Helpers
// =============================================================================

func (h *BotHandler) findEntryLocalIndex(ctx context.Context, userID, stackID, entryID int64) int {
	entries, err := h.service.GetStackEntries(ctx, userID, stackID)
	if err != nil {
		return 0
	}
	for i, e := range entries {
		if e.ID == entryID {
			return i + 1
		}
	}
	return 0
}

func (h *BotHandler) handleSelectEntryByLocalIndex(ctx *th.Context, chatID telego.ChatID, userID int64, stackID int64, localIndex int) error {
	entries, err := h.service.GetStackEntries(ctx.Context(), userID, stackID)
	if err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, userID, "select_by_index_get_entries", err)
		return h.sendError(ctx, chatID, 0, h.sanitizeError(err, "Failed to load stack entries."))
	}

	selectedEntry, err := ResolveEntryByLocalIndex(entries, localIndex)
	if err != nil {
		if errors.Is(err, ErrStackEmpty) {
			return h.sendError(ctx, chatID, 0, "This stack is currently empty! Click ✨ Add Media to add an entry.")
		}
		errMsg := fmt.Sprintf("Invalid entry number #%d. Please choose a number between 1 and %d.", localIndex, len(entries))
		return h.sendError(ctx, chatID, 0, errMsg)
	}

	userScore, _ := h.service.GetUserRating(ctx.Context(), userID, selectedEntry.ID)
	text := presenters.RenderEntryDetail(selectedEntry, userScore, localIndex)
	kb := keyboards.EntryDetailKeyboard(selectedEntry)

	_, sendErr := h.bot.SendMessage(ctx, tu.Message(chatID, text).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(kb))
	return sendErr
}

func (h *BotHandler) sendMainMenu(ctx *th.Context, chatID telego.ChatID) error {
	userID := chatID.ID
	stacks, err := h.service.GetUserStacks(ctx.Context(), userID)
	if err != nil {
		h.logger.LogEvent(service.Error, service.UnknownError, userID, "main_menu", err)
	}

	text := presenters.RenderMainMenu(stacks)
	kb := keyboards.MainMenuKeyboard(stacks)
	_, err = h.bot.SendMessage(ctx, tu.Message(chatID, text).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(kb))
	return err
}

func (h *BotHandler) answerCallback(ctx *th.Context, callbackID string, text ...string) {
	cb := tu.CallbackQuery(callbackID)
	if len(text) > 0 && text[0] != "" {
		cb = cb.WithText(text[0])
	}
	_ = h.bot.AnswerCallbackQuery(ctx.Context(), cb)
}

func (h *BotHandler) editOrSend(ctx *th.Context, chatID telego.ChatID, messageID int, text string, kb *telego.InlineKeyboardMarkup) error {
	if messageID != 0 {
		editParams := &telego.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ParseMode:   telego.ModeHTML,
			ReplyMarkup: kb,
		}
		if _, err := h.bot.EditMessageText(ctx, editParams); err == nil {
			return nil
		}
	}

	_, err := h.bot.SendMessage(ctx, tu.Message(chatID, text).
		WithParseMode(telego.ModeHTML).
		WithReplyMarkup(kb))
	return err
}

func (h *BotHandler) sendError(ctx *th.Context, chatID telego.ChatID, messageID int, errText string) error {
	text := presenters.RenderError(errText)
	kb := keyboards.BackToMenuKeyboard()
	return h.editOrSend(ctx, chatID, messageID, text, kb)
}

func (h *BotHandler) ensureUser(ctx context.Context, u *telego.User) {
	if u == nil {
		return
	}
	_ = h.service.EnsureUser(ctx, model.User{
		TelegramID: u.ID,
		Username:   u.Username,
		FirstName:  u.FirstName,
	})
}

// parseProgress extracts season and episode from user inputs like "2 5", "s2 e5", "S02E05"
func parseProgress(input string) (int64, int64) {
	input = strings.TrimSpace(input)

	// Try S02E05 or S2 E5
	re := regexp.MustCompile(`(?i)s(?:eason)?\s*(\d+)\s*e(?:pisode)?\s*(\d+)`)
	matches := re.FindStringSubmatch(input)
	if len(matches) == 3 {
		s, _ := strconv.ParseInt(matches[1], 10, 64)
		e, _ := strconv.ParseInt(matches[2], 10, 64)
		return s, e
	}

	// Try "2 5" or "2, 5"
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ' ' || r == ',' || r == '/' || r == '-' || r == ':'
	})
	if len(parts) >= 2 {
		s, err1 := strconv.ParseInt(parts[0], 10, 64)
		e, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 == nil && err2 == nil && s > 0 && e > 0 {
			return s, e
		}
	} else if len(parts) == 1 {
		// Single number: treat as episode, season = 1
		e, err := strconv.ParseInt(parts[0], 10, 64)
		if err == nil && e > 0 {
			return 1, e
		}
	}

	return 1, 1
}

func (h *BotHandler) sanitizeError(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	switch {
	case errors.Is(err, service.ErrEmptyTitle):
		return "Title cannot be empty."
	case errors.Is(err, service.ErrTitleTooLong):
		return "Title is too long (maximum 128 characters)."
	case errors.Is(err, service.ErrNotOwner):
		return "Only the stack owner can perform this action."
	case errors.Is(err, service.ErrNotMember):
		return "You do not have access to this stack."
	case errors.Is(err, service.ErrStackNotFound):
		return "Stack not found."
	case errors.Is(err, service.ErrEntryNotFound):
		return "Media entry not found."
	case errors.Is(err, service.ErrInvalidCode):
		return "Invalid or expired invite code."
	case errors.Is(err, service.ErrAlreadyMember):
		return "You are already a member of this stack."
	default:
		// Safe fallback: never leak raw internal or SQL error details to end users
		return fallback
	}
}
