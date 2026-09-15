package service

import (
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/utils/stackUtils"
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type Responder interface {
	RespondRandomly() error
	RespondWithMainMenu(ev *model.EventContext, stackInfos []model.StackInfo) *telego.SendMessageParams
	RespondWithCurrentStackMenu(ev *model.EventContext, stackInfos []model.StackInfo) *telego.SendMessageParams
	RespondWithCurrentStackView(ev *model.EventContext, stack model.Stack) *telego.EditMessageTextParams
	RespondWithError(ev *model.EventContext, errTxt ErrorText, v ...ErrorText) *telego.SendMessageParams
	RespondToCancellationCallback(ev *model.EventContext) *telego.EditMessageTextParams
	RespondToStart(ev *model.EventContext) *telego.SendMessageParams
	RespondToNewStackCmdCallback(ev *model.EventContext) *telego.EditMessageTextParams
	RespondToJoinStackCmdCallback(ev *model.EventContext) *telego.EditMessageTextParams
	RespondWithNewEntryRecordMenu(ev *model.EventContext, si *model.StackInfo) *telego.EditMessageTextParams
	RespondWithStackIsEmpty(ev *model.EventContext, si *model.StackInfo) *telego.EditMessageTextParams
	RespondToFieldUpdate(ev *model.EventContext, prompt ...UserInputPrompt) *telego.SendMessageParams
	RespondToNewEntryCallback(ev *model.EventContext) *telego.EditMessageTextParams
}

type StdResponder struct {
}

func (r StdResponder) RespondWithCurrentStackMenu(ev *model.EventContext, stacks []model.StackInfo) *telego.SendMessageParams {
	//TODO implement me
	panic("implement me")
}

func (r StdResponder) RespondWithCurrentStackView(ev *model.EventContext, stack model.Stack) *telego.EditMessageTextParams {
	typedEntries := stackUtils.SortStack(stack.Entries)
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf(
		string(stackFmt),
		stack.Info.Title,
	))
	for _, t := range typedEntries {
		sb.WriteString(fmt.Sprintf(
			string(typeFmt),
			t[0].Type,
		))
		for _, e := range t {
			sb.WriteString(fmt.Sprint(e))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf(
		"Last updated at %s",
		stack.Info.Updated.Format(string(timeShortFmt)),
	))

	msgTxt := sb.String()

	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✨ Add").WithCallbackData(
				fmt.Sprintf("entry:add:%d", stack.Info.ID),
			),
			tu.InlineKeyboardButton("👀 Update").WithCallbackData(
				fmt.Sprintf("stack:update:%d", stack.Info.ID),
			),
			tu.InlineKeyboardButton("🗑 Delete").WithCallbackData(
				fmt.Sprintf("stack:delete:%d", stack.Info.ID),
			),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔍 Search").WithCallbackData(
				fmt.Sprintf("stack:search:%d", stack.Info.ID),
			),
			tu.InlineKeyboardButton("📒 Filter").WithCallbackData(
				fmt.Sprintf("stack:filter:%d", stack.Info.ID),
			),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ Exit stack").WithCallbackData("menu:main"),
		),
	)

	msg := tu.EditMessageText(
		ev.ChatID,
		ev.MessageID,
		msgTxt,
	).WithParseMode(telego.ModeMarkdown).WithReplyMarkup(keyboard)

	return msg
}

func (r StdResponder) RespondToNewStackCmdCallback(ev *model.EventContext) *telego.EditMessageTextParams {
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
func (r StdResponder) RespondToJoinStackCmdCallback(ev *model.EventContext) *telego.EditMessageTextParams {
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

func (r StdResponder) RespondToCancellationCallback(ev *model.EventContext) *telego.EditMessageTextParams {
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

func (r StdResponder) RespondWithError(ev *model.EventContext, err ErrorText, v ...ErrorText) *telego.SendMessageParams {
	var prompt string
	if v == nil {
		prompt = "*Unfortunately*, we've run into an *issue*: \n" + string(err)
	} else {
		errTxt := fmt.Sprintf(string(err), v)
		prompt = "*Unfortunately*, we've run into an *issue*: \n" + errTxt
	}

	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Back to menu").WithCallbackData("menu:main"),
	))
	msg := tu.Message(ev.ChatID, prompt).WithReplyMarkup(keyboard).WithParseMode(telego.ModeMarkdown)
	return msg
}

func (r StdResponder) RespondToFieldUpdate(ev *model.EventContext, newInputPrompt ...UserInputPrompt) *telego.SendMessageParams {
	baseAnswer := "Recorded 👨‍💻\n"
	var prompt string
	if len(newInputPrompt) > 0 {
		sb := strings.Builder{}
		sb.WriteString(baseAnswer)
		for _, inputPrompt := range newInputPrompt {
			sb.WriteString(string(inputPrompt))
		}
		prompt = sb.String()
	} else {
		prompt = baseAnswer
	}
	msg := tu.Message(ev.ChatID, prompt).WithParseMode(telego.ModeMarkdown)

	var keyboard *telego.InlineKeyboardMarkup
	if len(prompt) == 0 {
		keyboard = tu.InlineKeyboard(tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Back to stack").WithCallbackData("stack:view:"),
			tu.InlineKeyboardButton("Back to menu").WithCallbackData("menu:main"),
		))
		msg = msg.WithReplyMarkup(keyboard)
	} else {
		keyboard = tu.InlineKeyboard(tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("action:deny"),
		))
		msg = msg.WithReplyMarkup(keyboard)
	}
	return msg
}

