package keyboards

import (
	"MeowSentinelBot/internal/model"
	"fmt"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// MainMenuKeyboard builds the main menu keyboard with user's stacks
func MainMenuKeyboard(stacks []model.StackInfo) *telego.InlineKeyboardMarkup {
	var rows [][]telego.InlineKeyboardButton

	for _, s := range stacks {
		prefix := "👥"
		if s.Role == model.RoleOwner {
			prefix = "👑"
		}
		btnText := fmt.Sprintf("%s %s (%d)", prefix, s.Title, s.EntriesCount)
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(btnText).WithCallbackData(fmt.Sprintf("stack:view:%d", s.ID)),
		))
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("✨ New Stack").WithCallbackData("stack:new"),
		tu.InlineKeyboardButton("🎟 Redeem Code").WithCallbackData("stack:redeem"),
	))

	return tu.InlineKeyboard(rows...)
}

// StackViewKeyboard builds keyboard for inside a stack
func StackViewKeyboard(stackID int64, entries []model.Entry) *telego.InlineKeyboardMarkup {
	var rows [][]telego.InlineKeyboardButton

	// Action buttons
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("✨ Add Media").WithCallbackData(fmt.Sprintf("entry:add:%d", stackID)),
		tu.InlineKeyboardButton("⚙️ Settings").WithCallbackData(fmt.Sprintf("stack:settings:%d", stackID)),
	))

	// Direct pickers for entries (up to 4 per row)
	if len(entries) > 0 {
		var entryBtns []telego.InlineKeyboardButton
		for _, e := range entries {
			entryBtns = append(entryBtns, tu.InlineKeyboardButton(fmt.Sprintf("#%d", e.ID)).
				WithCallbackData(fmt.Sprintf("entry:view:%d", e.ID)))
			if len(entryBtns) == 4 {
				rows = append(rows, tu.InlineKeyboardRow(entryBtns...))
				entryBtns = nil
			}
		}
		if len(entryBtns) > 0 {
			rows = append(rows, tu.InlineKeyboardRow(entryBtns...))
		}
	}

	// Navigation
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("⬅️ All Stacks").WithCallbackData("menu:main"),
	))

	return tu.InlineKeyboard(rows...)
}

// StackSettingsKeyboard builds settings keyboard depending on owner/member role
func StackSettingsKeyboard(stack *model.StackInfo) *telego.InlineKeyboardMarkup {
	var rows [][]telego.InlineKeyboardButton

	if stack.Role == model.RoleOwner {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔗 Share Temporary Code").WithCallbackData(fmt.Sprintf("stack:share:%d", stack.ID)),
		))
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✏️ Rename Stack").WithCallbackData(fmt.Sprintf("stack:rename:%d", stack.ID)),
			tu.InlineKeyboardButton("🗑 Delete Stack").WithCallbackData(fmt.Sprintf("stack:delete_confirm:%d", stack.ID)),
		))
	} else {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🚪 Leave Stack").WithCallbackData(fmt.Sprintf("stack:leave_confirm:%d", stack.ID)),
		))
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("⬅️ Back to Stack").WithCallbackData(fmt.Sprintf("stack:view:%d", stack.ID)),
	))

	return tu.InlineKeyboard(rows...)
}

// ConfirmStackDeleteKeyboard confirms permanent stack deletion
func ConfirmStackDeleteKeyboard(stackID int64) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⚠️ Yes, Delete Permanently").WithCallbackData(fmt.Sprintf("stack:delete:%d", stackID)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData(fmt.Sprintf("stack:settings:%d", stackID)),
		),
	)
}

// ConfirmLeaveStackKeyboard confirms leaving shared stack
func ConfirmLeaveStackKeyboard(stackID int64) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Yes, Leave Stack").WithCallbackData(fmt.Sprintf("stack:leave:%d", stackID)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData(fmt.Sprintf("stack:settings:%d", stackID)),
		),
	)
}

