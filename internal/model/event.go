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
	ev := &EventContext{
		UserID:     cb.From.ID,
		User:       &cb.From,
		IsCallback: true,
		CallbackID: cb.ID,
	}
	if cb.Message != nil {
		ev.ChatID = tu.ID(cb.Message.GetChat().ID)
		ev.MessageID = cb.Message.GetMessageID()
	}
	return ev
}

func ExtractEvent(update telego.Update) *EventContext {
	if update.CallbackQuery != nil {
		return ExtractEventFromCallback(update.CallbackQuery)
	}
	if update.Message != nil {
		ev := &EventContext{
			ChatID:     tu.ID(update.Message.Chat.ID),
			UserID:     update.Message.From.ID,
			User:       update.Message.From,
			MessageID:  update.Message.MessageID,
			IsCallback: false,
		}
		return ev
	}
	return &EventContext{}
}
