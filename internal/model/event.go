package model

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type EventContext struct {
	ChatID     telego.ChatID
	UserID     int64
	User       *telego.User
	MessageID  int
	IsCallback bool
	CallbackID string
}

func ExtractEventFromCallback(cb *telego.CallbackQuery) *EventContext {
	return &EventContext{
		ChatID:     tu.ID(cb.Message.GetChat().ID),
		UserID:     cb.From.ID,
		User:       &cb.From,
		MessageID:  cb.Message.GetMessageID(),
		IsCallback: true,
		CallbackID: cb.ID,
	}
}

func ExtractEvent(update telego.Update) *EventContext {
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		return &EventContext{
			ChatID:     tu.ID(cb.Message.GetChat().ID),
			UserID:     cb.From.ID,
			User:       &cb.From,
			MessageID:  cb.Message.GetMessageID(),
			IsCallback: true,
			CallbackID: cb.ID,
		}
	}
	if update.Message != nil {
		return &EventContext{
			ChatID:     tu.ID(update.Message.Chat.ID),
			UserID:     update.Message.From.ID,
			User:       update.Message.From,
			MessageID:  update.Message.MessageID,
			IsCallback: false,
		}
	}
	return &EventContext{}
}