// EntryDetailKeyboard builds entry view keyboard
func EntryDetailKeyboard(entry *model.Entry) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		// Rating button
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⭐️ Rate ⭐️").WithCallbackData(fmt.Sprintf("entry:rate_menu:%d", entry.ID)),
		),
		// Steppers
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⏪ -1 S").WithCallbackData(fmt.Sprintf("entry:step:season:-1:%d", entry.ID)),
			tu.InlineKeyboardButton("◀️ -1 Ep").WithCallbackData(fmt.Sprintf("entry:step:episode:-1:%d", entry.ID)),
			tu.InlineKeyboardButton("▶️ +1 Ep").WithCallbackData(fmt.Sprintf("entry:step:episode:1:%d", entry.ID)),
			tu.InlineKeyboardButton("⏩ +1 S").WithCallbackData(fmt.Sprintf("entry:step:season:1:%d", entry.ID)),
		),
		// Edits
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✍️ Title").WithCallbackData(fmt.Sprintf("entry:edit_title:%d", entry.ID)),
			tu.InlineKeyboardButton("🏷 Type").WithCallbackData(fmt.Sprintf("entry:edit_type:%d", entry.ID)),
			tu.InlineKeyboardButton("🔢 Progress").WithCallbackData(fmt.Sprintf("entry:edit_progress:%d", entry.ID)),
		),
		// Deletion & return
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🗑 Delete Entry").WithCallbackData(fmt.Sprintf("entry:delete_confirm:%d", entry.ID)),
			tu.InlineKeyboardButton("🗄 Back to Stack").WithCallbackData(fmt.Sprintf("stack:view:%d", entry.StackID)),
		),
	)
}

// RatingPickerKeyboard displays 1-6 star buttons
func RatingPickerKeyboard(entryID int64) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⭐️ (1)").WithCallbackData(fmt.Sprintf("entry:rate:%d:1", entryID)),
			tu.InlineKeyboardButton("⭐️⭐️ (2)").WithCallbackData(fmt.Sprintf("entry:rate:%d:2", entryID)),
			tu.InlineKeyboardButton("⭐️⭐️⭐️ (3)").WithCallbackData(fmt.Sprintf("entry:rate:%d:3", entryID)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⭐️⭐️⭐️⭐️ (4)").WithCallbackData(fmt.Sprintf("entry:rate:%d:4", entryID)),
			tu.InlineKeyboardButton("⭐️⭐️⭐️⭐️⭐️ (5)").WithCallbackData(fmt.Sprintf("entry:rate:%d:5", entryID)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🌟✨🌟✨🌟 MEOW (6)").WithCallbackData(fmt.Sprintf("entry:rate:%d:6", entryID)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData(fmt.Sprintf("entry:view:%d", entryID)),
		),
	)
}

// ConfirmEntryDeleteKeyboard confirms entry deletion
func ConfirmEntryDeleteKeyboard(entry *model.Entry) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⚠️ Yes, Delete").WithCallbackData(fmt.Sprintf("entry:delete:%d", entry.ID)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData(fmt.Sprintf("entry:view:%d", entry.ID)),
		),
	)
}

// MediaTypeKeyboard provides quick type choices or custom text prompt
func MediaTypeKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🎬 Movie").WithCallbackData("entry:type_choice:Movie"),
			tu.InlineKeyboardButton("📺 TV Series").WithCallbackData("entry:type_choice:TV Series"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🌸 Anime").WithCallbackData("entry:type_choice:Anime"),
			tu.InlineKeyboardButton("🍿 Cartoon").WithCallbackData("entry:type_choice:Cartoon"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("action:cancel"),
		),
	)
}

// CancelKeyboard returns a simple Cancel button
func CancelKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("action:cancel"),
		),
	)
}

// BackToMenuKeyboard returns button to return to main menu
func BackToMenuKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ Back to Menu").WithCallbackData("menu:main"),
		),
	)
}