func (r StdResponder) RespondToNewEntryCallback(ev *model.EventContext) *telego.EditMessageTextParams {
	prompt := string(newEntryInstructionsTitle)
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Cancel").WithCallbackData("action:deny"),
	))

	msg := tu.EditMessageText(ev.ChatID, ev.MessageID, prompt).
		WithReplyMarkup(keyboard).WithParseMode(telego.ModeMarkdown)
	return msg
}
func (r StdResponder) RespondWithStackIsEmpty(ev *model.EventContext, si *model.StackInfo) *telego.EditMessageTextParams {
	prompt := string(stackIsEmpty + newEntryInstructionsTitle)

	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Cancel").WithCallbackData("action:deny"),
	))

	msg := tu.EditMessageText(ev.ChatID, ev.MessageID, prompt).
		WithReplyMarkup(keyboard).WithParseMode(telego.ModeMarkdown)
	return msg
}

func (r StdResponder) RespondWithNewEntryRecordMenu(ev *model.EventContext, si *model.StackInfo) *telego.EditMessageTextParams {
	panic("implement me")
}

func (r StdResponder) RespondWithMainMenu(ev *model.EventContext, stacks []model.StackInfo) *telego.SendMessageParams {
	btnRows := make([][]telego.InlineKeyboardButton, 0, len(stacks)+1)

	for _, stack := range stacks {
		stackBtn := tu.InlineKeyboardButton(fmt.Sprintf("stack: %v", stack.Title)).
			WithCallbackData(fmt.Sprintf("stack:view:%d", stack.ID))
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

func (r StdResponder) RespondToStart(ev *model.EventContext) *telego.SendMessageParams {
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
	return &StdResponder{}
}

func (r StdResponder) RespondRandomly() error {
	return nil
}

type UserInputPrompt string

const (
	stackIsEmpty              UserInputPrompt = "It seems that your stack is empty at this point 🥺\n\nLet's configure your *first record*!✨\n"
	newEntryInstructionsTitle UserInputPrompt = "The very first thing you need to specify would be the *record title* (e.g., Game of Thrones). Feel free to use any aliases so that you could distinguish your records later on (GoT).\n\n*Enter the record name*:"
	newEntryInstructionsType  UserInputPrompt = "The next think you really need is the *record type*.\n\nRecord type — is a custom key you would then be able to filter records by. Think of any type of media (film, series, anime, dorama etc). \n\n*Enter the record type*:"
)

type ResponseFormat string

const (
	stackFmt     ResponseFormat = "**%s** ::\n"
	typeFmt      ResponseFormat = " :: *%s*\n"
	timeShortFmt ResponseFormat = "15:04 01.2.2006"
)

type ErrorText string

const (
	InvalidStackName      ErrorText = "Failed to create stack. Make sure you didn't put any shenanigans as a stack name. "
	InvalidStackID        ErrorText = "Failed to recognize stackID. Make sure you have a valid stackID. "
	StackIsEmpty          ErrorText = "Stack is empty."
	SystemError           ErrorText = "Unknown error."
	UpdError              ErrorText = "Failed to update the entry. %v."
	InvalidInput          ErrorText = "Invalid input."
	CannotGetStackInfo    ErrorText = "Cannot get stack info."
	CannotSelectUserStack ErrorText = "Cannot select user stack."
	CannotGetStackEntries ErrorText = "Cannot get stack entries."
)

func (e ErrorText) String() string {
	return string(e)
}
