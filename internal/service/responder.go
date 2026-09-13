package service

import (
	"MeowSentinelBot/internal/model"
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type Responder interface {
	RespondRandomly() error
	RespondWithMainMenu(*model.EventContext, []model.Stack) *telego.SendMessageParams
	RespondWithCurrentStackMenu(*model.EventContext, []model.Stack) *telego.SendMessageParams
	RespondWithError(*model.EventContext, ErrorText) *telego.SendMessageParams
	RespondToCancellationCallback(*model.EventContext) *telego.EditMessageTextParams
	RespondToStart(*model.EventContext) *telego.SendMessageParams
	RespondToNewStackCmdCallback(*model.EventContext) *telego.EditMessageTextParams
	RespondToJoinStackCmdCallback(*model.EventContext) *telego.EditMessageTextParams
}

type StdResponder struct {
	sb *strings.Builder
}

func (r *StdResponder) RespondToNewStackCmdCallback(ev *model.EventContext) *telego.EditMessageTextParams {
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Cancel").WithCallbackData("action:deny"),
	))
	editedMsg := tu.EditMessageText(
		ev.ChatID,
		ev.MessageID,
		"**Action chosen: ** create new stack\n\n> Please, enter new stack name:",
	).WithParseMode(telego.ModeMarkdown).WithReplyMarkup(keyboard)
	return editedMsg
}
func (r *StdResponder) RespondToJoinStackCmdCallback(ev *model.EventContext) *telego.EditMessageTextParams {
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Cancel").WithCallbackData("action:deny"),
	))
	editedMsg := tu.EditMessageText(
		ev.ChatID,
		ev.MessageID,
		"**Action chosen: ** join stack\n\n> Please, enter new stack ID\n\n *The stack owner must request an identifier.*",
	).WithParseMode(telego.ModeMarkdown).WithReplyMarkup(keyboard)
	return editedMsg
}

func (r *StdResponder) RespondToCancellationCallback(ev *model.EventContext) *telego.EditMessageTextParams {
	editedText := "Action cancelled"
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Main menu").WithCallbackData("menu:main")),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Go to settings").WithCallbackData("menu:settings")),
	)

	editedMsg := tu.EditMessageText(ev.ChatID, ev.MessageID, editedText).
		WithParseMode(telego.ModeMarkdown).WithReplyMarkup(keyboard)
	return editedMsg
}

func (r *StdResponder) RespondWithCurrentStackMenu(ev *model.EventContext, stacks []model.Stack) *telego.SendMessageParams {
	//TODO implement me
	panic("implement me")
}

func (r *StdResponder) RespondWithError(ev *model.EventContext, err ErrorText) *telego.SendMessageParams {
	//TODO implement me
	panic("implement me")
}

func (r *StdResponder) RespondWithMainMenu(ev *model.EventContext, stacks []model.Stack) *telego.SendMessageParams {
	btnRows := make([][]telego.InlineKeyboardButton, 0, len(stacks)+1)

	for _, stack := range stacks {
		stackBtn := tu.InlineKeyboardButton(fmt.Sprintf("stack: %v", stack.Title)).
			WithCallbackData(fmt.Sprintf("stack:option:%d", stack.ID))
		btnRows = append(btnRows, tu.InlineKeyboardRow(stackBtn))
	}

	btnRows = append(btnRows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("⚙️Manage my stacks").WithCallbackData("settings:stacks"),
	))

	keyboard := tu.InlineKeyboard(btnRows...)

	msgText := tu.Message(ev.ChatID, "main menu").
		WithParseMode(telego.ModeMarkdown).WithReplyMarkup(keyboard)

	return msgText
}

func (r *StdResponder) RespondToStart(ev *model.EventContext) *telego.SendMessageParams {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("New stack").WithCallbackData("stack:new-stack"),
			tu.InlineKeyboardButton("Join stack").WithCallbackData("stack:join-stack"),
		),
	)

	invitationMSG := tu.Message(ev.ChatID, "invitation").
		WithReplyMarkup(keyboard).WithParseMode(telego.ModeMarkdown)

	return invitationMSG
}

func NewStdResponder() *StdResponder {
	return &StdResponder{sb: new(strings.Builder)}
}

func (r *StdResponder) RespondRandomly() error {
	return nil
}

type ErrorText string

const (
	InvalidStackName ErrorText = "Failed to create stack. Make sure you didn't put any shenanigans as a stack name. "
	InvalidStackID   ErrorText = "Failed to recognize stackID. Make sure you have a valid stackID. "
)
