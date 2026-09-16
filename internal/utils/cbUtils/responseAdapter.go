package cbUtils

import (
	"github.com/mymmrac/telego"
)

func ToEditParams(msg *telego.SendMessageParams, messageID int) *telego.EditMessageTextParams {
	var inlineMarkup *telego.InlineKeyboardMarkup
	if msg.ReplyMarkup != nil {
		if markup, ok := msg.ReplyMarkup.(*telego.InlineKeyboardMarkup); ok {
			inlineMarkup = markup
		}
	}

	return &telego.EditMessageTextParams{
		ChatID:      msg.ChatID,
		MessageID:   messageID,
		Text:        msg.Text,
		ParseMode:   msg.ParseMode,
		ReplyMarkup: inlineMarkup,
		Entities:    msg.Entities,
	}
}
